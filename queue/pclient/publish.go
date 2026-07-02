package pclient

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/global-torque/go-common/validator"
	"github.com/pkg/errors"
)

func (b *Client) PublishEvent(
	ctx context.Context, topic string, event Event,
) (*Message, error) {
	valid := validator.New()
	if err := valid.Verify(event, http.StatusPreconditionFailed); err != nil {
		return nil, err
	}
	attr := map[string]string{}
	return b.PublishToTopic(ctx, topic, event, attr)
}

func (b *Client) PublishWebhook(
	ctx context.Context, topic string, webhook Webhook,
) (*Message, error) {
	valid := validator.New()
	if err := valid.Verify(webhook, http.StatusPreconditionFailed); err != nil {
		return nil, err
	}
	attr := map[string]string{}
	return b.PublishToTopic(ctx, topic, webhook, attr)
}

func (b *Client) Publish(
	ctx context.Context, topic string, data any, attr map[string]string,
) (*Message, error) {
	return b.PublishToTopic(ctx, topic, data, attr)
}

func (b *Client) PublishToTopic(
	ctx context.Context, topicID string, data any, attr map[string]string,
) (*Message, error) {
	if b.client == nil {
		return nil, ErrNotConnected
	}

	ok, err := b.TopicExist(ctx, topicID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTopicConnect, err)
	}
	if !ok {
		b.log.Error().Err(err).Stack().Interface("topic", topicID).Msg(ErrTopicNotExists.Error())
		return nil, errors.Wrapf(ErrTopicNotExists, ": %s", topicID)
	}

	msg, err := NewMessage(data, attr)
	if err != nil {
		b.log.Error().Err(err).Stack().Interface("data", data).Interface("attr", attr).Msg(ErrUnmarshalPubSub.Error())
		return nil, err
	}

	t := b.client.Publisher(topicID)
	defer t.Stop()

	result := t.Publish(ctx, &pubsub.Message{
		Data:       msg.Data,
		Attributes: msg.Attributes,
	})

	msgID, err := result.Get(ctx)
	if err != nil {
		b.log.Err(err).Msg(ErrPublish.Error())
		return nil, fmt.Errorf("%w: %w", ErrPublish, err)
	}

	b.log.Debug().Msgf("Published message; msg ID: %v to %s", msgID, topicID)
	msg.ID = msgID
	msg.PublishTime = time.Now()

	return msg, nil
}
