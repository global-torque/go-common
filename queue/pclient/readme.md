# queue/pclient

Import path: `github.com/global-torque/go-common/queue/v2/pclient`

Google Cloud Pub/Sub v2 client wrapper for topic/subscription management,
publishing, and pull listeners.

## Use For

- Creating and deleting Pub/Sub topics and subscriptions.
- Publishing raw data, events, or webhooks.
- Listening to raw messages, event DTOs, or webhook DTOs.
- Building default logging context from received Pub/Sub data.

## Do Not Use For

- Route-based worker wiring when `queue.PubSubListener` is enough.
- Push subscription HTTP handlers. Use `queue/pubsubpush`.

## Key APIs

- `Client`
- `New(ctx)`
- `Close`
- `CreateTopic`
- `DeleteTopic`
- `CreateSubscription`
- `DeleteSubscription`
- `TopicExist`
- `SubscriptionExist`
- `Publish`
- `PublishToTopic`
- `PublishEvent`
- `PublishWebhook`
- `ListenRawMsgs`
- `ListenEvents`
- `ListenWebhooks`
- `Message`
- `Event`
- `Webhook`
- `SetDefaultEventCtx`
- `SetDefaultWebhookCtx`

## Configuration

Config prefix is `PUBSUB`.

- `PUBSUB_PROJECT_ID`: required
- `PUBSUB_SERVICE_ACCOUNT_CREDENTIALS`: optional auth credentials file
- `PUBSUB_EMULATOR_HOST`: when set, credentials are not used and the emulator is
  targeted

## Wiring Pattern

```go
client, err := pclient.New(ctx)
if err != nil {
	return err
}
defer client.Close()

msg, err := client.PublishWebhook(ctx, topic, pclient.Webhook{
	Action:  "file.created",
	Object:  "file",
	Service: "files",
	Data:    payload,
})
```

Listeners ack when the callback returns nil and nack when it returns an error.

## Testing

Integration tests require:

- `PUBSUB_PROJECT_ID`
- `PUBSUB_EMULATOR_HOST`

The package tests skip when the emulator is not configured or reachable.

## Gotchas

- `PublishEvent` and `PublishWebhook` validate DTOs with HTTP 412 on validation
  errors.
- `CreateSubscription` enables exactly-once delivery and a 5 to 10 minute retry
  policy.
- Messages with delivery attempt greater than 10 are acked and skipped.
- `SetDefaultWebhookCtx` reads request ID and IP from webhook headers.
