# validator

Import path: `github.com/global-torque/go-common/validator`

Wrapper around `github.com/go-playground/validator/v10` that returns
`*response.Error` with compact field-keyed messages.

## Use For

- Echo request DTO validation.
- App-level precondition validation that should return a chosen HTTP status.
- Shared validation messages for common tags.

## Do Not Use For

- Raw validator errors when callers need full `validator.ValidationErrors`
  details.
- Complex nested response shaping; this package currently flattens by field.

## Key APIs

- `New() *Validator`
- `Validator.Validate(i) error`
- `Validator.Verify(i, httpStatus) error`
- `ParamName`
- custom `path` tag

## Validation Tags

Uses `validate` struct tags. Supported custom messages include:

- `required`
- `email`
- `len`
- `min`
- `max`
- `gt`
- `gte`
- `oneof`
- `eq`
- `ssn`
- `dirpath`
- `path`

Field names come from `json`, then `param`, then `form` tags.

## Wiring Pattern

With Echo:

```go
e.Validator = validator.New()
if err := c.Validate(&req); err != nil {
	return server.ErrorResponse(c, err)
}
```

Direct use:

```go
if err := validator.New().Verify(dto, http.StatusPreconditionFailed); err != nil {
	return err
}
```

## Testing

Cast returned errors to `*response.Error` and compare `Message.Map()`.

## Gotchas

- `Validate` always returns HTTP 400 on validation failure.
- `Verify` lets callers choose the status code.
- `New` panics if custom validation registration fails.
