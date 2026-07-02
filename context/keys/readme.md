# context/keys

Import path: `github.com/global-torque/go-common/context/keys`

Shared context keys and HTTP header constants used across go-common packages.

## Use For

- Request IDs, message IDs, identity IDs, IP addresses, and logger metadata.
- Header names used by HTTP and queue code.

## Key APIs

- `GetAsString`
- `GetCtxValue`
- `SetCtxValue`
- `SetCtxValues`
- `ContextKey`
- `ContextStr`

## Wiring Pattern

```go
ctx = keys.SetCtxValue(ctx, keys.RequestID, requestID)
requestID := keys.GetAsString(ctx, keys.RequestID)
```

## Gotchas

- Some values are stored under `ContextKey`; some header-like values use
  `ContextStr`.
- Missing or non-string values return `""` from `GetAsString`.
