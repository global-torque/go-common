// Package pubsubpush provides shared types and HTTP middleware for services
// that receive Pub/Sub messages over a push subscription.
//
// See https://cloud.google.com/pubsub/docs/push#receive_push for the wire
// format and https://cloud.google.com/pubsub/docs/handling-failures for the
// dead-letter / retry semantics.
package pubsubpush

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	fieldDeliveryAttempt = "deliveryAttempt"
	fieldMessageID       = "messageId"
	fieldOrderingKey     = "orderingKey"
	fieldPublishTime     = "publishTime"
)

var errInvalidPushJSON = errors.New("invalid Pub/Sub push JSON")

// PushRequest is the envelope Google Pub/Sub posts to push subscribers.
//
// DeliveryAttempt is populated at the *top level* of the envelope (not inside
// Message) when the subscription has dead-lettering configured. It tracks how
// many times Pub/Sub has tried to deliver this message.
//
// https://cloud.google.com/pubsub/docs/handling-failures#track_delivery_attempts
type PushRequest struct {
	Message         PushMessage `json:"message"`
	Subscription    string      `json:"subscription"`
	DeliveryAttempt *int        `json:"deliveryAttempt,omitempty"`
}

// UnmarshalJSON accepts both the protobuf JSON names and the original proto
// field names emitted by Pub/Sub while rejecting unknown and duplicate fields.
func (request *PushRequest) UnmarshalJSON(data []byte) error {
	fields, err := decodeStrictObject(data, map[string]string{
		"message":            "message",
		"subscription":       "subscription",
		fieldDeliveryAttempt: fieldDeliveryAttempt,
		"delivery_attempt":   fieldDeliveryAttempt,
	})
	if err != nil {
		return err
	}

	*request = PushRequest{}

	err = decodeField(fields, "message", &request.Message)
	if err != nil {
		return err
	}

	err = decodeField(fields, "subscription", &request.Subscription)
	if err != nil {
		return err
	}

	err = decodeField(fields, fieldDeliveryAttempt, &request.DeliveryAttempt)
	if err != nil {
		return err
	}

	return nil
}

// PushMessage is the inner Pub/Sub message carried by a push envelope.
type PushMessage struct {
	Attributes  map[string]string `json:"attributes"`
	Data        []byte            `json:"data"`
	MessageID   string            `json:"messageId"`
	PublishTime time.Time         `json:"publishTime"`
	OrderingKey string            `json:"orderingKey,omitempty"`
}

// UnmarshalJSON accepts the two field-name forms recognized by protobuf JSON.
// It intentionally does not relax the envelope to arbitrary metadata fields.
func (message *PushMessage) UnmarshalJSON(data []byte) error {
	fields, err := decodeStrictObject(data, map[string]string{
		"attributes":     "attributes",
		"data":           "data",
		fieldMessageID:   fieldMessageID,
		"message_id":     fieldMessageID,
		fieldPublishTime: fieldPublishTime,
		"publish_time":   fieldPublishTime,
		fieldOrderingKey: fieldOrderingKey,
		"ordering_key":   fieldOrderingKey,
	})
	if err != nil {
		return err
	}

	*message = PushMessage{}

	err = decodeField(fields, "attributes", &message.Attributes)
	if err != nil {
		return err
	}

	err = decodeField(fields, "data", &message.Data)
	if err != nil {
		return err
	}

	err = decodeField(fields, fieldMessageID, &message.MessageID)
	if err != nil {
		return err
	}

	err = decodeField(fields, fieldPublishTime, &message.PublishTime)
	if err != nil {
		return err
	}

	err = decodeField(fields, fieldOrderingKey, &message.OrderingKey)
	if err != nil {
		return err
	}

	return nil
}

func decodeStrictObject(data []byte, acceptedFields map[string]string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))

	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode object: %w", err)
	}

	delimiter, ok := token.(json.Delim)
	if !ok || delimiter != '{' {
		return nil, fmt.Errorf("%w: expected JSON object", errInvalidPushJSON)
	}

	fields := make(map[string]json.RawMessage)
	sources := make(map[string]string)

	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("decode field name: %w", err)
		}

		name, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("%w: expected JSON field name", errInvalidPushJSON)
		}

		canonical, ok := acceptedFields[name]
		if !ok {
			return nil, fmt.Errorf("%w: unknown field %q", errInvalidPushJSON, name)
		}

		if previous, exists := sources[canonical]; exists {
			return nil, fmt.Errorf("%w: duplicate field %q (also provided as %q)", errInvalidPushJSON, name, previous)
		}

		var raw json.RawMessage

		err = decoder.Decode(&raw)
		if err != nil {
			return nil, fmt.Errorf("decode field %q: %w", name, err)
		}

		fields[canonical] = raw
		sources[canonical] = name
	}

	token, err = decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode object close: %w", err)
	}

	if delimiter, ok = token.(json.Delim); !ok || delimiter != '}' {
		return nil, fmt.Errorf("%w: expected JSON object close", errInvalidPushJSON)
	}

	err = requireJSONEOF(decoder)
	if err != nil {
		return nil, err
	}

	return fields, nil
}

func decodeField(fields map[string]json.RawMessage, name string, target any) error {
	raw, exists := fields[name]
	if !exists {
		return nil
	}

	err := json.Unmarshal(raw, target)
	if err != nil {
		return fmt.Errorf("decode field %q: %w", name, err)
	}

	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing json.RawMessage

	err := decoder.Decode(&trailing)
	if err == nil {
		return fmt.Errorf("%w: JSON object contains trailing data", errInvalidPushJSON)
	}

	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode trailing JSON data: %w", err)
	}

	return nil
}
