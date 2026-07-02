# db/dbtests

Import path: `github.com/global-torque/go-common/db/v2/dbtests`

PostgreSQL fixture and SQL assertion helpers for integration tests.

## Use For

- Cleaning tables and loading JSON fixtures before scenarios.
- Sharing a `*db.DB` through test context.
- Running SQL assertions that can retry while async workers finish.

## Do Not Use For

- Unit tests without a real PostgreSQL database.
- Database migrations.

## Key APIs

- `NewFixture(table, filePath)`
- `NewFixturesManager(ctx, fixtures...)`
- `FixturesManager.WithFixtures`
- `FixturesManager.CleanAndApply`
- `FixturesManager.Close`
- `FixturesManager.ExecQuery`
- `FixturesManager.SelectQuery`
- `RawSQL(query)`
- `SQL(query, expected...)`

## Configuration

Uses the same `DB_*` env variables as the `db` package. Constructor sets
`TZ=UTC` for tests.

## Wiring Pattern

```go
fixtures := []tests.FixturesManager{
	dbtests.NewFixturesManager(ctx,
		dbtests.NewFixture("users", "fixtures/users.json"),
	),
}

tests.RunTableTest(t, ctx, fixtures, tableTest)
```

## Fixture Files

`LoadFixture` looks under `/tests/<filePath>` relative to the current project.
Rows are loaded from JSON arrays. Unknown columns are ignored.

## Gotchas

- `Clean(table)` assumes a sequence named `<table>_id_seq`.
- `SQL` wraps the query as `select row_to_json(q)::jsonb from (...) as q`.
- `SQL` retries until expected values match or the retry budget is exhausted.
