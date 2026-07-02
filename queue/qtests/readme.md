# queue/qtests

Import path: `github.com/global-torque/go-common/queue/qtests`

Pub/Sub integration-test fixtures and actions for the shared table-test runner.

## Use For

- Creating/deleting Pub/Sub emulator topics and subscriptions before scenarios.
- Publishing messages to topics inside `tests.RunTableTest`.
- Emulating Pub/Sub push delivery over HTTP.

## Do Not Use For

- Production Pub/Sub code.
- Unit tests that should not require the Pub/Sub emulator.

## Key APIs

- `NewFixture(topic, subscription, filePath)`
- `NewFixturesManager(ctx, fixtures...)`
- `FixturesManager.CleanAndApply`
- `FixturesManager.Delete`
- `FixturesManager.Clean`
- `FixturesManager.Close`
- `SendPubSubEvent`
- `SendPushWebhook`
- `SendPushEvent`
- `SendPushTo`

## Configuration

- Loads dotenv through `configurator.LoadDotEnv`.
- Requires `PUBSUB_PROJECT_ID`.
- Requires a reachable `PUBSUB_EMULATOR_HOST` for integration tests.
- Push helpers default to `HOST` and `PORT`.

## Wiring Pattern

```go
fixtures := qtests.NewFixturesManager(ctx,
	qtests.NewFixture(topic, subscription, ""),
)

tests.RunTableTest(t, ctx, []tests.FixturesManager{fixtures}, tableTest)
```

## Testing Actions

```go
qtests.SendPubSubEvent(topic, body, attrs)
qtests.SendPushWebhook("/pubsub/webhook", webhook, attrs)
```

## Gotchas

- Fixture `filePath` is currently stored but not loaded.
- `CleanAndApply` creates the topic and subscription.
- Use unique topic/subscription names in tests to avoid emulator state
  collisions.
