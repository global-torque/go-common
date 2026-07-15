package pubsubpush

import (
	"fmt"
	"strings"

	domainevents "github.com/global-torque/go-common/queue/v2/domainevents"
)

// DomainEventDelivery keeps immutable business identity separate from the
// Pub/Sub transport identity assigned to this particular publication.
type DomainEventDelivery struct {
	Event     domainevents.DomainEventV1
	MessageID string
	Attempt   int
}

// DecodeDomainEvent strictly validates both the payload and the metadata
// copied by the Debezium outbox route.
func DecodeDomainEvent(req PushRequest) (DomainEventDelivery, error) {
	messageID := strings.TrimSpace(req.Message.MessageID)
	if messageID == "" {
		return DomainEventDelivery{}, fmt.Errorf("%w: pubsub messageId is required", domainevents.ErrMalformedEvent)
	}

	if req.DeliveryAttempt != nil && *req.DeliveryAttempt < 1 {
		return DomainEventDelivery{}, fmt.Errorf("%w: deliveryAttempt must be positive", domainevents.ErrMalformedEvent)
	}

	event, err := domainevents.DecodeV1(req.Message.Data)
	if err != nil {
		return DomainEventDelivery{}, err
	}

	err = event.ValidateDelivery(req.Message.Attributes, req.Message.OrderingKey)
	if err != nil {
		return DomainEventDelivery{}, err
	}

	attempt := 0
	if req.DeliveryAttempt != nil {
		attempt = *req.DeliveryAttempt
	}

	return DomainEventDelivery{Event: event, MessageID: messageID, Attempt: attempt}, nil
}
