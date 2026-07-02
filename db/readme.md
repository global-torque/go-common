# db

Import path: `github.com/global-torque/go-common/db`

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
- ORM-style CRUD. Use `github.com/global-torque/go-common/orm` for lightweight
  Squirrel helpers.

## Key APIs

- `Config`
- `New(ctx) (*DB, error)`
- `MustNew(ctx) *DB`
- `NewDB(pool, log) *DB`
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

`DB` embeds `*pgxpool.Pool`, so use pgx pool methods directly.

## Testing

Use `github.com/global-torque/go-common/db/dbtests` for fixture setup and SQL
assertions in integration tests.

## Gotchas

- `MustNew` logs fatal on connection failure; use `New` in libraries.
- Pool and direct connection setup execute `SET TIME ZONE 'UTC'`.
- Initial connection retries use exponential backoff and `DB_MAX_RETRIES`.
- `Subscribe` returns a channel of notification payload bytes and releases the
  acquired connection when the context is cancelled.
- Query tracing uses pgx `tracelog` and the shared logger.
