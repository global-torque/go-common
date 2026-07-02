# context/keys

Import path: `github.com/global-torque/go-common/context/v2/keys`

Defines shared context keys and HTTP header names used by the server, logger,
database, queue, and HTTP utility packages.

## Use For

- Passing request IDs, IP addresses, message IDs, identity IDs, and log metadata
  through `context.Context`.
- Referring to shared header names such as `X-Request-Id`, `X-Forwarded-For`,
  and `X-Real-IP`.

## Do Not Use For

- Package-private values that do not need to interoperate with go-common.
- Large request-scoped objects that should be passed explicitly.

## Key APIs

- `ContextKey`
- `ContextStr`
- `GetAsString(ctx, key) string`
- `GetCtxValue(ctx, key) any`
- `SetCtxValue(ctx, key, value) context.Context`
- `SetCtxValues(ctx, values) context.Context`

## Shared Keys

- `RequestID`
- `IPAddress`
- `MSGID`
- `IdentityID`
- `LogInfo`
- `RequestLogID`
- `RequestIDStr`
- `IPAddressStr`
- `RequestTimeContextStr`

## Wiring Pattern

```go
ctx = keys.SetCtxValue(ctx, keys.RequestID, requestID)
requestID := keys.GetAsString(ctx, keys.RequestID)
```

## Testing

Create a context with the values needed by the package under test:

```go
ctx := keys.SetCtxValue(context.Background(), keys.MSGID, "message-id")
```

## Gotchas

- `SetCtxValue` takes a `ContextKey`, while some middleware stores values under
  `ContextStr` keys such as `IPAddressStr`.
- `GetAsString` returns an empty string when the value is missing or not a
  string.
