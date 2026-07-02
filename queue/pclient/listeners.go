package pclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gpubsub "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	backoff "github.com/cenkalti/backoff/v4"
	"github.com/global-torque/go-common/context/v2/keys"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxRetries         = 100
	maxDeliveryAttempt = 10
	// SubscriptionRetryTimeout bounds how long getSubscriptionRetry keeps
	// trying to (re)connect to a subscription before giving up. A deleted
	// subscription that is recreated within this window reconnects
	// transparently; one that stays gone surfaces an error to the caller
	// (instead of busy-looping for ~15 minutes on the default backoff).
	SubscriptionRetryTimeout = 60 * time.Second
)

func verifyDeliveryAttempt(msg *gpubsub.Message) bool {
	// ToDo
	// For some reason right now message does not goes in dead letter queue
	// Fix dead letter queue settings in GCP
	// For now we just ask message to stop working with it
	if msg.DeliveryAttempt != nil && *msg.DeliveryAttempt > maxDeliveryAttempt {
		msg.Ack()
		return false
	}

	return true
}

func (b *Client) getSubscriptionRetry(ctx context.Context, subscription, topic string) (*gpubsub.Subscriber, error) {
	expo := backoff.NewExponentialBackOff()
	expo.MaxElapsedTime = SubscriptionRetryTimeout
	sub, err := backoff.RetryWithData(
		func() (*gpubsub.Subscriber, error) {
			b.log.Info().Msgf("Connecting to subscription %s/%s", topic, subscription)
			return b.getSubscription(ctx, subscription, topic)
		},
		backoff.WithContext(
			backoff.WithMaxRetries(expo, maxRetries),
			ctx,
		),
	)
	if err != nil {
		b.log.Error().Stack().Err(err).Msg(ErrNotConnected.Error())
		return nil, err
	}
	return sub, nil
}

func (b *Client) getSubscription(ctx context.Context, subscription, topic string) (*gpubsub.Subscriber, error) {
	if b.client == nil {
		return nil, ErrNotConnected
	}

	ok, err := b.TopicExist(ctx, topic)
	if err != nil {
		b.log.Error().Err(err).Str("topic", topic).Msg(ErrTopicConnect.Error())
		return nil, fmt.Errorf("%w: %w", ErrTopicConnect, err)
	}
	if !ok {
		b.log.Error().Err(err).Str("topic", topic).Msg(ErrTopicNotExists.Error())
		return nil, fmt.Errorf("%w: %s", ErrTopicNotExists, topic)
	}

	_, err = b.client.SubscriptionAdminClient.GetSubscription(ctx, &pubsubpb.GetSubscriptionRequest{
		Subscription: b.subscriptionPath(subscription),
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			b.log.Error().Err(err).Str("subscription", subscription).Msg(ErrSubscriptionNotExist.Error())
			return nil, fmt.Errorf("%w: %w", ErrSubscriptionNotExist, err)
		}
		b.log.Error().Err(err).Str("subscription", subscription).Msg(ErrConnectSubscription.Error())
		return nil, fmt.Errorf("%w: %w", ErrConnectSubscription, err)
	}
	return b.client.Subscriber(subscription), nil
}

func (b *Client) ListenRawMsgs(
	ctx context.Context,
	subscription, topic string,
	callback func(ctx context.Context, msg Message) error,
) error {
	sub, err := b.getSubscriptionRetry(ctx, subscription, topic)
	if err != nil {
		return err
	}
	return b.listenRawGoroutine(ctx, callback, sub)
}

func (b *Client) listenRawGoroutine(
	ctx context.Context,
	callback func(ctx context.Context, msg Message) error,
	sub *gpubsub.Subscriber,
) error {
	// Start consuming messages from the subscription
	b.log.Trace().Msgf("connected to subscription %s listen messages", sub.ID())
	err := sub.Receive(ctx, func(ctx context.Context, msg *gpubsub.Message) {
		if !verifyDeliveryAttempt(msg) {
			return
		}

		// Unmarshal the message data into a struct
		m := Message{
			ID:          msg.ID,
			Data:        msg.Data,
			PublishTime: msg.PublishTime,
			Attempt:     msg.DeliveryAttempt,
			Attributes:  msg.Attributes,
		}

		ctx = keys.SetCtxValue(ctx, keys.MSGID, msg.ID)
		b.log.Trace().Str("msg", string(m.Data)).Msgf("received message")
		err := callback(ctx, m)
		if err != nil {
			b.log.Error().Err(err).Msg(ErrReceiveCallback.Error())
			msg.Nack()
			return
		}
		msg.Ack()
	})
	if err != nil {
		b.log.Error().Stack().Err(err).Msg(ErrReceiveSubscription.Error())
	}
	return err
}

func (b *Client) ListenWebhooks(
	ctx context.Context, subscription,
	topic string,
	callback func(ctx context.Context, msg Webhook) error,
) error {
	sub, err := b.getSubscriptionRetry(ctx, subscription, topic)
	if err != nil {
		return err
	}

	return b.listenWebhookGoroutine(ctx, callback, sub)
}

func (b *Client) listenWebhookGoroutine(
	ctx context.Context,
	callback func(ctx context.Context, msg Webhook) error,
	sub *gpubsub.Subscriber,
) error {
	// Start consuming messages from the subscription
	b.log.Trace().Msgf("connected to subscription %s listen for webhooks", sub.ID())
	err := sub.Receive(ctx, func(ctx context.Context, msg *gpubsub.Message) {
		if !verifyDeliveryAttempt(msg) {
			return
		}

		webhook := Webhook{}
		if err := json.Unmarshal(msg.Data, &webhook); err != nil {
			b.log.Error().Err(err).Interface("data", string(msg.Data)).Msg(ErrUnmarshalPubSub.Error())
			msg.Nack()
			return
		}
		webhook.ID = msg.ID
		webhook.Attempt = msg.DeliveryAttempt

		ctx = SetDefaultWebhookCtx(ctx, webhook)
		b.log.Trace().Interface("msg", webhook).Msgf("received webhook")
		err := callback(ctx, webhook)
		if err != nil {
			b.log.Error().Err(err).Msg(ErrReceiveCallback.Error())
			msg.Nack()
			return
		}
		msg.Ack()
	})
	if err != nil {
		b.log.Error().Stack().Err(err).Msg(ErrReceiveSubscription.Error())
	}
	return err
}

func (b *Client) ListenEvents(
	ctx context.Context,
	subscription,
	topic string,
	callback func(ctx context.Context, msg Event) error,
) error {
	sub, err := b.getSubscriptionRetry(ctx, subscription, topic)
	if err != nil {
		return err
	}
	return b.listenEventGoroutine(ctx, callback, sub)
}

func (b *Client) listenEventGoroutine(
	ctx context.Context,
	callback func(ctx context.Context, msg Event) error,
	sub *gpubsub.Subscriber,
) error {
	// Start consuming messages from the subscription
	b.log.Trace().Msgf("connected to subscription %s listen for events", sub.ID())
	err := sub.Receive(ctx, func(ctx context.Context, msg *gpubsub.Message) {
		if !verifyDeliveryAttempt(msg) {
			return
		}

		event := Event{}
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			b.log.Error().Err(err).Interface("data", string(msg.Data)).Msg(ErrUnmarshalPubSub.Error())
			msg.Nack()
			return
		}
		event.ID = msg.ID
		event.Attempt = msg.DeliveryAttempt

		ctx = SetDefaultEventCtx(ctx, event)

		b.log.Trace().Interface("msg", event).Msgf("received event")
		err := callback(ctx, event)
		if err != nil {
			b.log.Error().Err(err).Msg(ErrReceiveCallback.Error())
			msg.Nack()
			return
		}
		msg.Ack()
	})
	if err != nil {
		b.log.Error().Stack().Err(err).Msg(ErrReceiveSubscription.Error())
	}
	return err
}
