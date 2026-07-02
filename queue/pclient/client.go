package pclient

import (
	"context"
	"fmt"
	"os"
	"time"

	gpubsub "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/global-torque/go-common/configurator/v2"
	"github.com/global-torque/go-common/logger/v2"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	pkgName         = "pubsub"
	minPubsubBackof = 5
	maxPubsubBackof = 10
)

type Client struct {
	client *gpubsub.Client // google cloud pubsub client
	log    logger.Logger   // client logger
	cfg    *Config         // client config
}

func New(ctx context.Context) (*Client, error) {
	cfg := Config{}
	log := logger.NewComponentLogger(ctx, pkgName)

	err := configurator.NewConfiguration(&cfg, pkgName)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfigParse, err)
	}

	opts := make([]option.ClientOption, 0, 1)
	if cfg.ServiceAccountCredentials != "" && os.Getenv("PUBSUB_EMULATOR_HOST") == "" {
		opts = append(opts, option.WithAuthCredentialsFile(option.ServiceAccount, cfg.ServiceAccountCredentials))
	}

	bclient, err := gpubsub.NewClient(ctx, cfg.ProjectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnection, err)
	}

	b := &Client{
		log:    log,
		cfg:    &cfg,
		client: bclient,
	}

	return b, nil
}

func (b *Client) CreateTopic(ctx context.Context, name string) (*pubsubpb.Topic, error) {
	b.log.Trace().Msgf("creating topic %s", name)
	if b.client == nil {
		return nil, ErrNotConnected
	}
	topic, err := b.client.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{Name: b.topicPath(name)})
	if err != nil {
		b.log.Error().Err(err).Interface("name", name).Msg(ErrTopicCreate.Error())
		return nil, fmt.Errorf("%w: %w", ErrTopicCreate, err)
	}
	return topic, nil
}

func (b *Client) DeleteTopic(ctx context.Context, name string) error {
	b.log.Trace().Msgf("deleting topic %s", name)
	if b.client == nil {
		return ErrNotConnected
	}

	err := b.client.TopicAdminClient.DeleteTopic(ctx, &pubsubpb.DeleteTopicRequest{Topic: b.topicPath(name)})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	return err
}

func (b *Client) DeleteSubscription(ctx context.Context, name string) error {
	b.log.Trace().Msgf("deleting subscription %s", name)
	if b.client == nil {
		return ErrNotConnected
	}

	err := b.client.SubscriptionAdminClient.DeleteSubscription(ctx, &pubsubpb.DeleteSubscriptionRequest{
		Subscription: b.subscriptionPath(name),
	})
	if status.Code(err) == codes.NotFound {
		return nil
	}
	return err
}

func (b *Client) CreateSubscription(ctx context.Context, name, topic string) (*pubsubpb.Subscription, error) {
	b.log.Trace().Msgf("creating subscription %s", name)
	if b.client == nil {
		return nil, ErrNotConnected
	}

	sub, err := b.client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:                      b.subscriptionPath(name),
		Topic:                     b.topicPath(topic),
		EnableExactlyOnceDelivery: true,
		RetryPolicy: &pubsubpb.RetryPolicy{
			MinimumBackoff: durationpb.New(time.Minute * minPubsubBackof),
			MaximumBackoff: durationpb.New(time.Minute * maxPubsubBackof),
		},
	})
	if err != nil {
		b.log.Error().Err(err).Interface("name", name).Msg(ErrCreateSubscription.Error())
		return nil, fmt.Errorf("%w: %w", ErrCreateSubscription, err)
	}
	return sub, nil
}

func (b *Client) TopicExist(ctx context.Context, topic string) (bool, error) {
	if b.client == nil {
		return false, ErrNotConnected
	}

	_, err := b.client.TopicAdminClient.GetTopic(ctx, &pubsubpb.GetTopicRequest{Topic: b.topicPath(topic)})
	if err == nil {
		return true, nil
	}
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	return false, err
}

func (b *Client) SubscriptionExist(ctx context.Context, sub string) (bool, error) {
	if b.client == nil {
		return false, ErrNotConnected
	}

	_, err := b.client.SubscriptionAdminClient.GetSubscription(ctx, &pubsubpb.GetSubscriptionRequest{
		Subscription: b.subscriptionPath(sub),
	})
	if err == nil {
		return true, nil
	}
	if status.Code(err) == codes.NotFound {
		return false, nil
	}
	return false, err
}

func (b *Client) Close() {
	if b.client != nil {
		if err := b.client.Close(); err != nil {
			b.log.Error().Err(err).Msg(ErrCloseConnection.Error())
		}
	}
}

func (b *Client) topicPath(topic string) string {
	return fmt.Sprintf("projects/%s/topics/%s", b.cfg.ProjectID, topic)
}

func (b *Client) subscriptionPath(subscription string) string {
	return fmt.Sprintf("projects/%s/subscriptions/%s", b.cfg.ProjectID, subscription)
}
