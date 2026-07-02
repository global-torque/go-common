package qtests

import (
	"context"
	"fmt"
	"time"

	"github.com/global-torque/go-common/configurator/v2"
	"github.com/global-torque/go-common/queue/v2/pclient"
)

type contextKey string

const queueKey contextKey = "queue"

const fixtureOperationTimeout = 5 * time.Second

type Fixture struct {
	topic        string
	subscription string
	filePath     string
}

func NewFixture(topic, subscription, filePath string) Fixture {
	return Fixture{
		topic:        topic,
		subscription: subscription,
		filePath:     filePath,
	}
}

type FixturesManager struct {
	queue    *pclient.Client
	ctx      context.Context
	initErr  error
	fixtures []Fixture
}

func NewFixturesManager(ctx context.Context, fixtures ...Fixture) FixturesManager {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := configurator.LoadDotEnv(); err != nil {
		return FixturesManager{
			ctx:      ctx,
			initErr:  fmt.Errorf("load .env: %w", err),
			fixtures: fixtures,
		}
	}

	client, err := pclient.New(ctx)
	if err != nil {
		return FixturesManager{
			ctx:      ctx,
			initErr:  fmt.Errorf("create pubsub client: %w", err),
			fixtures: fixtures,
		}
	}
	return FixturesManager{
		queue:    client,
		ctx:      ctx,
		fixtures: fixtures,
	}
}

func (f FixturesManager) operationContext() (context.Context, context.CancelFunc) {
	baseCtx := f.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	return context.WithTimeout(baseCtx, fixtureOperationTimeout)
}

func (f FixturesManager) Close() {
	if f.queue != nil {
		f.queue.Close()
	}
}

func (f FixturesManager) CleanAndApply() error {
	if f.initErr != nil {
		return f.initErr
	}
	if f.queue == nil {
		return pclient.ErrNotConnected
	}

	for _, fixture := range f.fixtures {
		err := f.Clean(fixture.topic, fixture.subscription)
		if err != nil {
			return err
		}
	}
	// ToDo
	// Push data to the subscriptions
	// return PubSubF.LoadFixtures(fixture.filePath)
	return nil
}

func (f FixturesManager) SetCTX(ctx context.Context) context.Context {
	return context.WithValue(ctx, queueKey, f.queue)
}

func (f FixturesManager) Delete(topic string, subscription string) error {
	if f.initErr != nil {
		return f.initErr
	}
	if f.queue == nil {
		return pclient.ErrNotConnected
	}

	if subscription != "" {
		ctx, cancel := f.operationContext()
		err := f.queue.DeleteSubscription(ctx, subscription)
		cancel()
		if err != nil {
			return fmt.Errorf("failed delete subscription: %w", err)
		}
	}

	ctx, cancel := f.operationContext()
	err := f.queue.DeleteTopic(ctx, topic)
	cancel()
	if err != nil {
		return fmt.Errorf("failed delete topic: %w", err)
	}

	return nil
}

func (f FixturesManager) Clean(topic string, subscription string) error {
	if f.initErr != nil {
		return f.initErr
	}
	if f.queue == nil {
		return pclient.ErrNotConnected
	}

	if err := f.Delete(topic, subscription); err != nil {
		return err
	}

	ctx, cancel := f.operationContext()
	_, err := f.queue.CreateTopic(ctx, topic)
	cancel()
	if err != nil {
		return fmt.Errorf("failed create topic: %w", err)
	}

	ctx, cancel = f.operationContext()
	_, err = f.queue.CreateSubscription(ctx, subscription, topic)
	cancel()
	if err != nil {
		return fmt.Errorf("failed create subscription: %w", err)
	}

	return err
}
