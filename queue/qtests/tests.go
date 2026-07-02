package qtests

import (
	"context"

	"github.com/global-torque/go-common/queue/pclient"
	"github.com/global-torque/go-common/tests"
)

func getQueue(t tests.TestContext) *pclient.Client {
	//nolint:forcetypeassert
	return t.Ctx.Value(queueKey).(*pclient.Client)
}

func SendPubSubEvent(topic string, body any, attr map[string]string) tests.SomeAction {
	return func(t tests.TestContext) error {
		ctx := t.Ctx
		if ctx == nil {
			ctx = context.Background()
		}

		ctx, cancel := context.WithTimeout(ctx, fixtureOperationTimeout)
		defer cancel()

		_, err := getQueue(t).PublishToTopic(ctx, topic, body, attr)
		return err
	}
}
