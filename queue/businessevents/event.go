// Package businessevents constructs immutable facts and inserts them in caller-owned transactions.
package businessevents

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/global-torque/go-common/queue/v2/pclient"
	"github.com/google/uuid"
)

// Input contains authored fact values. Amounts and quantities must be explicit decimal strings.
type Input struct {
	ID         string
	Action     string
	Sender     string
	ObjectName string
	ObjectRef  string
	ObjectID   int
	OccurredAt time.Time
	RequestID  string
	Data       map[string]any
}

// ErrInvalid indicates an invalid authored business-event contract.
var ErrInvalid = errors.New("invalid business event")

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// New constructs a fact, generating an ID only when the caller has not supplied one.
func New(input Input) (pclient.Event, error) {
	id := input.ID
	if id == "" {
		id = uuid.NewString()
	}

	occurred := input.OccurredAt.UTC().Truncate(time.Microsecond)
	event := pclient.Event{
		ID: id, Version: 1, Action: pclient.EventType(input.Action), Sender: input.Sender,
		ObjectName: input.ObjectName, ObjectRef: input.ObjectRef, ObjectID: input.ObjectID,
		OccurredAt: &occurred, RequestID: input.RequestID, Data: input.Data,
	}

	err := Validate(event)
	if err != nil {
		return pclient.Event{}, err
	}
	// Snapshot maps/slices so subsequent caller mutations cannot rewrite the accepted fact.
	raw, err := json.Marshal(event.Data)
	if err != nil {
		return pclient.Event{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var snapshot map[string]any

	err = decoder.Decode(&snapshot)
	if err != nil {
		return pclient.Event{}, err
	}

	event.Data = snapshot

	return event, nil
}

var (
	numericReference  = regexp.MustCompile(`^[+-]?[0-9]+$`)
	positiveReference = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// Validate checks a complete authored event, including canonical PostgreSQL timestamp precision.
func Validate(event pclient.Event) error {
	parsed, err := uuid.Parse(event.ID)
	if err != nil || parsed.String() != event.ID || parsed == uuid.Nil {
		return invalid("business event id must be a canonical nonzero UUID")
	}

	if event.Version != 1 {
		return invalid("business event version must be 1")
	}

	metadata := map[string]string{
		"action": string(event.Action), "sender": event.Sender,
		"object_name": event.ObjectName, "object_ref": event.ObjectRef,
	}
	for name, value := range metadata {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return invalid("business event %s must be a nonempty canonical string", name)
		}
	}

	if numericReference.MatchString(event.ObjectRef) && !positiveReference.MatchString(event.ObjectRef) {
		return invalid("business event numeric object_ref must be canonical positive base-10")
	}

	if event.ObjectID < 0 || (event.ObjectID > 0 && strconv.Itoa(event.ObjectID) != event.ObjectRef) {
		return invalid("business event object_id must match object_ref")
	}

	if event.OccurredAt == nil || event.OccurredAt.IsZero() ||
		event.OccurredAt.Location() != time.UTC || event.OccurredAt.Nanosecond()%1000 != 0 {
		return invalid("business event occurred_at must be nonzero UTC at microsecond precision")
	}

	if event.Attempt != nil || event.IPAddress != "" {
		return invalid("business event cannot contain transport attempt or client IP")
	}

	if event.Data == nil {
		return invalid("business event data must be an object")
	}

	// encoding/json detects cyclic maps/slices before the typed-value traversal.
	_, err = json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("%w: serialize data: %w", ErrInvalid, err)
	}

	return validateValue(event.Data)
}

func validateValue(value any) error {
	switch value := value.(type) {
	case nil, bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return invalid("business event contains nonfinite number")
		}

		return nil
	case float32:
		return validateValue(float64(value))
	case json.Number:
		if !json.Valid([]byte(value)) || len(value) == 0 || (value[0] != '-' && (value[0] < '0' || value[0] > '9')) {
			return invalid("business event contains invalid JSON number")
		}

		var number json.Number

		err := json.Unmarshal([]byte(value), &number)
		if err != nil {
			return fmt.Errorf("%w: invalid JSON number: %w", ErrInvalid, err)
		}

		return nil
	case json.RawMessage:
		if !json.Valid(value) {
			return invalid("business event contains invalid raw JSON")
		}

		return nil
	case map[string]any:
		for _, v := range value {
			err := validateValue(v)
			if err != nil {
				return err
			}
		}

		return nil
	case []any:
		for _, v := range value {
			err := validateValue(v)
			if err != nil {
				return err
			}
		}

		return nil
	case []string:
		return nil
	default:
		return invalid("business event contains unsupported value %T; convert domain values explicitly", value)
	}
}

// Marshal emits only authored envelope fields, retaining legacy pclient JSON behavior elsewhere.
func Marshal(event pclient.Event) ([]byte, error) {
	err := Validate(event)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"id": event.ID, "version": event.Version, "action": event.Action, "sender": event.Sender,
		"object_name": event.ObjectName, "object_ref": event.ObjectRef, "occurred_at": event.OccurredAt, "data": event.Data,
	}
	if event.ObjectID != 0 {
		payload["object_id"] = event.ObjectID
	}

	if event.RequestID != "" {
		payload["request_id"] = event.RequestID
	}

	return json.Marshal(payload)
}
