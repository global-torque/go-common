// Package domainevents defines the versioned business-event contract carried
// by the PostgreSQL transactional outbox.
package domainevents

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// VersionV1 is the only domain-event payload version supported by this package.
	VersionV1 = 1
	// SourcePostgresOutbox identifies events produced by the PostgreSQL outbox trigger.
	SourcePostgresOutbox = "postgres-outbox"
	// ModeOff validates deliveries without evaluating service actions.
	ModeOff Mode = "off"
	// ModeShadow evaluates and logs service actions without performing them.
	ModeShadow Mode = "shadow"
	// ModeActive evaluates and performs service actions.
	ModeActive Mode = "active"

	statusField = "status"
)

// Mode controls a consumer's migration behavior.
type Mode string

var (
	// ErrMalformedEvent identifies a payload that does not satisfy the v1 contract.
	ErrMalformedEvent = errors.New("malformed domain event")
	// ErrUnsupportedVersion identifies a syntactically valid event with an unsupported version.
	ErrUnsupportedVersion = errors.New("unsupported domain event version")

	objectPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	fieldPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// DomainEventV1 is the canonical payload emitted by the PostgreSQL outbox.
// Data contains exactly one key: Field. Its RawMessage value intentionally
// preserves JSON strings, numbers, booleans, objects, and null transitions.
type DomainEventV1 struct {
	ID       string                     `json:"id"`
	Type     string                     `json:"type"`
	Version  int                        `json:"version"`
	Source   string                     `json:"source"`
	Object   string                     `json:"object"`
	ObjectID string                     `json:"object_id"`
	Field    string                     `json:"field"`
	Data     map[string]json.RawMessage `json:"data"`
	Time     time.Time                  `json:"time"`
}

// ParseMode validates the DOMAIN_EVENTS_MODE value. An empty value is treated
// as off so deploying the endpoint cannot accidentally activate side effects.
func ParseMode(value string) (Mode, error) {
	mode := Mode(strings.ToLower(strings.TrimSpace(value)))
	if mode == "" {
		return ModeOff, nil
	}

	switch mode {
	case ModeOff, ModeShadow, ModeActive:
		return mode, nil
	default:
		return "", fmt.Errorf("%w: domain events mode %q", ErrMalformedEvent, value)
	}
}

// EnvironmentMode returns the safe rollout mode configured for every domain
// event push endpoint. Keeping this unprefixed preserves the deployment
// contract DOMAIN_EVENTS_MODE=off|shadow|active.
func EnvironmentMode() (Mode, error) {
	return ParseMode(os.Getenv("DOMAIN_EVENTS_MODE"))
}

// SuppressLegacyFieldEvent reports whether an old field-change delivery is
// owned by the transactional outbox after this worker moves to active mode.
// Insert/delete and unmonitored-field events intentionally remain on the
// legacy path during the incremental migration.
func SuppressLegacyFieldEvent(objectName, action string, data map[string]any) (bool, error) {
	mode, err := EnvironmentMode()
	if err != nil {
		return false, err
	}

	if mode != ModeActive || action != "post_update" {
		return false, nil
	}
	// "publish" is an escrow command, not the persisted offer status
	// ("published"). Synthetic commands remain on the legacy topic.
	if objectName == "offer" {
		if status, ok := data[statusField].(string); ok && status == "publish" {
			return false, nil
		}
	}

	monitoredFields := map[string][]string{
		"offer":                {statusField, "subscribed_shares", "confirmed_shares"},
		"profile":              {"kyc_status", "accreditation_status"},
		"investment":           {statusField, "funding_status", "funding_type"},
		"evm_wallet_operation": {statusField},
		"transaction":          {statusField},
		"wallet_transaction":   {statusField},
	}
	for _, field := range monitoredFields[objectName] {
		if _, present := data[field]; present {
			return true, nil
		}
	}

	return false, nil
}

// StringValue returns the changed value when it is a JSON string. A JSON null
// is returned as ("", false, nil), allowing consumers to ignore nullable
// transitions without conflating them with malformed scalar values.
func (event DomainEventV1) StringValue() (string, bool, error) {
	raw, ok := event.Data[event.Field]
	if !ok {
		return "", false, malformed("data must contain field %q", event.Field)
	}

	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false, nil
	}

	var value string

	err := json.Unmarshal(raw, &value)
	if err != nil {
		return "", false, malformed("data.%s must be a JSON string or null", event.Field)
	}

	return value, true, nil
}

// PositiveIntObjectID parses the numeric primary keys used by the current six
// workers. It returns a contract error for zero and negative IDs as well as
// non-numeric values, so known events cannot be silently acknowledged.
func (event DomainEventV1) PositiveIntObjectID() (int, error) {
	objectID, err := strconv.Atoi(event.ObjectID)
	if err != nil {
		return 0, malformed("object_id %q must be a positive integer", event.ObjectID)
	}

	if objectID < 1 {
		return 0, malformed("object_id %q must be a positive integer", event.ObjectID)
	}

	return objectID, nil
}

// DecodeV1 strictly decodes and validates one v1 domain-event payload.
// Unknown top-level fields and trailing JSON are rejected so producers and
// consumers cannot silently diverge on the wire contract.
func DecodeV1(payload []byte) (DomainEventV1, error) {
	err := rejectDuplicateObjectKeys(payload)
	if err != nil {
		return DomainEventV1{}, err
	}

	var event DomainEventV1

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()

	err = decoder.Decode(&event)
	if err != nil {
		return DomainEventV1{}, fmt.Errorf("%w: decode payload: %w", ErrMalformedEvent, err)
	}

	err = ensureJSONEOF(decoder)
	if err != nil {
		return DomainEventV1{}, err
	}

	err = event.Validate()
	if err != nil {
		return DomainEventV1{}, err
	}

	return event, nil
}

func rejectDuplicateObjectKeys(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()

	err := consumeJSONValue(decoder)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedEvent, err)
	}

	err = ensureJSONEOF(decoder)
	if err != nil {
		return err
	}

	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode JSON token: %w", err)
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		seen := make(map[string]struct{})

		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("decode object key: %w", err)
			}

			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("%w: object key is not a string", ErrMalformedEvent)
			}

			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("%w: duplicate object key %q", ErrMalformedEvent, key)
			}

			seen[key] = struct{}{}

			err = consumeJSONValue(decoder)
			if err != nil {
				return err
			}
		}

		return consumeClosingDelimiter(decoder, '}')
	case '[':
		for decoder.More() {
			err := consumeJSONValue(decoder)
			if err != nil {
				return err
			}
		}

		return consumeClosingDelimiter(decoder, ']')
	default:
		return fmt.Errorf("%w: unexpected opening delimiter %q", ErrMalformedEvent, delimiter)
	}
}

func consumeClosingDelimiter(decoder *json.Decoder, expected json.Delim) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode closing delimiter: %w", err)
	}

	if token != expected {
		return fmt.Errorf("%w: closing delimiter is %q, want %q", ErrMalformedEvent, token, expected)
	}

	return nil
}

// Validate checks the event payload independently of its Pub/Sub delivery metadata.
func (event DomainEventV1) Validate() error {
	if event.Version != VersionV1 {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedVersion, event.Version, VersionV1)
	}

	_, err := uuid.Parse(event.ID)
	if err != nil {
		return malformed("id must be a UUID")
	}

	if event.Source != SourcePostgresOutbox {
		return malformed("source must be %q", SourcePostgresOutbox)
	}

	if !objectPattern.MatchString(event.Object) {
		return malformed("object %q is not a lower-case hyphenated name", event.Object)
	}

	if strings.TrimSpace(event.ObjectID) == "" {
		return malformed("object_id is required")
	}

	if !fieldPattern.MatchString(event.Field) {
		return malformed("field %q is not a lower-case SQL field name", event.Field)
	}

	expectedType := event.Object + "." + strings.ReplaceAll(event.Field, "_", "-") + ".changed.v1"
	if event.Type != expectedType {
		return malformed("type is %q, want %q", event.Type, expectedType)
	}

	if len(event.Data) != 1 {
		return malformed("data must contain exactly one field")
	}

	value, ok := event.Data[event.Field]
	if !ok {
		return malformed("data must contain field %q", event.Field)
	}

	if len(value) == 0 || !json.Valid(value) {
		return malformed("data.%s must contain a JSON value", event.Field)
	}

	if event.Time.IsZero() {
		return malformed("time is required")
	}

	return nil
}

// ValidateDelivery checks the Pub/Sub attributes and ordering key copied from
// an outbox event by the Debezium Outbox Event Router.
func (event DomainEventV1) ValidateDelivery(attributes map[string]string, orderingKey string) error {
	expected := map[string]string{
		"type":      event.Type,
		"version":   strconv.Itoa(event.Version),
		"object":    event.Object,
		"object_id": event.ObjectID,
		"field":     event.Field,
	}
	for name, value := range expected {
		if attributes[name] != value {
			return malformed("attribute %s is %q, want %q", name, attributes[name], value)
		}
	}

	expectedOrderingKey := event.Object + ":" + event.ObjectID
	if orderingKey != expectedOrderingKey {
		return malformed("ordering key is %q, want %q", orderingKey, expectedOrderingKey)
	}

	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing json.RawMessage

	err := decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return malformed("payload contains trailing JSON")
		}

		return fmt.Errorf("%w: decode trailing data: %w", ErrMalformedEvent, err)
	}

	return nil
}

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrMalformedEvent, fmt.Sprintf(format, args...))
}
