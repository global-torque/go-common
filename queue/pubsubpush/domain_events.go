package pubsubpush

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	domainevents "github.com/global-torque/go-common/queue/v2/domainevents"
)

// DomainEventDelivery keeps immutable business identity separate from the
// Pub/Sub transport identity assigned to this particular publication.
type DomainEventDelivery struct {
	Event        domainevents.DomainEventV1
	MessageID    string
	Subscription string
	Attempt      int
}

// DecodeDomainEventPush strictly decodes one Pub/Sub push envelope, validates
// its exact full subscription resource, and then validates the domain event
// payload and delivery metadata. This is the authoritative HTTP-boundary
// decoder for domain-event consumers.
func DecodeDomainEventPush(body io.Reader, expectedSubscription string) (DomainEventDelivery, error) {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	var req PushRequest

	err := decoder.Decode(&req)
	if err != nil {
		return DomainEventDelivery{}, fmt.Errorf("%w: decode push envelope: %w", domainevents.ErrMalformedEvent, err)
	}

	var trailing json.RawMessage

	err = decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return DomainEventDelivery{}, fmt.Errorf("%w: push envelope contains trailing JSON", domainevents.ErrMalformedEvent)
		}

		return DomainEventDelivery{}, fmt.Errorf("%w: decode trailing push data: %w", domainevents.ErrMalformedEvent, err)
	}

	err = ValidateSubscription(req.Subscription, expectedSubscription)
	if err != nil {
		return DomainEventDelivery{}, err
	}

	delivery, err := DecodeDomainEvent(req)
	if err != nil {
		return DomainEventDelivery{}, err
	}

	delivery.Subscription = req.Subscription

	return delivery, nil
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
