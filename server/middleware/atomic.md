# Atomic database requests

`NewAtomic` gives POST, PUT, PATCH, and DELETE handlers one PostgreSQL transaction.
Participating database calls use the request context. A successful response is buffered
until the transaction commits. Install the middleware once after global recovery and
authentication middleware:

```go
import (
    "context"

    "github.com/global-torque/go-common/db/v2"
    commonmw "github.com/global-torque/go-common/server/v2/middleware"
    "github.com/labstack/echo/v4"
    echomw "github.com/labstack/echo/v4/middleware"
)

func newHTTP(ctx context.Context) (*echo.Echo, *db.DB, error) {
    database, err := db.New(ctx)
    if err != nil {
        return nil, nil, err
    }
    atomic, err := commonmw.NewAtomic(database, commonmw.AtomicConfig{})
    if err != nil {
        database.Close()
        return nil, nil, err
    }

    e := echo.New()
    e.Use(echomw.Recover(), commonmw.SetLogger, atomic)
    e.POST("/items", func(c echo.Context) error {
        ctx := c.Request().Context()
        if _, err := database.Exec(ctx, "INSERT INTO items (name) VALUES ($1)", "example"); err != nil {
            return err
        }
        return c.JSON(201, map[string]string{"status": "created"})
    })
    return e, database, nil // The service owns database.Close at shutdown.
}
```

With the shared server, call `server.AddDefaultMiddlewares` before
`httpServer.Echo.Use(atomic)`. Enable the existing recovery middleware or install
`echo/middleware.Recover` before Atomic. Route authentication inside Atomic is also
valid. Atomic logs lifecycle and delivery errors through the request's structured
logger, even when Echo's own logger is disabled.

Group and route registration also work: `e.Group("/api", atomic)` or
`e.POST("/items", handler, atomic)`. Install only one instance on any request path;
nesting global and route Atomic middleware is rejected.

## Configuration

| Field | Default | Meaning |
| --- | --- | --- |
| `Methods` | POST, PUT, PATCH, DELETE | HTTP methods to include; GET, HEAD, and OPTIONS pass through |
| `Skipper` | nil | Optional Echo skipper; true bypasses Atomic |
| `TxOptions` | `pgx.TxOptions{}` | Existing PostgreSQL isolation/access defaults |
| `MaxResponseBytes` | 1048576 | Maximum buffered body size, in bytes |
| `RollbackTimeout` | 5 seconds | Cancellation-independent cleanup deadline |

Zero response and cleanup limits select defaults. Negative limits or a missing
database return a constructor error. Use a skipper for endpoints that must stream
or exceed the configured response limit. Configure the middleware explicitly at
service startup; it is not automatically added by `AddDefaultMiddlewares`.

## Transaction and response contract

- Handler success with a 2xx/3xx status commits, then replays its headers and body.
- A returned error rolls back and goes through the normal Echo error handler.
- A 4xx/5xx response returned with nil rolls back, then replays that response.
- A panic rolls back with an independent cleanup context, restores Echo state, and
  propagates unchanged to outer recovery middleware.
- Cancellation, overflow, invalid response framing, or database lifecycle failure
  discards the buffered success. Lifecycle failures return a generic 500 and log
  their underlying errors.
- Echo before/after hooks run during staging; successful replay does not run them
  again. First effective status and headers win. Baseline CORS/request-ID headers
  survive discarded responses. Outer BodyDump and RequestLogger observe the final
  response and metadata.
- Streaming (`c.Stream`, SSE, transfer encoding), flushing, hijacking, server push,
  trailers, informational statuses, prohibited bodies, and malformed/mismatched
  Content-Length are rejected before commit. These failures stay recorded even
  if a handler ignores the returned error. A 304 may describe a representation
  length but must have no body.
- The middleware wraps `echo.Context` to reject `Stream`. Handlers should use the
  Echo interface or context values; concrete custom-context assertions inside
  Atomic need adaptation. Do not replace the response or bypass its writer.

Calls to adapter `Exec`, `Query`, `QueryRow`, `SendBatch`, and `CopyFrom` use the
scoped transaction. `Begin` inside a scope uses native pgx savepoints, including
existing `orm.WithTx` helpers. Releasing a savepoint does not commit the request;
a later rollback removes those writes too. Nested `BeginTx` with nonzero options,
duplicate root scopes, wrong-pool access, `Acquire`, and `AcquireFunc` fail.
Scoped `Ping` runs on the same transaction, including with a one-connection pool.
A completed transaction context returns `pgx.ErrTxClosed` and cannot fall back to
the pool. ORM helper signatures remain unchanged.

Application calls must retain `c.Request().Context()` and use the upgraded shared
adapter. Raw `.Pool` access, acquired connections, manual transaction-control SQL,
and work using another context bypass the guarantee. Run transaction work
sequentially; close rows and batches before returning. Do not launch detached
database work from the handler. Response hooks should only change metadata or
observe the response, since they run before the transaction outcome is known.

Handlers must return failures or write error statuses. Swallowed errors, including
`c.Error` after a response has already been staged, cannot reliably trigger
rollback. The guarantee covers participating writes in one PostgreSQL database;
it does not change isolation, retry transactions, or undo external side effects.
A connection loss during commit can leave the outcome unknown. Response delivery
failure after commit is logged and cannot undo the committed transaction; Atomic
does not retry or write a replacement error after delivery begins.

## Upgrade and verification

Release the changed `db/v2` and `server/v2` modules using the repository's normal
module release procedure. Services must pin both released versions and run their
own tests with `GOWORK=off` before adopting this middleware. The legacy evm-api
adapter needs a compatibility bridge before its queries participate. This change
does not automatically install middleware or upgrade deployed services.

For local integration checks, point `ATOMIC_TEST_DATABASE_URL` at a disposable
PostgreSQL database. The new tests use temporary tables, including a deferred
unique constraint that fails at commit after preparing a success response:

```sh
export ATOMIC_TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/test?sslmode=disable'
(cd db && go test -race -count=1 -run TestScope ./...)
(cd server && go test -race -count=1 -run TestAtomic ./middleware)
(cd db && go vet ./... && golangci-lint run --new-from-rev HEAD ./...)
(cd server && go vet ./... && golangci-lint run --new-from-rev HEAD ./...)
```

Also run affected modules' full test suites and the workspace's normal tests. The
database module's existing tests require its documented `DB_*` environment.
Without the integration DSN, new PostgreSQL tests explicitly skip. Standalone
server tests compile against published adapter versions; the combined PostgreSQL
test explicitly skips if that version does not yet provide `BeginScope`. Run it
in the local workspace before release, then again with `GOWORK=off` after updating
the published adapter dependency. Never count that skip as integration evidence.
