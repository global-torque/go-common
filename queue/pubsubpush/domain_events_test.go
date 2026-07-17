package pubsubpush

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

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

func TestSubscriptionResource(t *testing.T) {
	t.Parallel()

	resource, err := SubscriptionResource(" webdevelop-live ", " dev-domain-events-email-worker-dev ")
	require.NoError(t, err)
	require.Equal(t, "projects/webdevelop-live/subscriptions/dev-domain-events-email-worker-dev", resource)

	_, err = SubscriptionResource("webdevelop-live", "projects/other/subscriptions/forged")
	require.Error(t, err)
}
