# queue

Import path: `github.com/global-torque/go-common/queue`

Route-based Google Pub/Sub pull listener for workers. It wraps
`queue/pclient` listeners and can optionally deduplicate event and webhook
deliveries.

## Use For

- Long-running pull-subscription workers.
- Routing Pub/Sub messages by route name to webhook, event, or raw-message
  callbacks.
- Message-level deduplication backed by service-owned storage.

## Do Not Use For

- Pub/Sub push HTTP endpoints. Use `queue/pubsubpush` and normal server routes.
- One-off publishing. Use `queue/pclient` directly.

## Key APIs

- `PubSubListener`
- `PubSubRoute`
- `Deduper`
- `New(routes)`
- `MustNew(routes)`
- `NewWithDeduper(routes, service, deduper)`
- `MustNewWithDeduper(routes, service, deduper)`
- `(*PubSubListener).Start(ctx)`
- `(*PubSubListener).Close()`
- `(*PubSubListener).AddRoutes(routes)`

## Configuration

- `PUBSUB_RECONNECT_WINDOW`: optional Go duration such as `5m`. Overrides the
  listener reconnect-or-panic window.
- Uses `queue/pclient` config for Pub/Sub connection.

## Wiring Pattern

```go
listener, err := queue.New([]queue.PubSubRoute{
	{
		Name:             "webhooks",
		Topic:            topic,
		Subscription:     subscription,
		WebhooksListener: processor.HandleWebhook,
	},
})
if err != nil {
	return err
}
defer listener.Close()

return listener.Start(ctx)
```

Valid route names are exactly:

- `webhooks`
- `events`
- `messages`

## Deduplication

`NewWithDeduper` wraps event and webhook callbacks with:

1. `Claim`
2. callback execution
3. `MarkProcessed` or `MarkFailed`

Raw message callbacks are not dedup-wrapped.

## Testing

Use `github.com/global-torque/go-common/queue/qtests` with a Pub/Sub emulator.

## Gotchas

- Invalid route names fail at `Start`.
- Listener reconnect failures eventually panic so a process supervisor can
  restart the worker.
- `Must...` constructors log fatal on setup errors; use `New...` in libraries.
