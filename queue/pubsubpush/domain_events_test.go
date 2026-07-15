package pubsubpush

import (
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
