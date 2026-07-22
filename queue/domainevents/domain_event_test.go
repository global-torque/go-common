package domainevents

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const validPayload = `{
  "id": "6d0aaf23-d5ea-4ed5-b020-60fb9ba72155",
  "type": "investment.funding-status.changed.v1",
  "version": 1,
  "source": "postgres-outbox",
  "object": "investment",
  "object_id": "42",
  "field": "funding_status",
  "data": {"funding_status": "legally_confirmed"},
  "time": "2026-07-13T12:34:56.123456Z"
}`

const validCreatedPayload = `{
  "id": "80e8e58e-9184-4316-b174-4da418786be2",
  "type": "offer.created.v1",
  "version": 1,
  "source": "postgres-outbox",
  "object": "offer",
  "object_id": "42",
  "field": "created",
  "data": {
    "id": 42,
    "name": "Series A",
    "status": "new",
    "entity_id": null,
    "data": {"tokenization": false}
  },
  "time": "2026-07-22T12:34:56.123456Z"
}`

func TestDecodeV1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		payload   string
		wantValue string
		wantError error
	}{
		{name: "valid string transition", payload: validPayload, wantValue: "legally_confirmed"},
		{
			name: "valid null transition",
			payload: replaceJSONField(t, validPayload, "data", map[string]any{
				"funding_status": nil,
			}),
		},
		{
			name:      "unsupported version",
			payload:   replaceJSONField(t, validPayload, "version", 2),
			wantError: ErrUnsupportedVersion,
		},
		{
			name:      "wrong event type",
			payload:   replaceJSONField(t, validPayload, "type", "investment.status.changed.v1"),
			wantError: ErrMalformedEvent,
		},
		{
			name: "multiple data fields",
			payload: replaceJSONField(t, validPayload, "data", map[string]any{
				"funding_status": "legally_confirmed",
				"status":         "active",
			}),
			wantError: ErrMalformedEvent,
		},
		{
			name:      "duplicate top-level field",
			payload:   validPayload[:len(validPayload)-1] + `, "field": "status"}`,
			wantError: ErrMalformedEvent,
		},
		{
			name: "duplicate changed field",
			payload: `{
  "id":"6d0aaf23-d5ea-4ed5-b020-60fb9ba72155",
  "type":"offer.status.changed.v1",
  "version":1,
  "source":"postgres-outbox",
  "object":"offer",
  "object_id":"42",
  "field":"status",
  "data":{"status":"draft","status":"legal-accepted"},
  "time":"2026-07-13T12:34:56.123456Z"
}`,
			wantError: ErrMalformedEvent,
		},
		{
			name:      "unknown top-level field",
			payload:   validPayload[:len(validPayload)-1] + `, "old_value": "pending"}`,
			wantError: ErrMalformedEvent,
		},
		{
			name:      "trailing JSON",
			payload:   validPayload + `{}`,
			wantError: ErrMalformedEvent,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			event, err := DecodeV1([]byte(test.payload))
			if test.wantError != nil {
				require.ErrorIs(t, err, test.wantError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "6d0aaf23-d5ea-4ed5-b020-60fb9ba72155", event.ID)
			require.Equal(t, test.wantValue, rawString(t, event.Data["funding_status"]))
		})
	}
}

func TestDecodeV1MonitoredEventMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		object    string
		field     string
		value     any
		wantType  string
		wantValue string
	}{
		{name: "offer status", object: "offer", field: "status", value: "legal-accepted", wantType: "offer.status.changed.v1", wantValue: `"legal-accepted"`},
		{name: "offer subscribed shares", object: "offer", field: "subscribed_shares", value: 150, wantType: "offer.subscribed-shares.changed.v1", wantValue: `150`},
		{name: "offer confirmed shares", object: "offer", field: "confirmed_shares", value: 120, wantType: "offer.confirmed-shares.changed.v1", wantValue: `120`},
		{name: "profile kyc status", object: "profile", field: "kyc_status", value: "approved", wantType: "profile.kyc-status.changed.v1", wantValue: `"approved"`},
		{name: "profile accreditation status", object: "profile", field: "accreditation_status", value: nil, wantType: "profile.accreditation-status.changed.v1", wantValue: `null`},
		{name: "investment status", object: "investment", field: "status", value: "legally_confirmed", wantType: "investment.status.changed.v1", wantValue: `"legally_confirmed"`},
		{name: "investment funding status", object: "investment", field: "funding_status", value: "settled", wantType: "investment.funding-status.changed.v1", wantValue: `"settled"`},
		{name: "investment funding type", object: "investment", field: "funding_type", value: "wallet", wantType: "investment.funding-type.changed.v1", wantValue: `"wallet"`},
		{name: "evm wallet operation status", object: "evm-wallet-operation", field: "status", value: "confirmed", wantType: "evm-wallet-operation.status.changed.v1", wantValue: `"confirmed"`},
		{name: "wallet transaction status", object: "wallet-transaction", field: "status", value: "processed", wantType: "wallet-transaction.status.changed.v1", wantValue: `"processed"`},
		{name: "boolean remains boolean", object: "offer", field: "status", value: true, wantType: "offer.status.changed.v1", wantValue: `true`},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			payload := eventPayload(t, test.object, test.field, test.value)
			event, err := DecodeV1(payload)
			require.NoError(t, err)
			require.Equal(t, test.wantType, event.Type)
			require.JSONEq(t, test.wantValue, string(event.Data[test.field]))
		})
	}
}

func TestDecodeV1CreatedEventMatrix(t *testing.T) {
	t.Parallel()

	for _, object := range []string{
		"offer",
		"investment",
		"profile",
		"wallet-transaction",
		"evm-wallet-operation",
	} {
		object := object
		t.Run(object, func(t *testing.T) {
			t.Parallel()

			payload := createdEventPayload(t, object, map[string]any{
				"id":         42,
				"status":     "new",
				"nullable":   nil,
				"metadata":   map[string]any{"provider": "sandbox"},
				"created_at": "2026-07-22T12:34:56.123456Z",
			})
			event, err := DecodeV1(payload)
			require.NoError(t, err)
			require.True(t, event.IsCreated())
			require.Equal(t, object+".created.v1", event.Type)
			require.Equal(t, CreatedField, event.Field)
			require.Len(t, event.Data, 5)
			require.JSONEq(t, `42`, string(event.Data["id"]))
		})
	}
}

func TestDecodeV1RejectsMalformedCreatedEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "wrong lifecycle type",
			payload: replaceJSONField(t, validCreatedPayload, "type", "offer.created.v2"),
		},
		{
			name:    "empty row",
			payload: replaceJSONField(t, validCreatedPayload, "data", map[string]any{}),
		},
		{
			name: "missing row id",
			payload: replaceJSONField(t, validCreatedPayload, "data", map[string]any{
				"name": "Series A",
			}),
		},
		{
			name: "mismatched row id",
			payload: replaceJSONField(t, validCreatedPayload, "data", map[string]any{
				"id": 43,
			}),
		},
		{
			name: "invalid row field",
			payload: replaceJSONField(t, validCreatedPayload, "data", map[string]any{
				"id":        42,
				"bad-field": true,
			}),
		},
		{
			name: "created type on changed shape",
			payload: replaceJSONField(t,
				replaceJSONField(t, validPayload, "type", "investment.created.v1"),
				"field", "status"),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := DecodeV1([]byte(test.payload))
			require.ErrorIs(t, err, ErrMalformedEvent)
		})
	}
}

func TestDecodeV1RejectsMalformedContractFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		field     string
		value     any
		wantError error
	}{
		{name: "non UUID id", field: "id", value: "event-42", wantError: ErrMalformedEvent},
		{name: "invalid time", field: "time", value: "yesterday", wantError: ErrMalformedEvent},
		{name: "null time", field: "time", value: nil, wantError: ErrMalformedEvent},
		{name: "empty object id", field: "object_id", value: "", wantError: ErrMalformedEvent},
		{name: "wrong data key", field: "data", value: map[string]any{"status": "settled"}, wantError: ErrMalformedEvent},
		{name: "null data object", field: "data", value: nil, wantError: ErrMalformedEvent},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := DecodeV1([]byte(replaceJSONField(t, validPayload, test.field, test.value)))
			require.ErrorIs(t, err, test.wantError)
		})
	}
}

func TestDomainEventV1ValidateDelivery(t *testing.T) {
	t.Parallel()

	event, err := DecodeV1([]byte(validPayload))
	require.NoError(t, err)

	validAttributes := map[string]string{
		"type":      "investment.funding-status.changed.v1",
		"version":   "1",
		"object":    "investment",
		"object_id": "42",
		"field":     "funding_status",
	}

	tests := []struct {
		name        string
		attributes  map[string]string
		orderingKey string
		wantError   bool
	}{
		{
			name:        "matching delivery metadata",
			attributes:  validAttributes,
			orderingKey: "investment:42",
		},
		{
			name: "message id is not an event attribute",
			attributes: map[string]string{
				"type":      "investment.funding-status.changed.v1",
				"version":   "1",
				"object":    "investment",
				"object_id": "42",
				"field":     "status",
			},
			orderingKey: "investment:42",
			wantError:   true,
		},
		{
			name:        "wrong ordering key",
			attributes:  validAttributes,
			orderingKey: "investment:43",
			wantError:   true,
		},
	}
	for _, attribute := range []string{"type", "version", "object", "object_id", "field"} {
		attributes := cloneAttributes(validAttributes)
		attributes[attribute] = "mismatch"
		tests = append(tests, struct {
			name        string
			attributes  map[string]string
			orderingKey string
			wantError   bool
		}{
			name:        "mismatched " + attribute + " attribute",
			attributes:  attributes,
			orderingKey: "investment:42",
			wantError:   true,
		})
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := event.ValidateDelivery(test.attributes, test.orderingKey)
			if test.wantError {
				require.ErrorIs(t, err, ErrMalformedEvent)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDomainEventV1CreatedValidateDelivery(t *testing.T) {
	t.Parallel()

	event, err := DecodeV1([]byte(validCreatedPayload))
	require.NoError(t, err)

	attributes := map[string]string{
		"type":      "offer.created.v1",
		"version":   "1",
		"object":    "offer",
		"object_id": "42",
		"field":     "created",
	}
	require.NoError(t, event.ValidateDelivery(attributes, "offer:42"))

	attributes["field"] = "status"
	require.ErrorIs(t, event.ValidateDelivery(attributes, "offer:42"), ErrMalformedEvent)
}

func TestParseMode(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]Mode{
		"":       ModeOff,
		" OFF ":  ModeOff,
		"shadow": ModeShadow,
		"ACTIVE": ModeActive,
	} {
		got, err := ParseMode(input)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	_, err := ParseMode("enabled")
	require.ErrorIs(t, err, ErrMalformedEvent)
}

func TestSuppressLegacyFieldEvent(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		objectName string
		action     string
		data       map[string]any
		want       bool
		wantError  bool
	}{
		{name: "off keeps monitored field", mode: "off", objectName: "investment", action: "post_update", data: map[string]any{"status": "confirmed"}},
		{name: "shadow keeps monitored field", mode: "shadow", objectName: "investment", action: "post_update", data: map[string]any{"status": "confirmed"}},
		{name: "active suppresses monitored field", mode: "active", objectName: "investment", action: "post_update", data: map[string]any{"funding_type": "wallet"}, want: true},
		{name: "active keeps insert", mode: "active", objectName: "profile", action: "post_add", data: map[string]any{"kyc_status": "approved"}},
		{name: "active keeps unmonitored field", mode: "active", objectName: "offer", action: "post_update", data: map[string]any{"name": "updated"}},
		{name: "active keeps synthetic publish command", mode: "active", objectName: "offer", action: "post_update", data: map[string]any{"status": "publish"}},
		{name: "mixed update is owned by outbox", mode: "active", objectName: "offer", action: "post_update", data: map[string]any{"name": "updated", "status": "legal-accepted"}, want: true},
		{name: "invalid mode fails safely", mode: "enabled", objectName: "offer", action: "post_update", data: map[string]any{"status": "legal-accepted"}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("DOMAIN_EVENTS_MODE", test.mode)
			got, err := SuppressLegacyFieldEvent(test.objectName, test.action, test.data)
			if test.wantError {
				require.ErrorIs(t, err, ErrMalformedEvent)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestDomainEventV1PositiveIntObjectID(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		objectID string
		want     int
		wantErr  bool
	}{
		{objectID: "42", want: 42},
		{objectID: "0", wantErr: true},
		{objectID: "-7", wantErr: true},
		{objectID: "profile-42", wantErr: true},
	} {
		event := DomainEventV1{ObjectID: test.objectID}
		got, err := event.PositiveIntObjectID()
		if test.wantErr {
			require.ErrorIs(t, err, ErrMalformedEvent)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, test.want, got)
	}
}

func TestDomainEventV1StringValue(t *testing.T) {
	t.Parallel()

	event, err := DecodeV1([]byte(validPayload))
	require.NoError(t, err)
	value, present, err := event.StringValue()
	require.NoError(t, err)
	require.True(t, present)
	require.Equal(t, "legally_confirmed", value)

	event.Data[event.Field] = json.RawMessage("null")
	value, present, err = event.StringValue()
	require.NoError(t, err)
	require.False(t, present)
	require.Empty(t, value)

	event.Data[event.Field] = json.RawMessage("42")
	_, _, err = event.StringValue()
	require.ErrorIs(t, err, ErrMalformedEvent)

	created, err := DecodeV1([]byte(validCreatedPayload))
	require.NoError(t, err)
	_, _, err = created.StringValue()
	require.ErrorIs(t, err, ErrMalformedEvent)
}

func eventPayload(t *testing.T, object, field string, value any) []byte {
	t.Helper()

	payload := map[string]any{
		"id":        "6d0aaf23-d5ea-4ed5-b020-60fb9ba72155",
		"type":      object + "." + replaceUnderscores(field) + ".changed.v1",
		"version":   1,
		"source":    "postgres-outbox",
		"object":    object,
		"object_id": "42",
		"field":     field,
		"data":      map[string]any{field: value},
		"time":      "2026-07-13T12:34:56.123456Z",
	}
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	return encoded
}

func createdEventPayload(t *testing.T, object string, data map[string]any) []byte {
	t.Helper()

	payload := map[string]any{
		"id":        "80e8e58e-9184-4316-b174-4da418786be2",
		"type":      object + ".created.v1",
		"version":   1,
		"source":    "postgres-outbox",
		"object":    object,
		"object_id": "42",
		"field":     CreatedField,
		"data":      data,
		"time":      "2026-07-22T12:34:56.123456Z",
	}
	encoded, err := json.Marshal(payload)
	require.NoError(t, err)
	return encoded
}

func replaceUnderscores(value string) string {
	result := []byte(value)
	for index := range result {
		if result[index] == '_' {
			result[index] = '-'
		}
	}
	return string(result)
}

func cloneAttributes(attributes map[string]string) map[string]string {
	clone := make(map[string]string, len(attributes))
	for name, value := range attributes {
		clone[name] = value
	}
	return clone
}

func replaceJSONField(t *testing.T, payload, field string, value any) string {
	t.Helper()

	var object map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &object))
	object[field] = value
	updated, err := json.Marshal(object)
	require.NoError(t, err)
	return string(updated)
}

func rawString(t *testing.T, value json.RawMessage) string {
	t.Helper()

	if string(value) == "null" {
		return ""
	}
	var decoded string
	require.NoError(t, json.Unmarshal(value, &decoded))
	return decoded
}
