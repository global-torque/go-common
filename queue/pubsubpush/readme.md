# queue/pubsubpush

Import path: `github.com/global-torque/go-common/queue/pubsubpush`

DTOs and Echo middleware for Google Pub/Sub push subscription HTTP requests.

## Use For

- Decoding Pub/Sub push envelopes in HTTP handlers.
- Dropping push deliveries after a code-level attempt limit.

## Do Not Use For

- Pull subscription listeners. Use `queue` or `queue/pclient`.
- Replacing Pub/Sub dead-letter configuration. `MaxAttempts` is a safety net.

## Key APIs

- `PushRequest`
- `PushMessage`
- `MaxAttempts(n)`

## Wire Format

`PushRequest.DeliveryAttempt` is top-level on the push envelope. It is not
inside `PushMessage`.

## Wiring Pattern

```go
route.Route{
	Method:  http.MethodPost,
	Path:    "/pubsub/webhook",
	Handler: h.HandlePush,
	Middlewares: []echo.MiddlewareFunc{
		pubsubpush.MaxAttempts(10),
	},
}
```

## Testing

Use `queue/qtests.SendPushWebhook`, `SendPushEvent`, or `SendPushTo` to post a
push envelope to a service under test.

## Gotchas

- `MaxAttempts` reads and rewinds the request body, so downstream handlers can
  decode it normally.
- If the request body cannot be parsed as a push envelope, the middleware passes
  it through unchanged.
- When delivery attempt is greater than `n`, it returns HTTP 204 and does not
  call the handler.
