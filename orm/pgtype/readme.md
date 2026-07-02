# orm/pgtype

Import path: `github.com/global-torque/go-common/orm/pgtype`

Compatibility wrappers for legacy nullable pgtype-style values on top of pgx
v5 codecs.

## Use For

- Migrating old model fields that used `Status`, `Null`, `Present`, and
  `Undefined` semantics.
- PostgreSQL text, timestamptz, json, and jsonb values that need explicit null
  status.

## Do Not Use For

- New code that can use native pointers, pgx v5 types, or clearer domain value
  objects.
- JSON unmarshalling helpers. This package only implements raw JSON marshal
  behavior for `JSON` and `JSONB`.

## Key APIs

- `Status`
- `Undefined`
- `Null`
- `Present`
- `InfinityModifier`
- `Infinity`
- `None`
- `NegativeInfinity`
- `Text`
- `Timestamptz`
- `JSON`
- `JSONB`

## Wiring Pattern

Use pgx codec methods:

```go
var txt pgtype.Text
err := txt.ScanText(pgxpgtype.Text{String: "hello", Valid: true})
value, err := txt.TextValue()
```

For JSONB compatibility:

```go
var js pgtype.JSONB
err := js.Set(map[string]any{"a": 1})
var out map[string]any
err = js.AssignTo(&out)
```

## Testing

Tests cover null and present status, infinity timestamps, raw JSON marshal, and
undefined encode errors.

## Gotchas

- Undefined values return errors when encoded.
- `JSON` and `JSONB` marshal raw payloads, not wrapper struct fields.
- `JSONB.Set` rejects non-pointer `JSON` and `JSONB` inputs to avoid accidental
  copies.
