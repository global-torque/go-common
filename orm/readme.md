# orm

Import path: `github.com/global-torque/go-common/orm/v2`

Lightweight PostgreSQL helpers built on pgx and Squirrel. This is not a full
ORM; it provides explicit CRUD helpers, a pure `Save` helper, transaction
wrapping, and change tracking.

## Use For

- Small repository methods that need generated SQL with pgx.
- Generic CRUD over simple model contracts.
- Explicit insert/update persistence with model-provided maps.
- Transaction wrappers that handle rollback, panic, and commit errors.

## Do Not Use For

- Domain side effects, Pub/Sub history logs, hooks, callbacks, or implicit model
  reflection.
- Non-PostgreSQL repositories.
- Models that need generated IDs scanned into non-`int` values without custom
  handling.

## Key APIs

- `Repository`
- `Beginner`
- `DefaultFields`
- `RetrieveOne`
- `RetrieveAll`
- `Create`
- `Update`
- `Exists`
- `Delete`
- `Save`
- `SaveModel`
- `PrimaryKeyer`
- `ChangeResetter`
- `SaveResult`
- `ChangeSet`
- `WithTx`

## Model Contracts

CRUD helpers expect the model pointer type to implement:

```go
type model interface {
	SetID(id any)
	Fields() []string
	Table() string
}
```

`Save` uses explicit methods:

```go
type SaveModel interface {
	Table() string
	GetID() any
	SetID(id any)
	InsertValues() map[string]any
	Changes() map[string]any
}
```

## Wiring Pattern

```go
user, err := orm.RetrieveOne[User](ctx, repo, sq.Eq{"id": id})
```

```go
result, err := orm.Save(ctx, repo, user)
```

Use `ChangeSet` in models that track changed columns:

```go
u.changes.Set("name", name)
```

## Testing

Stub the small `Repository` interface in unit tests. Package tests show the
expected generated SQL for retrieve, create, update, delete, save, and
transaction paths.

## Gotchas

- `Create` and `Save` scan returned IDs into `int`.
- `RetrieveOne` wraps both `ErrRecordNotFound` and `pgx.ErrNoRows`.
- `RetrieveAll` returns a non-nil empty slice.
- `Update` and `Delete` reject empty predicates and tautologies such as `1=1`.
- `Save` only inserts `InsertValues` or updates `Changes`; it does not reflect
  over all struct fields.
- `WithTx` rolls back with `context.WithoutCancel(ctx)` on callback errors or
  panic.
