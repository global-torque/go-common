# response

Import path: `github.com/global-torque/go-common/response`

Shared application error type and message helpers for consistent client-facing
errors across HTTP, CLI, worker, and app layers.

## Use For

- Expected user-facing failures.
- Validation or domain errors that should carry an HTTP status at the boundary.
- Error message maps shaped as `{field: [messages...]}`.

## Do Not Use For

- Internal-only errors that should stay wrapped until an adapter maps them.
- Logging sensitive internal details to clients.

## Key APIs

- `Error`
- `ErrorMessages`
- `New`
- `NewError`
- `BadRequest`
- `NotFound`
- `InternalError`
- `ErrBadRequest`
- `ErrUnauthorized`
- `ErrInternalError`
- `SingleErrorMessage`
- `MessagesFromAny`

## Wiring Pattern

App code returns `*response.Error` for expected failures:

```go
if item == nil {
	return response.NotFound(nil, "item not found")
}
```

Echo handlers pass errors to `server.ErrorResponse`, which knows how to
serialize `*response.Error`.

## Testing

Use `Message.Map()` to compare messages without depending on JSON encoding:

```go
assert.Equal(t, map[string][]string{"name": {"missing"}}, err.Message.Map())
```

## Gotchas

- `New` returns an `Error` value; helper constructors generally return
  `*Error`.
- Message maps are defensively copied.
- Empty messages marshal as `{}`.
- Default messages use the `__error__` key.
