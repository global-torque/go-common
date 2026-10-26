# db

Import path: `github.com/global-torque/go-common/db/v2`

PostgreSQL setup around `pgx/v5`: connection pools, direct connections,
configuration validation, query logging, UTC session time zone, and PostgreSQL
notifications.

## Use For

- Standard PostgreSQL pool creation from env.
- Direct `pgx.Conn` creation when a pool is not appropriate.
- Shared DB logging through the go-common logger.
- PostgreSQL `LISTEN` subscriptions.

## Do Not Use For

- MySQL, SQLite, or other storage engines. `Config.Type` validates only
  `postgres`.
- ORM-style CRUD. Use `github.com/global-torque/go-common/orm/v2` for lightweight
  Squirrel helpers.

## Key APIs

- `Config`
- `New(ctx) (*DB, error)`
- `MustNew(ctx) *DB`
- `NewDB(pool, log) *DB`
- `BeginScope(ctx, pgx.TxOptions) (context.Context, pgx.Tx, error)`
- `NewPool(ctx)`
- `NewPoolFromConfig(ctx, pgConfig, log)`
- `NewConn(ctx)`
- `NewConnFromConfig(ctx, pgConfig, log)`
- `GetConfigPool(log)`
- `GetConfigConn(log)`
- `GetConnString(cfg)`
- `Subscribe(ctx, topicName)`
- `Repository`
- `NewDBLogger(log)`
- `CleanSQL`

## Configuration

Config prefix is `DB`.

- `DB_TYPE`: required, must be `postgres`
- `DB_HOST`: default `localhost`
- `DB_PORT`: default `5432`
- `DB_USER`: required
- `DB_PASSWORD`: required
- `DB_DATABASE`: required
- `DB_APP_NAME`: required, mapped to `application_name`
- `DB_SSL_MODE`: default `disable`
- `DB_MIN_CONNECTIONS`: default `4`
- `DB_MAX_CONNECTIONS`: default `16`
- `DB_MAX_CONN_LIFETIME`: default `3600` seconds
- `DB_MAX_RETRIES`: default `5`
- `DB_LOG_LEVEL`: default `error`

## Wiring Pattern

```go
database, err := db.New(ctx)
if err != nil {
	return err
}
defer database.Close()

var name string
err = database.QueryRow(ctx, "select name from users where id=$1", id).Scan(&name)
```

`DB` embeds `*pgxpool.Pool`. Call database methods on `DB` to participate in contextual transactions.

## Transaction scopes

`BeginScope` starts one root transaction and returns its context and transaction handle.
Pass the returned context into every repository call and commit or roll back the handle when work finishes.
The server module's `middleware.NewAtomic` manages this lifecycle for Echo requests.

`Exec`, `Query`, `QueryRow`, `SendBatch`, and `CopyFrom` automatically use the scoped transaction.
`Begin` and zero-option `BeginTx` create pgx savepoints. Existing `orm.WithTx` calls therefore work inside a scope;
releasing their savepoints cannot commit the root transaction. Without a scope, methods retain normal pool behavior.

Scopes belong to the underlying pool, so different `DB` wrappers sharing one pool can participate.
Using a different pool, creating a duplicate scope, specifying nested transaction options, or calling `Acquire` or
`AcquireFunc` inside a scope returns an error. Scoped `Ping` uses the held transaction connection.
A completed scope returns `pgx.ErrTxClosed` and never falls back to the pool.

Execute transaction work sequentially and close rows and batches before completion. Explicit `.Pool` access,
raw connections, and transaction-control SQL bypass scope management. Scopes cover one PostgreSQL database and
provide neither distributed transactions nor automatic retries.

## Testing

Use `github.com/global-torque/go-common/db/v2/dbtests` for fixture setup and SQL
assertions in integration tests.

Set `ATOMIC_TEST_DATABASE_URL` to an isolated PostgreSQL database to run the scope integration tests:

```sh
ATOMIC_TEST_DATABASE_URL='postgres://localhost/test_database?sslmode=disable' go test -race -run Scope ./...
```

These tests use temporary tables and exercise a one-connection pool, savepoints, ORM helpers, and concurrent scopes.

## Gotchas

- `MustNew` logs fatal on connection failure; use `New` in libraries.
- Pool and direct connection setup execute `SET TIME ZONE 'UTC'`.
- Initial connection retries use exponential backoff and `DB_MAX_RETRIES`.
- `Subscribe` returns a channel of notification payload bytes and releases the
  acquired connection when the context is cancelled.
- Query tracing uses pgx `tracelog` and the shared logger.
