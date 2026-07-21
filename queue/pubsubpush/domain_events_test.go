package pubsubpush

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	domainevents "github.com/global-torque/go-common/queue/v2/domainevents"
	"github.com/stretchr/testify/require"
)

func TestDecodeDomainEventRequiresTransportIdentity(t *testing.T) {
	t.Parallel()

	valid := PushRequest{
		Message: PushMessage{
			MessageID:   "pubsub-123",
			OrderingKey: "offer:42",
			Attributes: map[string]string{
				"type": "offer.status.changed.v1", "version": "1", "object": "offer",
				"object_id": "42", "field": "status",
			},
			Data: []byte(`{"id":"6d0aaf23-d5ea-4ed5-b020-60fb9ba72155","type":"offer.status.changed.v1","version":1,"source":"postgres-outbox","object":"offer","object_id":"42","field":"status","data":{"status":"legal-accepted"},"time":"2026-07-13T12:34:56Z"}`),
		},
	}

	delivery, err := DecodeDomainEvent(valid)
	require.NoError(t, err)
	require.Equal(t, "pubsub-123", delivery.MessageID)

	missingID := valid
	missingID.Message.MessageID = "  "
	_, err = DecodeDomainEvent(missingID)
	require.True(t, errors.Is(err, domainevents.ErrMalformedEvent))

	invalidAttempt := valid
	zero := 0
	invalidAttempt.DeliveryAttempt = &zero
	_, err = DecodeDomainEvent(invalidAttempt)
	require.True(t, errors.Is(err, domainevents.ErrMalformedEvent))
}

func TestDecodeDomainEventPushRequiresExactSubscriptionAndStrictEnvelope(t *testing.T) {
	t.Parallel()

	const expected = "projects/webdevelop-live/subscriptions/dev-domain-events-escrow-worker-dev"
	valid := PushRequest{
		Subscription: expected,
		Message: PushMessage{
			MessageID:   "pubsub-123",
			OrderingKey: "offer:42",
			Attributes: map[string]string{
				"type": "offer.status.changed.v1", "version": "1", "object": "offer",
				"object_id": "42", "field": "status",
			},
			Data: []byte(`{"id":"6d0aaf23-d5ea-4ed5-b020-60fb9ba72155","type":"offer.status.changed.v1","version":1,"source":"postgres-outbox","object":"offer","object_id":"42","field":"status","data":{"status":"legal-accepted"},"time":"2026-07-13T12:34:56Z"}`),
		},
	}
	payload, err := json.Marshal(valid)
	require.NoError(t, err)

	delivery, err := DecodeDomainEventPush(bytes.NewReader(payload), expected)
	require.NoError(t, err)
	require.Equal(t, expected, delivery.Subscription)

	forged := valid
	forged.Subscription = "projects/other/subscriptions/dev-domain-events-escrow-worker-dev"
	payload, err = json.Marshal(forged)
	require.NoError(t, err)
	_, err = DecodeDomainEventPush(bytes.NewReader(payload), expected)
	require.ErrorIs(t, err, ErrUnexpectedSubscription)

	partial := valid
	partial.Subscription = "dev-domain-events-escrow-worker-dev"
	payload, err = json.Marshal(partial)
	require.NoError(t, err)
	_, err = DecodeDomainEventPush(bytes.NewReader(payload), expected)
	require.ErrorIs(t, err, ErrUnexpectedSubscription)

	payload, err = json.Marshal(valid)
	require.NoError(t, err)
	_, err = DecodeDomainEventPush(bytes.NewReader(append(payload, []byte(` {}`)...)), expected)
	require.ErrorIs(t, err, domainevents.ErrMalformedEvent)

	unknown := append(payload[:len(payload)-1], []byte(`,"unexpected":true}`)...)
	_, err = DecodeDomainEventPush(bytes.NewReader(unknown), expected)
	require.ErrorIs(t, err, domainevents.ErrMalformedEvent)
}

func TestDecodeDomainEventPushAcceptsPubSubProtoJSONFieldNames(t *testing.T) {
	t.Parallel()

	const expected = "projects/webdevelop-live/subscriptions/dev-domain-events-escrow-worker-dev"

	tests := []struct {
		name            string
		messageMetadata string
		attemptMetadata string
	}{
		{
			name:            "protobuf JSON names",
			messageMetadata: `"messageId":"pubsub-camel","publishTime":"2026-07-17T09:00:00Z","orderingKey":"offer:42"`,
			attemptMetadata: `,"deliveryAttempt":2`,
		},
		{
			name:            "original proto field names",
			messageMetadata: `"message_id":"pubsub-snake","publish_time":"2026-07-17T09:00:00Z","ordering_key":"offer:42"`,
			attemptMetadata: `,"delivery_attempt":3`,
		},
		{
			name:            "mixed recognized names",
			messageMetadata: `"message_id":"pubsub-mixed","publishTime":"2026-07-17T09:00:00Z","ordering_key":"offer:42"`,
			attemptMetadata: `,"deliveryAttempt":4`,
		},
		{
			name: "official wrapped envelope with equal aliases",
			messageMetadata: `"messageId":"pubsub-both","message_id":"pubsub-both",` +
				`"publishTime":"2026-07-17T09:00:00Z","publish_time":"2026-07-17T09:00:00Z",` +
				`"orderingKey":"offer:42","ordering_key":"offer:42"`,
			attemptMetadata: `,"deliveryAttempt":5,"delivery_attempt":5`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			payload := domainEventPushPayload(expected, test.messageMetadata, test.attemptMetadata)
			delivery, err := DecodeDomainEventPush(bytes.NewReader(payload), expected)
			require.NoError(t, err)
			require.Equal(t, "offer.status.changed.v1", delivery.Event.Type)
			require.Equal(t, "42", delivery.Event.ObjectID)
			require.NotEmpty(t, delivery.MessageID)
			require.GreaterOrEqual(t, delivery.Attempt, 2)
		})
	}
}

func TestDecodeDomainEventPushRejectsDuplicateAliasConflictsAndUnknownFields(t *testing.T) {
	t.Parallel()

	const expected = "projects/webdevelop-live/subscriptions/dev-domain-events-escrow-worker-dev"

	tests := []struct {
		name            string
		messageMetadata string
		attemptMetadata string
	}{
		{
			name:            "conflicting metadata through both aliases",
			messageMetadata: `"messageId":"pubsub-123","message_id":"pubsub-forged","orderingKey":"offer:42"`,
		},
		{
			name: "conflicting publish time aliases",
			messageMetadata: `"message_id":"pubsub-123","publishTime":"2026-07-17T09:00:00Z",` +
				`"publish_time":"2026-07-17T09:00:01Z","ordering_key":"offer:42"`,
		},
		{
			name:            "conflicting ordering key aliases",
			messageMetadata: `"message_id":"pubsub-123","orderingKey":"offer:42","ordering_key":"offer:43"`,
		},
		{
			name:            "duplicate exact field name",
			messageMetadata: `"message_id":"pubsub-123","message_id":"pubsub-123","ordering_key":"offer:42"`,
		},
		{
			name:            "conflicting delivery attempt aliases",
			messageMetadata: `"message_id":"pubsub-123","ordering_key":"offer:42"`,
			attemptMetadata: `,"deliveryAttempt":2,"delivery_attempt":3`,
		},
		{
			name:            "unknown message metadata",
			messageMetadata: `"message_id":"pubsub-123","ordering_key":"offer:42","trace_id":"forged"`,
		},
		{
			name:            "case-insensitive near match is unknown",
			messageMetadata: `"MessageId":"pubsub-123","ordering_key":"offer:42"`,
		},
		{
			name:            "unknown top-level metadata",
			messageMetadata: `"message_id":"pubsub-123","ordering_key":"offer:42"`,
			attemptMetadata: `,"unexpected":true`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			payload := domainEventPushPayload(expected, test.messageMetadata, test.attemptMetadata)
			_, err := DecodeDomainEventPush(bytes.NewReader(payload), expected)
			require.ErrorIs(t, err, domainevents.ErrMalformedEvent)
		})
	}
}

func TestDecodeDomainEventPushWithTransportReturnsMetadataForInvalidPayload(t *testing.T) {
	t.Parallel()

	const expected = "projects/test-project/subscriptions/domain-events"
	payload := domainEventPushPayload(
		expected,
		`"messageId":"pubsub-invalid-123","orderingKey":"offer:42"`,
		`,"deliveryAttempt":3`,
	)
	var req PushRequest
	require.NoError(t, json.Unmarshal(payload, &req))
	req.Message.Data = bytes.Replace(
		req.Message.Data,
		[]byte("6d0aaf23-d5ea-4ed5-b020-60fb9ba72155"),
		[]byte("not-a-uuid"),
		1,
	)
	payload, err := json.Marshal(req)
	require.NoError(t, err)

	delivery, transport, err := DecodeDomainEventPushWithTransport(bytes.NewReader(payload), expected)
	require.ErrorIs(t, err, domainevents.ErrMalformedEvent)
	require.Empty(t, delivery.Event.ID)
	require.Equal(t, DomainEventTransport{
		MessageID: "pubsub-invalid-123", Subscription: expected, Attempt: 3,
	}, transport)

	wrongSubscription := domainEventPushPayload(
		"projects/test-project/subscriptions/other",
		`"messageId":"pubsub-invalid-123","orderingKey":"offer:42"`,
		`,"deliveryAttempt":3`,
	)
	_, transport, err = DecodeDomainEventPushWithTransport(bytes.NewReader(wrongSubscription), expected)
	require.ErrorIs(t, err, ErrUnexpectedSubscription)
	require.Empty(t, transport)
}

func domainEventPushPayload(subscription, messageMetadata, attemptMetadata string) []byte {
	const event = `{"id":"6d0aaf23-d5ea-4ed5-b020-60fb9ba72155","type":"offer.status.changed.v1","version":1,"source":"postgres-outbox","object":"offer","object_id":"42","field":"status","data":{"status":"legal-accepted"},"time":"2026-07-13T12:34:56Z"}`

	data := base64.StdEncoding.EncodeToString([]byte(event))

	return []byte(fmt.Sprintf(
		`{"message":{"attributes":{"type":"offer.status.changed.v1","version":"1","object":"offer","object_id":"42","field":"status"},"data":%q,%s},"subscription":%q%s}`,
		data,
		messageMetadata,
		subscription,
		attemptMetadata,
	))
}

func TestPushRequestUnmarshalPreservesPublishTimeAliases(t *testing.T) {
	t.Parallel()

	const expected = "projects/webdevelop-live/subscriptions/dev-domain-events-escrow-worker-dev"
	const published = "2026-07-17T09:00:00Z"

	for _, field := range []string{"publishTime", "publish_time"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()

			payload := domainEventPushPayload(
				expected,
				fmt.Sprintf(`"message_id":"pubsub-123",%q:%q,"ordering_key":"offer:42"`, field, published),
				``,
			)

			var request PushRequest
			require.NoError(t, json.Unmarshal(payload, &request))
			require.Equal(t, time.Date(2026, time.July, 17, 9, 0, 0, 0, time.UTC), request.Message.PublishTime)
		})
	}
}

func TestSubscriptionResource(t *testing.T) {
	t.Parallel()

	resource, err := SubscriptionResource(" webdevelop-live ", " dev-domain-events-email-worker-dev ")
	require.NoError(t, err)
	require.Equal(t, "projects/webdevelop-live/subscriptions/dev-domain-events-email-worker-dev", resource)

	_, err = SubscriptionResource("webdevelop-live", "projects/other/subscriptions/forged")
	require.Error(t, err)
}
