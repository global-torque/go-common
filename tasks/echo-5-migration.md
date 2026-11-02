# Echo v5 migration plan

## Goal

Migrate `go-common`, `kyc-api`, and `escrow-api` from Echo v4 to Echo v5 without
changing HTTP contracts, Pub/Sub acknowledgement behavior, middleware order, or
Fx lifecycle behavior.

Use a direct replacement, not a transitional migration path. A short service
downtime during deployment is acceptable. After validation, stop the old
service if needed and start the Echo v5 build in its place.

Keep the migration straightforward:

- preserve existing routes, methods, headers, status codes, request and response
  bodies, authentication behavior, and Pub/Sub acknowledgement behavior;
- do not add parallel Echo v4 and v5 runtimes, compatibility adapters, feature
  flags, canary deployment, blue-green deployment, or staged traffic switching;
- do not create rollback code, rollback procedures, or a rollback release;
- fix any deployment issue forward in the Echo v5 implementation.

This plan is based on independent source reviews of all three repositories and
the Echo v5.4.2 source and migration guide. It was prepared on 2026-10-29.

## Target versions and compatibility decision

Pin exact versions during the migration:

```text
github.com/labstack/echo/v5         v5.4.2
github.com/labstack/echo-contrib/v5 v5.0.1
Go                                  1.25 or newer
```

Echo v5 requires Go 1.25 or newer. The affected `go-common` modules already use
Go 1.25 or newer. Escrow uses Go 1.25.8. KYC declares Go 1.25.8 and prefers the
Go 1.26.1 toolchain. Keep each repository's current toolchain declaration; no Go
toolchain migration is required for Echo v5.

`go-common/server/v2` and `go-common/queue/v2` expose Echo v4 types in public
APIs. Echo v4 and v5 handler types are not assignable to each other. Publish the
Echo v5 implementations as new major versions:

```text
github.com/global-torque/go-common/server/v3
github.com/global-torque/go-common/queue/v3
```

Do not silently replace Echo v4 inside the existing `/v2` modules. That would
break semantic versioning and every consumer that implements `route.Configurator`
or uses `pubsubpush.MaxAttempts`.

Keep the existing `/v2` tags available for services that have not migrated.
Changing the module path in the current `server` and `queue` directories for the
new release does not remove old published tags.

## Current repository impact

### `go-common`

Four modules directly require Echo v4:

| Module | Current dependency | Migration work |
| --- | --- | --- |
| `server` | Echo v4 and `echo-contrib` | Public API, server setup, middleware, error handling, tests, and docs |
| `queue` | Echo v4 | `pubsubpush.MaxAttempts` and its tests |
| `logger/example` | Echo v4 | Web example and module files |
| `docker` | Echo v4 | Main dependency seed |

The main public boundary is `server/route/route.go`:

```go
type Route struct {
    Method      string
    Path        string
    Handler     echo.HandlerFunc
    Middlewares []echo.MiddlewareFunc
}
```

The import behind these types changes from `echo/v4` to `echo/v5`. This is why
services and the shared server must move together.

The highest-risk implementation is `server/middleware/atomic.go`. It embeds the
v4 `echo.Context` interface and copies and modifies `echo.Response` internals.
Echo v5 makes `Context` a concrete struct and changes `Context.Response()` to
return `http.ResponseWriter`.

### `kyc-api`

KYC directly uses Echo in:

- authenticated, internal, and Pub/Sub push handlers;
- authenticated route middleware declarations;
- direct handler and route tests;
- Plaid request header constants.

The shared `server/v2` route and middleware types prevent a KYC-only migration.
All three binaries must compile after migration:

```text
cmd/http
cmd/worker
cmd/cloudrun
```

Preserve these Pub/Sub push contracts:

- malformed webhook envelopes return `204` and are acknowledged;
- permanent webhook payload errors return `204`;
- transient webhook failures return `500` for retry;
- malformed domain event contracts retain their current `400` or `403` result.

### `escrow-api`

Escrow has a wider direct dependency than KYC:

- all HTTP handlers accept `echo.Context`;
- `internal/app/app.go` passes Echo contexts into application methods;
- North Capital code uses Echo header constants;
- route declarations and push handler tests use Echo types directly.

All HTTP and Cloud Run binaries must compile after migration. The legacy pull
worker does not construct Echo, but it must still compile against the selected
`queue` major version.

Preserve these Cloud Run contracts:

- malformed provider webhooks return `204`;
- malformed or permanent domain event contracts return `400`;
- domain events from an unexpected subscription return `403`;
- transient domain event failures return `500`;
- `/pubsub/events` remains absent.

## Confirmed Echo v5 changes that affect this code

| Echo v4 behavior | Echo v5 behavior | Required action |
| --- | --- | --- |
| `echo.Context` is an interface | `echo.Context` is a struct | Change handlers and callbacks to `*echo.Context` |
| `Context.Response()` returns `*echo.Response` | It returns `http.ResponseWriter` | Use `echo.UnwrapResponse` when status or commit state is needed |
| Error handler is `(err, context)` | Error handler is `(context, err)` | Change the shared handler signature |
| Most status errors are `*echo.HTTPError` | 404/405 can be immutable status-coded errors | Use `echo.StatusCode(err)` instead of only `errors.As(..., *echo.HTTPError)` |
| `HTTPError.Message` is `any` | It is `string` | Remove type assertions and `fmt.Sprint` workarounds |
| `NewHTTPError(code)` accepts an omitted message | v5.4.2 requires `NewHTTPError(code, message)` | Pass `http.StatusText(code)` or return an Echo status sentinel |
| Echo owns a custom logger and banner flags | Echo uses `*slog.Logger`; banner fields are removed | Configure a discard `slog` logger because startup uses `http.Server` directly |
| `BodyLimit` accepts strings such as `20M` | It accepts an `int64` byte count | Parse the existing environment value before middleware registration |
| `BodyDumpHandler` receives three arguments | It also receives the downstream error | Add the fourth callback argument |
| Recover supports `LogLevel` and `LogErrorFunc` | Those fields are removed | Log returned `PanicStackError` in a small go-common middleware |
| CORS uses `AllowOriginFunc` | v5 supports explicit `AllowOrigins` | Reject `*` and migrate every environment to explicit origins |
| Test context uses `SetParamNames/Values` | v5 uses `SetPathValues` | Update direct route tests |
| `echo-contrib` has no major suffix | Echo v5 support is in `echo-contrib/v5` | Change the module and import paths together |

`echo-contrib/echoprometheus` is deprecated upstream, but replacing it with a
different metrics package is not required for this migration. Use
`echo-contrib/v5/echoprometheus` first. Replace it in a separate change after
behavior and metric names have been compared.

## Step-by-step migration

### 1. Record the Echo v4 baseline

Before changing dependencies, run the current canonical checks and save any
pre-existing failures. Do not classify an existing failure as an Echo v5
regression.

From `go-common`, run the affected modules independently:

```sh
(cd server && go vet ./... && go test -count=1 ./...)
(cd queue && go vet ./... && go test -count=1 ./...)
(cd logger/example && go vet ./... && go test -count=1 ./...)
(cd docker && go vet ./... && go test -count=1 ./...)
```

The current root `make.sh` does not loop through modules and has no `vet`
command. Do not use it as proof that all affected modules passed.

From each service, run its canonical test command:

```sh
# kyc-api
./make.sh test

# escrow-api
./make.sh test
```

Before implementing v3, verify release ancestry. The current branch does not
contain `server/v2.0.3`, even though consumers already use that release. Stop
and incorporate the released changes before building v3:

```sh
git merge-base --is-ancestor server/v2.0.3 HEAD
git merge-base --is-ancestor queue/v2.1.0 HEAD
```

Both commands must succeed. In particular, preserve the released
`CORS_ALLOW_HEADERS` configuration and its tests from `server/v2.0.3`. Do not
tag v3 from a commit that drops behavior from a published v2 release.

### 2. Create the new shared module majors

Update the module declarations and all self-imports:

```go
// server/go.mod
module github.com/global-torque/go-common/server/v3

// queue/go.mod
module github.com/global-torque/go-common/queue/v3
```

Change imports under `server/` from `server/v2/...` to `server/v3/...`.
Update `go.work` replacement paths if required by the final module layout.

The queue major bump affects DTO, listener, and client import paths even though
only `pubsubpush.MaxAttempts` exposes Echo. Update each service's queue imports
as one mechanical change after `queue/v3` is published.

### 3. Update shared dependency manifests

Apply these dependency changes to `server/go.mod`, `queue/go.mod`,
`logger/example/go.mod`, and `docker/go.mod` as applicable:

```text
github.com/labstack/echo/v4              -> github.com/labstack/echo/v5 v5.4.2
github.com/labstack/echo-contrib         -> github.com/labstack/echo-contrib/v5 v5.0.1
github.com/labstack/echo/v4/middleware   -> github.com/labstack/echo/v5/middleware
```

Then run `go mod tidy` from each affected module. Do not run one root-level
`go mod tidy`; `go-common` is a multi-module repository.

Update the main Docker dependency seed:

```text
docker/main.go
docker/go.mod
docker/go.sum
```

Do not hand-edit generated `go.sum` entries. Regenerate them through Go module
commands.

### 4. Migrate shared handler and middleware signatures

Change every v4 import and every context argument in `server`, `queue`, tests,
and the logger web example.

```go
// Before
func Healthcheck(c echo.Context) error {
    return c.String(http.StatusOK, "OK")
}

// After
func Healthcheck(c *echo.Context) error {
    return c.String(http.StatusOK, "OK")
}
```

Middleware keeps the same outer shape, but its handler receives a pointer:

```go
func CheckIdentityID(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c *echo.Context) error {
        identityID := c.Request().Header.Get(echo.HeaderAuthorization)
        if identityID == "" {
            return c.JSON(http.StatusUnauthorized, map[string][]string{
                "__error__": {"empty identity_id"},
            })
        }

        ctx := keys.SetCtxValue(c.Request().Context(), keys.IdentityID, identityID)
        c.SetRequest(c.Request().WithContext(ctx))
        return next(c)
    }
}
```

Apply the same pointer change to:

- auth, JWT, request-time, IP, logger, healthcheck, and Pub/Sub middleware;
- CORS, request ID, body dump, request logger, and recovery callbacks;
- direct handler tests and test helpers;
- `server/route.Route` fields through the v5 import.

### 5. Adapt `server.NewServer` and CORS

Echo v5 removes `HideBanner`, `HidePort`, and the old logger's `SetLevel`.
This server already starts a standard `http.Server`, so Echo never prints its
startup banner. Preserve the current no-native-logging behavior with a discard
`slog` logger:

```go
echoLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
e := echo.NewWithConfig(echo.Config{Logger: echoLogger})
e.Validator = validator.New()
```

Echo v5 rejects `*` when credentials are enabled. This migration intentionally
does not support wildcard origins. Before upgrading the shared server, replace
every KYC `CORS_ALLOWED_ORIGINS=*` value with the explicit origins needed by
that environment. Update every affected `.env` file, Compose manifest, and
deployment environment as part of the migration. This includes:

```text
kyc-api/.local.env
kyc-api/.env.tests
kyc-api/tests/http/.env
kyc-api/tests/worker/.env
kyc-api/docker-compose.yaml
deployment secrets and environment configuration outside the repository
```

Environment variables are strings, but the existing `envconfig` dependency
already decodes comma-separated values into string slices. Change the server
configuration field to a slice instead of adding another parsing function:

```go
type Config struct {
    CORSAllowedOrigins []string `required:"true" split_words:"true"`
}
```

Write explicit origins as a comma-separated value in each affected environment
file. Do not use JSON or shell array syntax:

```dotenv
CORS_ALLOWED_ORIGINS=https://admin.example.com,https://app.example.com
```

Pass the decoded slice directly to Echo v5's ordinary allowlist:

```go
e.Use(echoMW.CORSWithConfig(echoMW.CORSConfig{
    Skipper:          corsSkipper,
    AllowOrigins:     cfg.CORSAllowedOrigins,
    AllowCredentials: true,
    AllowMethods:     allowedMethods,
    AllowHeaders:     allowedHeaders,
}))
```

Validate the decoded slice at startup. Reject an empty list, empty entries, and
`*` with a clear error before Echo constructs the middleware. Add tests for one
allowed origin, multiple allowed origins, one rejected origin, rejected `*`,
and the existing OPTIONS route skip behavior.

### 6. Preserve the body-limit environment contract

Echo v5 accepts bytes rather than a string such as `20M`. Keep
`HTTP_BODY_LIMIT` compatible during this migration. Parse it once at startup
and return a clear startup error for invalid values.

The smallest compatibility implementation can continue using the parser that
Echo v4 used:

```go
import gommonbytes "github.com/labstack/gommon/bytes"

func bodyLimit() (int64, error) {
    value := os.Getenv("HTTP_BODY_LIMIT")
    if value == "" {
        value = "20M"
    }

    limit, err := gommonbytes.Parse(value)
    if err != nil {
        return 0, fmt.Errorf("parse HTTP_BODY_LIMIT %q: %w", value, err)
    }
    return limit, nil
}
```

Register `echoMW.BodyLimit(limit)` after parsing. A later cleanup can replace
the parser and remove `gommon`, but that is not required to move to Echo v5.

### 7. Adapt body dump, request logging, and recovery

The body dump callback now receives the downstream error:

```go
func BodyDumpHandler(c *echo.Context, incoming, outgoing []byte, handlerErr error) {
    // Keep the existing body sanitization and zerolog fields.
    if handlerErr != nil {
        logger.FromCtx(c.Request().Context(), "body_dump").Debug().Err(handlerErr).
            Msg("request completed with error")
    }
}
```

Echo v5's `RequestLoggerConfig` no longer has `LogError`. Remove that field.
Keep `HandleError`, and inspect `RequestLoggerValues.Error` in
`LogValuesFunc` when choosing the log level.

Echo v5 recovery returns `*middleware.PanicStackError` instead of calling
`LogErrorFunc`. Preserve current panic logging with a small outer middleware:

```go
func panicLogger(log logger.Logger) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c *echo.Context) error {
            err := next(c)
            var panicErr *echoMW.PanicStackError
            if errors.As(err, &panicErr) {
                log.Error().Err(panicErr.Unwrap()).Bytes("stacktrace", panicErr.Stack).
                    Msg("panic recover")
            }
            return err
        }
    }
}

srv.Echo.Use(panicLogger(srv.log))
srv.Echo.Use(echoMW.RecoverWithConfig(echoMW.RecoverConfig{
    StackSize: 10 << 10,
}))
```

Register `panicLogger` before `RecoverWithConfig`. Echo applies middleware in
reverse around handlers, so the logger then receives the recovery error.

### 8. Correct the shared error handler for Echo v5

Do not migrate the current handler by changing only its argument order. A v5
404 or 405 can implement `HTTPStatusCoder` without being `*echo.HTTPError`.
Checking only `*echo.HTTPError` would turn valid 404 and 405 results into 500.

Use this shape:

```go
func (s *HTTPServer) httpErrorHandler(c *echo.Context, err error) {
    if resp, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && resp.Committed {
        return
    }

    var appErr *response.Error
    if !errors.As(err, &appErr) {
        code := echo.StatusCode(err)
        if code == 0 {
            code = http.StatusInternalServerError
        }

        message := http.StatusText(code)
        var echoErr *echo.HTTPError
        if errors.As(err, &echoErr) && echoErr.Message != "" {
            message = echoErr.Message
        }

        appErr = &response.Error{
            StatusCode: code,
            Message:    response.SingleErrorMessage(message),
        }
    }

    var writeErr error
    if c.Request().Method == http.MethodHead {
        writeErr = c.NoContent(appErr.StatusCode)
    } else {
        writeErr = c.JSON(appErr.StatusCode, appErr.Message)
    }
    if writeErr != nil {
        s.log.Err(writeErr).Msg("cannot send error response")
    }
}
```

Add focused tests for:

- `response.Error` serialization;
- `echo.NewHTTPError` with a message;
- `echo.ErrNotFound` returning 404;
- method-not-allowed returning 405 and an `Allow` header;
- unknown errors returning 500;
- committed responses not being rewritten;
- HEAD responses containing no body.

Also change calls such as this:

```go
// Before
return echo.NewHTTPError(http.StatusInternalServerError)

// After
return echo.NewHTTPError(
    http.StatusInternalServerError,
    http.StatusText(http.StatusInternalServerError),
)
```

### 9. Redesign Atomic around `http.ResponseWriter`

Do not try to embed `*echo.Context` and pass a custom context to `next`.
`echo.HandlerFunc` requires exactly `*echo.Context`, so the v4 `atomicContext`
substitution no longer works.

Do not swap the writer inside the original `*echo.Response`. In Echo v5, outer
middleware such as BodyDump uses `Context.SetResponse` wrappers. Mutating the
original response and replaying through the outer wrapper would capture the
body twice and run response hooks twice.

Instead, save the current response writer, install a separate Echo response for
staging, and restore the saved writer before one replay. Outer middleware then
sees only replayed bytes. Middleware inside Atomic sees only staged bytes.
Replay must not add a second callback cycle on either side.

```go
func runAtomic(
    c *echo.Context,
    next echo.HandlerFunc,
    database TransactionStarter,
    config AtomicConfig,
) error {
    request := c.Request()
    replayWriter := c.Response()

    response, err := echo.UnwrapResponse(replayWriter)
    if err != nil {
        return atomicFailure(c, fmt.Errorf("atomic unwrap response: %w", err))
    }
    if response.Committed || request.Context().Value(atomicScopeKey{}) != nil {
        return atomicFailure(c, errAtomicScope)
    }

    buffer := newAtomicBuffer(replayWriter.Header(), config.MaxResponseBytes)
    stagedResponse := echo.NewResponse(buffer, c.Logger())
    c.SetResponse(stagedResponse)

    defer func() {
        c.SetResponse(replayWriter)
        c.SetRequest(request)
    }()

    // Begin the transaction and install its context as in the current code.
    handlerErr := next(c)

    if handlerErr == nil && !stagedResponse.Committed && buffer.err == nil {
        stagedResponse.WriteHeader(http.StatusOK)
    }

    // Validate, then roll back or commit as in the current code. Restore the
    // original chain before one successful replay through all outer writers.
    c.SetResponse(replayWriter)
    c.SetRequest(request)
    copyAtomicHeaders(replayWriter.Header(), buffer.snapshot)
    replayWriter.WriteHeader(buffer.status)
    n, deliveryErr := replayWriter.Write(buffer.body.Bytes())
    if deliveryErr == nil && n != buffer.body.Len() {
        deliveryErr = io.ErrShortWrite
    }

    // Preserve the current behavior after transaction commit: log delivery
    // failures because a second HTTP error response is no longer safe.
    if deliveryErr != nil {
        atomicLog(c, deliveryErr, "atomic response delivery after transaction completion failed")
    }
    return nil
}
```

This is a structural sample, not a drop-in replacement. Retain all current
transaction begin, rollback, commit, cancellation, bounded-buffer, and logging
branches around it.

Echo v5 creates one unavoidable behavior decision: Atomic can no longer
override only `Context.Stream`. The smallest migration is to allow finite
`c.Stream` output to use the same bounded response buffer. Continue rejecting:

- flush, hijack, and server push;
- trailers and transfer encoding;
- `text/event-stream`;
- responses larger than `MaxResponseBytes`.

Document and test the finite-stream behavior. If strict rejection of every
`c.Stream` call is required, stop and design a separate transaction-aware
handler API. Do not infer streaming from write count or reader type because
those checks are unreliable.

Port the existing atomic tests before changing implementation. Update response
metadata assertions to use:

```go
resp, err := echo.UnwrapResponse(c.Response())
require.NoError(t, err)
require.Equal(t, http.StatusCreated, resp.Status)
```

Retain coverage for commit, rollback, panic cleanup, short writes, response
limits, hooks, body dump, request logging, flush, hijack, push, trailers, and
invalid content lengths. Echo `Before` runs once when a response commits, while
`After` runs after every `Write`. Test one-write and multi-write handlers
separately. Assert that replay does not duplicate either callback pattern and
BodyDump receives one copy of the final response body.

### 10. Migrate Pub/Sub push middleware and examples

`queue/pubsubpush.MaxAttempts` only needs the new import and context pointer:

```go
func MaxAttempts(n int) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c *echo.Context) error {
            // Keep the existing read, rewind, decode, log, and 204 behavior.
            return next(c)
        }
    }
}
```

Update `logger/example/web/main.go` in the same way. Update all README and local
skill examples that still show `echo/v4` or `echo.Context`.

From `go-common/server`, run the real PostgreSQL Atomic test explicitly.
Ordinary module tests skip it
when `ATOMIC_TEST_DATABASE_URL` is absent:

```sh
docker run --rm --detach --name go-common-atomic-postgres \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=atomic \
  -p 55432:5432 postgres:17-alpine
until docker exec go-common-atomic-postgres pg_isready -U postgres -d atomic; do sleep 1; done
ATOMIC_TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:55432/atomic?sslmode=disable' \
  go test -count=1 -run '^TestAtomicPostgres$' ./middleware
docker rm --force go-common-atomic-postgres
```

### 11. Verify the `go-common` candidate

Run these checks from each affected module:

```sh
go vet ./...
golangci-lint run
go test -count=1 ./...
go test -race -count=1 ./...
```

Build the Docker seed. Verify that no Echo v4 module remains in the `server`,
`queue`, `logger/example`, or main `docker` module graphs:

```sh
go list -m all | grep 'github.com/labstack/echo'
go mod graph | grep 'github.com/labstack/echo'
```

Expected results contain `/v5` and do not contain `/v4`.

Do not tag yet. First migrate and test both consumers against the local
candidate modules as described below.

### 12. Migrate `kyc-api` against the local candidate

Update `go.mod` to require `server/v3`, `queue/v3`, and Echo v5. Add temporary,
uncommitted replacements while the shared modules are unpublished:

```text
replace github.com/global-torque/go-common/server/v3 => ../go-common/server
replace github.com/global-torque/go-common/queue/v3 => ../go-common/queue
```

Run `go mod tidy`, then update imports and handler signatures.

KYC handler example:

```go
// Before
func (hg *HandlerGroup) PushWebhook(c echo.Context) error {

// After
func (hg *HandlerGroup) PushWebhook(c *echo.Context) error {
```

The handler body can remain unchanged. Preserve its `204` and `500` decisions.

Route declarations keep their structure after all imports use the same major:

```go
import (
    "github.com/global-torque/go-common/server/v3/middleware"
    "github.com/global-torque/go-common/server/v3/route"
    "github.com/labstack/echo/v5"
)

func (hg *HandlerGroup) GetRoutes() []route.Route {
    middlewares := []echo.MiddlewareFunc{middleware.CheckIdentityID}
    // Existing route values remain unchanged.
}
```

Update direct handler tests:

```go
func performPush(t *testing.T, handler echo.HandlerFunc, body []byte) int {
    t.Helper()

    e := echo.New()
    req := httptest.NewRequest(http.MethodPost, "/pubsub/test", bytes.NewReader(body))
    req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
    rec := httptest.NewRecorder()

    require.NoError(t, handler(e.NewContext(req, rec)))
    return rec.Code
}
```

Replace removed path-parameter test APIs:

```go
ctx.SetPath(route.Path)
ctx.SetPathValues(echo.PathValues{
    {Name: "profile_id", Value: "42"},
})
```

Plaid code uses Echo only for HTTP header names. Remove that framework coupling:

```go
req.Header.Set("Content-Type", "application/json")
req.Header.Set(string(keys.RequestIDStr), requestID)
```

Build every runtime and retain the mock provider tag:

```sh
go build -tags=mockplaidkyc ./cmd/http
go build -tags=mockplaidkyc ./cmd/worker
go build -tags=mockplaidkyc ./cmd/cloudrun
go test -count=1 -tags=mockplaidkyc ./...
./make.sh test
```

The last two commands cover package tests but skip integration packages unless
`RUN_INTEGRATION_TESTS=true` and running services are available. Run the actual
HTTP and Cloud Run suites through the repository Compose stack:

```sh
export I_MIGRATION_SERVICE_DIR=../migration-service
docker compose down
docker compose up --abort-on-container-exit --exit-code-from tests tests
docker compose down
```

The `tests` service starts the required dependencies, waits for healthy HTTP
and Cloud Run services, and sets `RUN_INTEGRATION_TESTS=true` for both suites.

### 13. Migrate `escrow-api` against the local candidate

Update `go.mod` and imports to `server/v3`, `queue/v3`, and Echo v5. Use the same
temporary local replacements as KYC. Change all HTTP and push handler arguments
to `*echo.Context`.

Minimal handler migration:

```go
func (h HTTPServer) EscrowCreateAccount(c *echo.Context) error {
    req := new(domain.RequestEscrowCreateAccount)
    if err := c.Bind(req); err != nil {
        return h.ErrorBadReqestResponse(c, err)
    }

    res, err := h.app.EscrowCreateAccount(c.Request().Context(), req)
    if err != nil {
        return h.ErrorResponse(c, err)
    }
    return c.JSON(http.StatusCreated, res)
}
```

For the first migration release, change existing application methods from
`echo.Context` to `*echo.Context` without combining a large application-layer
refactor. Follow with a separate change that moves binding, file extraction,
and validation into the HTTP port and passes `context.Context` plus explicit
values to the application.

Fix the current unsafe request-ID assertion while touching these signatures.
The shared request-ID middleware already stores the value in standard context:

```go
ctx := c.Request().Context()
requestID := keys.GetAsString(ctx, keys.RequestID)
```

Use this instead of:

```go
requestID := c.Get(echo.HeaderXRequestID).(string)
```

The latter panics when middleware is absent in a focused test or alternate
composition root.

Update the local bad-request mapper to use a pointer target and v5's string
message:

```go
var echoErr *echo.HTTPError
if errors.As(err, &echoErr) {
    resp = map[string]any{"__error__": []string{echoErr.Message}}
}
```

The current code uses a value target and a v4-only string assertion. Do not
carry either pattern into v5.

North Capital code also uses Echo only for header constants. Replace those with
transport-owned values:

```go
req.Header.Set("Content-Type", w.FormDataContentType())
req.Header.Set("Content-Length", strconv.FormatInt(req.ContentLength, 10))
req.Header.Set(string(keys.RequestIDStr), requestID)
```

Build and test with the required provider tag:

```sh
go build -tags=mocknorthcapital ./cmd/http
go build -tags=mocknorthcapital ./cmd/worker
go build -tags=mocknorthcapital ./cmd/cloudrun
./make.sh test
./make.sh test docker
```

The current script has no distinct `unit`, `http`, or `cloudrun` test targets.
`./make.sh test docker` starts both services and executes the HTTP and worker
integration suites. Do not run those suites in parallel because they share
database state.

### 14. Publish shared modules and pin both consumers

After both migrated services pass against local candidates, follow the module
release controls in `go-common/AGENTS.md`:

```sh
(cd server && go vet ./... && golangci-lint run --new-from-rev server/v2.0.3 && go test -count=1 ./... && go test -race -count=1 ./...)
(cd queue && go vet ./... && golangci-lint run --new-from-rev queue/v2.1.0 && go test -count=1 ./... && go test -race -count=1 ./...)
```

After explicit release authorization, create annotated `server/v3.0.0` and
`queue/v3.0.0` tags and push those exact tag refs. Tagging and pushing are not
implied by implementing the migration.

Verify the published modules:

```sh
GOWORK=off go mod download -json github.com/global-torque/go-common/server/v3@v3.0.0
GOWORK=off go mod download -json github.com/global-torque/go-common/queue/v3@v3.0.0
```

In both services, remove the temporary replacements, pin the published
`v3.0.0` versions, run `go mod tidy`, and repeat all build, package, and
integration checks. Do not publish service manifests containing local
replacements or unpublished module versions.

### 15. Run cross-repository acceptance checks

The migration is complete only when all of these conditions hold:

1. `go-common/server`, `queue`, `logger/example`, and `docker` contain no Echo
   v4 imports or module edges.
2. KYC and escrow contain no Echo v4 imports or module edges.
3. Shared 404, 405, application error, HEAD, and committed-response tests pass.
4. CORS preflight and explicit OPTIONS behavior remain unchanged.
5. Body limit accepts the existing `20M` format and rejects oversized bodies.
6. Panic recovery logs the stack and still reaches the shared error handler.
7. Atomic transaction and response tests pass under Echo v5.
8. KYC webhook and domain-event acknowledgement statuses remain unchanged.
9. Escrow webhook and domain-event acknowledgement statuses remain unchanged.
10. `cmd/http`, `cmd/worker`, and `cmd/cloudrun` build in both services with
    their normal mock-provider tags.
11. Service integration tests use published `server/v3` and `queue/v3` modules,
    not local `replace` directives.

Replace one service at a time with its Echo v5 build. Brief downtime is
acceptable during each replacement. After startup, verify `/healthcheck`,
`/metrics`, one normal JSON endpoint, one validation failure, one authenticated
route, and each Pub/Sub push endpoint before replacing the next service.

Do not build an operational migration or rollback path around the replacement.
The deployed service must expose the same API and behavior after the direct
swap. If verification finds a problem, correct the Echo v5 implementation and
deploy it again.

## Migration order summary

```text
1. Record v4 baseline
2. Reconcile the latest v2 tag ancestry
3. Build and verify go-common server/v3 and queue/v3 candidates
4. Update logger example and the main Docker dependency seed
5. Migrate and verify kyc-api with temporary local replacements
6. Migrate and verify escrow-api with temporary local replacements
7. Publish and independently download the shared module versions
8. Pin both services to published versions and repeat verification
9. Run cross-service and deployed smoke checks
```

Do not run a mixed-major intermediate release. A process can technically contain
both Echo modules, but v4 and v5 route, handler, context, and middleware types do
not interoperate. Replace each service directly after its Echo v5 checks pass.

## Upstream references

- Echo v5 API changes: <https://github.com/labstack/echo/blob/v5.4.2/API_CHANGES_V5.md>
- Echo v5.4.2 source: <https://github.com/labstack/echo/tree/v5.4.2>
- Echo v5 error handling: <https://echo.labstack.com/guide/error-handling/>
- Echo contrib v5: <https://github.com/labstack/echo-contrib>
