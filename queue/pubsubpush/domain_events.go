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

// DomainEventTransport identifies a Pub/Sub delivery after the push envelope,
// subscription, and transport fields have been validated. It is safe to use
// for audit writes even when the domain-event payload itself is invalid.
type DomainEventTransport struct {
	MessageID    string
	Subscription string
	Attempt      int
}

// DecodeDomainEventPush strictly decodes one Pub/Sub push envelope, validates
// its exact full subscription resource, and then validates the domain event
// payload and delivery metadata. This is the authoritative HTTP-boundary
// decoder for domain-event consumers.
func DecodeDomainEventPush(body io.Reader, expectedSubscription string) (DomainEventDelivery, error) {
	delivery, _, err := DecodeDomainEventPushWithTransport(body, expectedSubscription)
	return delivery, err
}

// DecodeDomainEventPushWithTransport is DecodeDomainEventPush plus trusted
// transport metadata on payload-validation failures. Metadata is returned only
// after strict envelope decoding and exact subscription validation succeed.
func DecodeDomainEventPushWithTransport(
	body io.Reader,
	expectedSubscription string,
) (DomainEventDelivery, DomainEventTransport, error) {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	var req PushRequest

	err := decoder.Decode(&req)
	if err != nil {
		return DomainEventDelivery{}, DomainEventTransport{},
			fmt.Errorf("%w: decode push envelope: %w", domainevents.ErrMalformedEvent, err)
	}

	var trailing json.RawMessage

	err = decoder.Decode(&trailing)
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return DomainEventDelivery{}, DomainEventTransport{},
				fmt.Errorf("%w: push envelope contains trailing JSON", domainevents.ErrMalformedEvent)
		}

		return DomainEventDelivery{}, DomainEventTransport{},
			fmt.Errorf("%w: decode trailing push data: %w", domainevents.ErrMalformedEvent, err)
	}

	err = ValidateSubscription(req.Subscription, expectedSubscription)
	if err != nil {
		return DomainEventDelivery{}, DomainEventTransport{}, err
	}

	transport, err := domainEventTransport(req)
	if err != nil {
		return DomainEventDelivery{}, DomainEventTransport{}, err
	}

	delivery, err := DecodeDomainEvent(req)
	if err != nil {
		return DomainEventDelivery{}, transport, err
	}

	delivery.Subscription = transport.Subscription

	return delivery, transport, nil
}

// DecodeDomainEvent strictly validates both the payload and the metadata
// copied by the Debezium outbox route.
func DecodeDomainEvent(req PushRequest) (DomainEventDelivery, error) {
	transport, err := domainEventTransport(req)
	if err != nil {
		return DomainEventDelivery{}, err
	}

	event, err := domainevents.DecodeV1(req.Message.Data)
	if err != nil {
		return DomainEventDelivery{}, err
	}

	err = event.ValidateDelivery(req.Message.Attributes, req.Message.OrderingKey)
	if err != nil {
		return DomainEventDelivery{}, err
	}

	return DomainEventDelivery{
		Event: event, MessageID: transport.MessageID, Attempt: transport.Attempt,
	}, nil
}

func domainEventTransport(req PushRequest) (DomainEventTransport, error) {
	messageID := strings.TrimSpace(req.Message.MessageID)
	if messageID == "" {
		return DomainEventTransport{},
			fmt.Errorf("%w: pubsub messageId is required", domainevents.ErrMalformedEvent)
	}

	if req.DeliveryAttempt != nil && *req.DeliveryAttempt < 1 {
		return DomainEventTransport{},
			fmt.Errorf("%w: deliveryAttempt must be positive", domainevents.ErrMalformedEvent)
	}

	attempt := 0
	if req.DeliveryAttempt != nil {
		attempt = *req.DeliveryAttempt
	}

	return DomainEventTransport{
		MessageID: messageID, Subscription: req.Subscription, Attempt: attempt,
	}, nil
}
