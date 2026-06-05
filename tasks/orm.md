# Add `go-common/orm`

## Summary

Create a new standalone module `github.com/webdevelop-pro/go-common/orm` to own the lightweight ORM/query-helper layer currently in `../i-models/models`. Keep `go-common/db` focused on pgx setup. Keep Pub/Sub logging, notifications, history logs, and domain-specific `Save` side effects out of `orm`.

## Key Changes

- Add `orm/` as a new go-common module and add `./orm` to `go.work`.
- Public root API:
  - `Repository` with only `Query`, `Exec`, `QueryRow`.
  - `RetrieveOne`, `RetrieveAll`, `Create`, `Update`, `Exists`, `Delete`.
  - `WithTx(ctx, beginner, func(ctx, tx orm.Repository) error) error`.
  - `Save(ctx, repo, model) (SaveResult, error)` for pure persistence only.
- `WithTx`:
  - Accepts anything with `Begin(ctx) (pgx.Tx, error)`.
  - Rolls back on callback error or panic.
  - Commits only after callback success.
  - Returns wrapped begin/callback/rollback/commit errors.
- Add `orm/pgtype` v1 with `Status`, `Timestamptz`, `Text`, `JSON`, and `JSONB`.
- Do not implement `Set`, `Get`, `sql.Scanner`, `driver.Valuer`, or JSON unmarshal.
  `JSON`/`JSONB` implement `MarshalJSON` only so pgx JSON codecs encode the raw
  payload instead of the wrapper struct shape.

## Save API

- Use explicit model contracts; do not reflect over the whole struct in `Save`.

```go
type SaveModel interface {
	Table() string
	GetID() any
	SetID(any)
	InsertValues() map[string]any
	Changes() map[string]any
}

type PrimaryKeyer interface {
	PrimaryKey() string
}

type ChangeResetter interface {
	ResetChanges()
}

type SaveResult struct {
	Action       SaveAction
	Table        string
	PrimaryKey   string
	ID           any
	RowsAffected int64
}

type SaveAction string

const (
	SaveInserted SaveAction = "inserted"
	SaveUpdated  SaveAction = "updated"
)
```

- If `GetID()` is zero, `Save` inserts using `InsertValues()`, appends `RETURNING id`, scans the generated ID, and calls `SetID`.
- If `GetID()` is non-zero, `Save` updates by primary key using only `Changes()`.
- Default primary key is `id`; `PrimaryKey()` overrides it.
- Empty insert values return `ErrEmptyValues`.
- Empty update changes return `ErrNoChanges`.
- Update with zero affected rows returns `ErrRecordNotFound`.
- Update with more than one affected row returns an integrity error.
- After successful insert/update, call `ResetChanges()` if implemented.
- No Pub/Sub, callbacks, notifications, history logs, or business side effects.

## Change Tracking

- Add an ORM helper to replace repeated `updatedFields`, `fns`, and `GetValueByTag` boilerplate in new models.

```go
type ChangeSet struct {
	values map[string]any
}

func (c *ChangeSet) Set(column string, value any)
func (c *ChangeSet) Changes() map[string]any
func (c *ChangeSet) ResetChanges()
```

- Support Squirrel expressions as values, e.g. `changes.Set("balance", sq.Expr("balance + ?", amount))`.
- Existing domain `Save` methods can migrate later to build `Changes()` through `ChangeSet`, while still owning their own Pub/Sub/history behavior outside `orm.Save`.

## Compatibility

- Update `../i-models/models` as a compatibility wrapper around `orm`:
  - Delegate generic CRUD and `DefaultFields` to `orm`.
  - Do not keep `RetriveOne`, `RetriveAll`, existing error names, and Pub/Sub log helpers.
  - Preserve existing public import paths for current services.
- Do not migrate all domain model imports in the first implementation.
- Replace `../i-models/pgtype` immediately use new code `go-common/orm/pgtype`.

## Test Plan

- Add table-driven `orm` tests for CRUD SQL generation, empty predicate rejection, affected-row handling, not-found wrapping, and transaction commit/rollback/panic paths.
- Add `Save` tests for insert, update, custom primary key, empty values, empty changes, zero affected rows, multiple affected rows, and reset-after-success behavior.
- Add `ChangeSet` tests for clone safety, reset, overwrite behavior, and Squirrel expression values.
- Add `orm/pgtype` tests for scan/value/json/null behavior.
- Keep/adapt `../i-models/models` tests to prove wrapper compatibility.
- Follow `tasks/enhancements.md`: after each implementation iteration, run tests, create an independent `golang-pro` review subagent, address findings, and repeat until clean.

## Assumptions

- V1 keeps integer `id` behavior for `Create` and `Save`.
- `orm.Save` is a new pure persistence helper, not a replacement for existing domain `Save` methods.
- Reflection helpers remain available for explicit helper calls, but `Save` never updates all reflected fields implicitly.

## Implementation Status - 2026-06-05

- Added `github.com/webdevelop-pro/go-common/orm` with CRUD helpers, `Save`,
  `ChangeSet`, `WithTx`, and `orm/pgtype`; added `./orm` to `go.work`.
- Updated `../i-models/models` into a compatibility wrapper around `orm` for
  generic CRUD and `DefaultFields`.
- Removed `RetriveOne`/`RetriveAll`, removed legacy wrapper error names except
  the migration sentinel `ErrRecordNotFound`, and removed Pub/Sub log helpers
  from `i-models/models`.
- Deleted the old `../i-models/pgtype` package and moved model/service imports
  to `github.com/webdevelop-pro/go-common/orm/pgtype`.
- Moved active Pub/Sub log persistence into the owning payment/esign services.
- Updated `../i-payment-api`, `../i-escrow-api`, `../i-kyc-api`, and
  `../i-esign-api` for the new ORM/pgtype module and `RetrieveOne` spelling.

Validation completed:

- `go-common/orm`: `go test ./...`, `go vet ./...`, `go test -race ./...`,
  `golangci-lint run ./...`.
- `../i-models`: `go test ./...`, `go vet ./...`, `go test -race ./...`.
- Four API repos: `go vet ./...`, `go test -run '^$' ./...`,
  `go test -race -run '^$' ./...`.
- Full tests passed in `../i-kyc-api` and `../i-esign-api`.
- Full `../i-payment-api` and `../i-escrow-api` tests still require external
  integration state: payment needs the local service on `127.0.0.1:8086` and a
  clean fixture DB; escrow needs required DB env vars such as `DB_TYPE`.
- `golangci-lint` is blocked in `../i-models`, `../i-payment-api`,
  `../i-escrow-api`, and `../i-kyc-api` by their existing unsupported
  golangci config version.
