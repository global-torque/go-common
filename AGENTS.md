# AGENTS.md

## Purpose

This repository is a multi-module Go library for shared Global Torque service plumbing. Use it before adding custom config loading, logging, HTTP server/error mapping, validation, PostgreSQL setup, Pub/Sub helpers, test actions, or lightweight ORM code in a dependent service.

## Repository Model

- There is no single root Go module. Discover modules with `find . -name go.mod -not -path './.git/*' | sort`.
- `go.work` includes all local modules for development. Run Go commands from the affected module directory, or use root `./make.sh test`, `./make.sh vet`, and `./make.sh lint` to loop modules.
- Import by each module path, for example `github.com/global-torque/go-common/db/v2`, not a root `go-common` package.
- Many modules depend on published sibling versions in `go.mod`; when editing across modules, check `go.work` and local replaces before assuming dependency behavior.

## Package Map

- `github.com/global-torque/go-common/configurator/v2`: env and `.env` loading through `envconfig`.
- `github.com/global-torque/go-common/context/v2/keys`: shared typed context keys and header names.
- `github.com/global-torque/go-common/db/v2`: pgx connection/pool setup, DB logging, PostgreSQL `LISTEN`.
- `github.com/global-torque/go-common/db/v2/dbtests`: PostgreSQL fixture manager and SQL table-test actions.
- `github.com/global-torque/go-common/httputils/v2`: HTTP request builders, multipart requests, response sending, forwarded IP extraction.
- `github.com/global-torque/go-common/logger/v2`: zerolog wrapper with severity and service-context hooks.
- `github.com/global-torque/go-common/logger/v2/echo_google_cloud`: logger variant for Google Cloud Error Reporting fields.
- `github.com/global-torque/go-common/logger/example/v2/cli`: executable logger example, not a library package.
- `github.com/global-torque/go-common/logger/example/v2/web`: executable Echo logger example, not a library package.
- `github.com/global-torque/go-common/logger/v2/fxzerolog`: Uber Fx event logger adapter.
- `github.com/global-torque/go-common/logger/v2/tests`: logger module tests, not a stable helper package.
- `github.com/global-torque/go-common/misc/v2/round`: largest-remainder rounding helper.
- `github.com/global-torque/go-common/orm/v2`: lightweight pgx/Squirrel CRUD, save, transaction, and change tracking helpers.
- `github.com/global-torque/go-common/orm/v2/pgtype`: pgx v5 compatible legacy nullable `pgtype` wrappers.
- `github.com/global-torque/go-common/queue/v2`: Pub/Sub pull listener router with optional deduplication.
- `github.com/global-torque/go-common/queue/v2/pclient`: Google Pub/Sub v2 client, DTOs, publish/listen helpers.
- `github.com/global-torque/go-common/queue/v2/pubsubpush`: Pub/Sub push envelope DTOs and Echo max-attempt middleware.
- `github.com/global-torque/go-common/queue/v2/qtests`: Pub/Sub integration-test fixtures and push/publish actions.
- `github.com/global-torque/go-common/response/v2`: shared application error type and HTTP message helpers.
- `github.com/global-torque/go-common/server/v2`: Echo HTTP server wired for Fx, default middleware, and common error responses.
- `github.com/global-torque/go-common/server/v2/healthcheck`: `/healthcheck` handler.
- `github.com/global-torque/go-common/server/v2/middleware`: auth, JWT, request context, logging, IP, body dump middleware.
- `github.com/global-torque/go-common/server/v2/route`: route registration contract for handler groups.
- `github.com/global-torque/go-common/tests/v2`: table-test runner, HTTP test actions, JSON comparison helpers.
- `github.com/global-torque/go-common/validator/v2`: `go-playground/validator` wrapper returning `*response.Error`.
- `github.com/global-torque/go-common/verser/v2`: process-global service/version/repository/revision metadata.
- `docker` module (`github.com/global-torque/go-common/docker/v2`): build image seed, not a reusable library import.

## Component Notes

### `configurator`

- Import path: `github.com/global-torque/go-common/configurator/v2`.
- Use for: loading env-backed config structs during startup, optionally from `.env` or `ENV_FILE`.
- Do not use for: hot-path config reloads; `NewConfiguration` reads disk/env each call.
- Key APIs: `Parse[T](prefixes ...string)`, `NewConfiguration(conf, prefixes...)`, `LoadDotEnv`, `NewConfigurator`, `Get`, `MustGet`, `New`, `MustNew`, `Print`, `RequiredString`.
- Configuration: struct tags are `envconfig` tags such as `required:"true"`, `default:"..."`, and `split_words:"true"`. `RequiredString` rejects missing, empty, and whitespace-only values without needing `required:"true"`.
- Wiring pattern: use `Parse[T]("DB")` for simple startup config; use `NewConfigurator().New("key", &Config{}, "prefix")` when sharing parsed config through Fx.
- Testing helpers: use `t.Setenv("ENV_FILE", "")` or `ENV_FILE=.env.tests` to control dotenv loading.
- Gotchas: `MustGet`, `MustNew`, and `MustPrint` panic and should stay in app startup code, not library internals. `Get` copies cached configs with `copier` and ignores empty fields.

### `context/keys`

- Import path: `github.com/global-torque/go-common/context/v2/keys`.
- Use for: shared context values and HTTP header names used by logger, server, queue, and httputils.
- Do not use for: unrelated package-private context keys; avoid collisions by keeping local keys local.
- Key APIs: `ContextKey`, `ContextStr`, `GetAsString`, `GetCtxValue`, `SetCtxValue`, `SetCtxValues`.
- Configuration: none.
- Wiring pattern: store request/message data under keys such as `RequestID`, `IPAddress`, `MSGID`, `IdentityID`, and `LogInfo`; use header constants such as `X-Request-Id`, `X-Forwarded-For`, and `X-Real-IP`.
- Testing helpers: `keys.SetCtxValue(context.Background(), keys.RequestID, "...")`.
- Gotchas: `SetCtxValue` accepts only `ContextKey`; `SetIPAddress` middleware stores IP under `IPAddressStr`, while Pub/Sub context helpers store IP under `IPAddress`.

### `response`

- Import path: `github.com/global-torque/go-common/response/v2`.
- Use for: user-facing application errors that HTTP handlers can serialize consistently.
- Do not use for: internal-only errors that should remain plain wrapped errors until the boundary maps them.
- Key APIs: `Error`, `ErrorMessages`, `New`, `NewError`, `BadRequest`, `NotFound`, `InternalError`, `ErrBadRequest`, `ErrUnauthorized`, `ErrInternalError`, `MessagesFromAny`, `SingleErrorMessage`.
- Configuration: none.
- Wiring pattern: app/service code returns `*response.Error` for expected client-visible failures; Echo handlers pass errors to `server.ErrorResponse`.
- Testing helpers: compare `err.(*response.Error).StatusCode` and `err.(*response.Error).Message.Map()`.
- Gotchas: `New` returns an `Error` value, while most helpers return `*Error`. Message maps are cloned and JSON marshal to `{}` when empty. Default messages use `__error__`.

### `validator`

- Import path: `github.com/global-torque/go-common/validator/v2`.
- Use for: validating request DTOs and returning field-keyed `*response.Error` maps.
- Do not use for: config validation that needs raw `go-playground/validator` errors; `db` does that directly.
- Key APIs: `New`, `Validator.Validate`, `Validator.Verify`, `ParamName`, custom `path` validation.
- Configuration: validation uses `validate` tags and field names from `json`, then `param`, then `form` tags.
- Wiring pattern: set `e.Validator = validator.New()` in Echo or call `validator.New().Verify(dto, http.StatusPreconditionFailed)` for app-level precondition checks.
- Testing helpers: assert `err.(*response.Error).Message.Map()`.
- Gotchas: `New` panics if custom validation registration fails. `Validate` always uses HTTP 400; use `Verify` when status must be 412 or another code.

### `logger`

- Import path: `github.com/global-torque/go-common/logger/v2`.
- Use for: structured zerolog logging with component, severity, stack traces, and optional service context.
- Do not use for: ad hoc `fmt.Println` or Echo native logger output in services.
- Key APIs: `Logger`, `NewLogger`, `NewComponentLogger`, `NewComponentLoggerE`, `NewDefaultLogger`, `NewDefaultLoggerE`, `DefaultStdoutLogger`, `FromCtx`, `ServiceContext`.
- Configuration: `LOG_LEVEL`, `LOG_CONSOLE`; invalid or empty log level falls back to info.
- Wiring pattern: create component loggers with `logger.NewComponentLogger(ctx, "component")`; attach request metadata by storing `logger.ServiceContext` under `keys.LogInfo`; log wrapped errors with `.Stack().Err(err)`.
- Testing helpers: `logger/tests` contains internal stdout comparison tests; dependent services usually use `tests.CompareJSONBody`.
- Gotchas: caller fields are enabled only at debug/trace. `.Ctx(ctx)` on `Logger` returns `zerolog.Ctx(ctx)`; prefer passing context into log events when service context matters.

### `logger/echo_google_cloud`

- Import path: `github.com/global-torque/go-common/logger/v2/echo_google_cloud`.
- Use for: logs that should include Google Cloud Error Reporting `@type` on error, fatal, and panic levels.
- Do not use for: console-local logging where the Google error-reporting event type is noisy.
- Key APIs: `NewEchoGCLogger`, `NewComponentLogger`, `NewComponentLoggerE`, `DefaultStdoutLogger`, `EchoGoogleCloud`.
- Configuration: same logger config shape, loaded with prefix `logger`.
- Wiring pattern: swap this package in where GCP error reporting needs the special event marker.
- Testing helpers: logger module tests compare JSON output.
- Gotchas: the hook skips `@type` when output is `zerolog.ConsoleWriter`.

### `logger/fxzerolog`

- Import path: `github.com/global-torque/go-common/logger/v2/fxzerolog`.
- Use for: adapting `logger.Logger` to Uber Fx event logging.
- Do not use for: application logs; use `logger` directly.
- Key APIs: `Init`, `ZeroLogger.LogEvent`.
- Configuration: inherits the injected `logger.Logger`.
- Wiring pattern: pass `fx.WithLogger(fxzerolog.Init())` when constructing an Fx app.
- Testing helpers: none.
- Gotchas: event names are mapped manually; unknown Fx events may be ignored unless handled in `LogEvent`.

### `logger/example/*` and `logger/tests`

- Import path: `github.com/global-torque/go-common/logger/example/v2/cli`, `github.com/global-torque/go-common/logger/example/v2/web`, and `github.com/global-torque/go-common/logger/v2/tests`.
- Use for: source examples and logger module self-tests only.
- Do not use for: reusable service code; import `logger`, `logger/echo_google_cloud`, or `logger/fxzerolog` instead.
- Key APIs: example `main` packages demonstrate `NewComponentLogger`; test package has local stdout helpers.
- Configuration: examples use normal logger env such as `LOG_LEVEL`.
- Wiring pattern: read examples to understand stack logging and Echo request context, but do not depend on them.
- Testing helpers: no stable public helper contract.
- Gotchas: example modules exist so `go list` reports packages, but they are not framework APIs.

### `httputils`

- Import path: `github.com/global-torque/go-common/httputils/v2`.
- Use for: integration-test HTTP calls, small direct HTTP clients, multipart upload requests, and forwarded-IP normalization.
- Do not use for: production clients needing retries, tracing, custom transports, or richer error policies unless you inject your own `http.Client`.
- Key APIs: `Request`, `CreateDefaultRequest`, `CreateRequestWithFiles`, `SendRequest`, `SendRequestWithClient`, `GetIPAddress`.
- Configuration: `CreateDefaultRequest` falls back to `HOST` and `PORT` when `Request.Host` is empty; default scheme is `http`; default content type is `application/json`.
- Wiring pattern: build a `httputils.Request`, convert it with `CreateDefaultRequest(ctx, req)`, then send with `SendRequest` or `SendRequestWithClient(client)`.
- Testing helpers: used by `tests.SendHTTPRequest` and `tests.SendHTTPRequestFiles`.
- Gotchas: `GetIPAddress` checks `X-Original-Forwarded-For`, then `X-Forwarded-For`, then `X-Real-IP`, and defaults to `127.0.0.1`.

### `server`

- Import path: `github.com/global-torque/go-common/server/v2`.
- Use for: Echo HTTP services using Fx lifecycle, common middleware, validation, healthcheck, metrics, and response error serialization.
- Do not use for: non-Echo servers or services that need fully custom middleware ordering.
- Key APIs: `HTTPServer`, `InitAndRun`, `NewServer`, `MustNewServer`, `AddDefaultMiddlewares`, `StartServer`, `ErrorResponse`, `ErrorBadRequestResponse`, `NewHandlerGroups`.
- Configuration: `HOST`, `PORT`, required `CORS_ALLOWED_ORIGINS`, optional `READ_TIMEOUT_SECONDS`, `READ_HEADER_TIMEOUT_SECONDS`, `WRITE_TIMEOUT_SECONDS`, `IDLE_TIMEOUT_SECONDS`, `STARTUP_GRACE_MILLISECONDS`; middleware flags include `HTTP_HEALTHCHECK`, `HTTP_BODY_LIMIT`, `HTTP_PROMETHEUS`, `HTTP_BODY_DUMP`, `HTTP_REQUEST_LOGGER`, `HTTP_REQUEST_RECOVER`.
- Wiring pattern: expose route configurators with `server.NewHandlerGroups(NewRoutes)` and include `server.InitAndRun()` in an Fx module.
- Testing helpers: build an Echo context directly for handler tests or use `tests.SendHTTPRequest` for integration tests.
- Gotchas: `NewServer` fails when `CORS_ALLOWED_ORIGINS` is blank. `ErrorResponse` returns HTTP 501 for non-`response.Error` errors, so app code should map expected failures to `*response.Error`.

### `server/route`

- Import path: `github.com/global-torque/go-common/server/v2/route`.
- Use for: declaring HTTP route groups consumed by `server.InitHandlerGroups`.
- Do not use for: direct Echo route registration outside the shared server module.
- Key APIs: `Route`, `Configurator`, `ConfiguratorIn`.
- Configuration: none.
- Wiring pattern: implement `GetRoutes() []route.Route` and return `Method`, `Path`, `Handler`, and optional Echo `Middlewares`.
- Testing helpers: route structs are simple values.
- Gotchas: the field is `Handler`, not the older README's `Handle`.

### `server/middleware`

- Import path: `github.com/global-torque/go-common/server/v2/middleware`.
- Use for: shared Echo middleware for auth, JWT context, request IP/time, logger context, and request/response body dump.
- Do not use for: authorization rules that belong to a domain service.
- Key APIs: `SetIPAddress`, `SetRequestTime`, `SetLogger`, `CheckIdentityID`, `NewAuth0MW`, `NewAuthMiddleware`, `MustNewAuthMiddleware`, `ParseJWTPayload`, `SetJWTPayload`, `GetJWTPayload`, `ExtractTokenFromString`, `FileAndHealtchCheckSkipper`, `BodyDumpHandler`.
- Configuration: Auth0 middleware uses `AUTH_VALIDATE_URI` and `AUTH_HTTP_TIMEOUT_SECONDS`; HTTP middleware toggles are handled by `server.AddDefaultMiddlewares`.
- Wiring pattern: add middleware globally through `AddDefaultMiddlewares` or per-route through `route.Route.Middlewares`.
- Testing helpers: middleware tests use Echo `httptest` contexts.
- Gotchas: `CheckIdentityID` currently reads identity from the `Authorization` header and writes `identity_id` into logger context. `ExtractTokenFromString` is lenient; `Auth0Middleware` has stricter Bearer parsing.

### `server/healthcheck`

- Import path: `github.com/global-torque/go-common/server/v2/healthcheck`.
- Use for: the default `GET /healthcheck` endpoint.
- Do not use for: readiness/liveness dependency checks; this handler only returns `200 OK` and body `OK`.
- Key APIs: `Healthcheck`.
- Configuration: disabled by `HTTP_HEALTHCHECK=false` in `server.NewServer`.
- Wiring pattern: normally registered automatically by `server.NewServer`.
- Testing helpers: direct Echo handler invocation.
- Gotchas: no dependency status is included.

### `db`

- Import path: `github.com/global-torque/go-common/db/v2`.
- Use for: PostgreSQL pgx pool/connection setup, pgx query logging, UTC session time zone, and PostgreSQL notifications.
- Do not use for: non-PostgreSQL databases; `Config.Type` validates only `postgres`.
- Key APIs: `Config`, `New`, `MustNew`, `NewDB`, `NewPool`, `NewPoolFromConfig`, `NewConn`, `NewConnFromConfig`, `GetConfigPool`, `GetConfigConn`, `Subscribe`, `NewDBLogger`, `CleanSQL`, `Repository`.
- Configuration: `DB_TYPE`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_DATABASE`, `DB_APP_NAME`, `DB_SSL_MODE`, `DB_MIN_CONNECTIONS`, `DB_MAX_CONNECTIONS`, `DB_MAX_CONN_LIFETIME`, `DB_MAX_RETRIES`, `DB_LOG_LEVEL`.
- Wiring pattern: use `db.New(ctx)` for a standard pool; use `db.NewDB(pool, log)` when tests or custom modules already own the pool.
- Testing helpers: use `db/dbtests` for fixtures and SQL assertions.
- Gotchas: `DB` embeds `*pgxpool.Pool`, so pgx pool methods are available directly. Pool and connection constructors set `SET TIME ZONE 'UTC'`. Initial connection uses exponential backoff with `DB_MAX_RETRIES`; `MustNew` logs fatal.

### `db/dbtests`

- Import path: `github.com/global-torque/go-common/db/v2/dbtests`.
- Use for: integration tests that need PostgreSQL fixtures, cleanup, and SQL assertions.
- Do not use for: unit tests without a real PostgreSQL database.
- Key APIs: `NewFixture`, `NewFixturesManager`, `FixturesManager.CleanAndApply`, `WithFixtures`, `Close`, `ExecQuery`, `SelectQuery`, `RawSQL`, `SQL`.
- Configuration: same `DB_*` env as `db`; constructor sets `TZ=UTC`.
- Wiring pattern: pass `dbtests.NewFixturesManager(ctx, dbtests.NewFixture("table", "fixtures/file.json"))` into `tests.RunTableTest`.
- Testing helpers: `SQL(query, expected...)` retries until expected columns match, useful for async workers.
- Gotchas: fixture JSON is resolved under `/tests/<filePath>` relative to the project; `Clean` assumes a `<table>_id_seq` sequence.

### `orm`

- Import path: `github.com/global-torque/go-common/orm/v2`.
- Use for: small pgx repositories that want Squirrel SQL generation without a full ORM.
- Do not use for: domain side effects, Pub/Sub history logs, model callbacks, or implicit reflection-based saves.
- Key APIs: `Repository`, `Beginner`, `DefaultFields`, `RetrieveOne`, `RetrieveAll`, `Create`, `Update`, `Exists`, `Delete`, `Save`, `SaveModel`, `PrimaryKeyer`, `ChangeResetter`, `SaveResult`, `ChangeSet`, `WithTx`.
- Configuration: none.
- Wiring pattern: model types provide `Fields() []string`, `Table() string`, and `SetID(any)` for CRUD helpers; `Save` uses explicit `InsertValues()` and `Changes()` maps.
- Testing helpers: stub the small `Repository` interface in unit tests; orm tests show expected Squirrel SQL.
- Gotchas: `Create` and `Save` scan generated IDs into `int`. `Update` and `Delete` reject empty predicates and tautologies such as `1=1`. `RetrieveOne` wraps `ErrRecordNotFound` and `pgx.ErrNoRows`; `RetrieveAll` returns a non-nil empty slice.

### `orm/pgtype`

- Import path: `github.com/global-torque/go-common/orm/v2/pgtype`.
- Use for: migrating old pgtype-style nullable fields to pgx v5 codecs.
- Do not use for: new domain models that can use native pointers, `pgtype` v5 types, or custom value objects cleanly.
- Key APIs: `Status`, `Undefined`, `Null`, `Present`, `Text`, `Timestamptz`, `JSON`, `JSONB`, `InfinityModifier`, `Infinity`, `None`, `NegativeInfinity`.
- Configuration: none.
- Wiring pattern: use `ScanText`/`TextValue`, `ScanTimestamptz`/`TimestamptzValue`, and `ScanBytes`/`BytesValue` with pgx codecs; use `JSONB.Set` and `JSONB.AssignTo` for compatibility conversions.
- Testing helpers: pgtype tests cover null, present, infinity, raw JSON marshal, and undefined encode errors.
- Gotchas: undefined values return errors on encode. `JSON` and `JSONB` marshal raw payloads but do not provide JSON unmarshal helpers.

### `queue`

- Import path: `github.com/global-torque/go-common/queue/v2`.
- Use for: long-running Pub/Sub pull workers with route-based callbacks and optional deduplication.
- Do not use for: Pub/Sub push HTTP endpoints; use `queue/pubsubpush` and service handlers.
- Key APIs: `PubSubListener`, `PubSubRoute`, `Deduper`, `New`, `MustNew`, `NewWithDeduper`, `MustNewWithDeduper`, `Start(ctx)`, `Close`, `AddRoutes`.
- Configuration: `PUBSUB_RECONNECT_WINDOW` overrides the listener reconnect-or-panic window.
- Wiring pattern: build `[]queue.PubSubRoute` with `Name` set exactly to `webhooks`, `events`, or `messages`, then call `listener.Start(ctx)` and `defer listener.Close()`.
- Testing helpers: use `queue/qtests` with the Pub/Sub emulator.
- Gotchas: invalid route names fail at `Start`. Deduplication currently wraps `events` and `webhooks`; raw `messages` callbacks are not dedup-wrapped. If a listener stays broken beyond the reconnect window, it panics so a supervisor restarts the process.

### `queue/pclient`

- Import path: `github.com/global-torque/go-common/queue/v2/pclient`.
- Use for: direct Google Pub/Sub topic/subscription management, publish, and pull listeners.
- Do not use for: service-level routing when `queue.PubSubListener` is enough.
- Key APIs: `Client`, `New`, `Close`, `CreateTopic`, `DeleteTopic`, `CreateSubscription`, `DeleteSubscription`, `TopicExist`, `SubscriptionExist`, `Publish`, `PublishToTopic`, `PublishEvent`, `PublishWebhook`, `ListenRawMsgs`, `ListenEvents`, `ListenWebhooks`, `Message`, `Event`, `Webhook`, `SetDefaultEventCtx`, `SetDefaultWebhookCtx`.
- Configuration: `PUBSUB_PROJECT_ID`, optional `PUBSUB_SERVICE_ACCOUNT_CREDENTIALS`; `PUBSUB_EMULATOR_HOST` disables credentials and targets the emulator.
- Wiring pattern: create one client with `pclient.New(ctx)`, defer `Close`, create topics/subscriptions as needed, publish DTOs or listen with callbacks that return nil to ack and error to nack.
- Testing helpers: package integration tests require `PUBSUB_PROJECT_ID` and reachable `PUBSUB_EMULATOR_HOST`.
- Gotchas: `PublishEvent` and `PublishWebhook` validate DTOs with HTTP 412 on validation errors. Listener drops messages with delivery attempt greater than 10 by acking. `CreateSubscription` enables exactly-once delivery and a 5 to 10 minute retry policy.

### `queue/pubsubpush`

- Import path: `github.com/global-torque/go-common/queue/v2/pubsubpush`.
- Use for: HTTP push subscription handlers that decode Google Pub/Sub envelopes or need a code-level max-attempt guard.
- Do not use for: pull subscriptions.
- Key APIs: `PushRequest`, `PushMessage`, `MaxAttempts`.
- Configuration: none in package; delivery attempts require Pub/Sub dead-lettering configuration.
- Wiring pattern: bind/decode `PushRequest` in an Echo handler and add `pubsubpush.MaxAttempts(n)` to route middleware.
- Testing helpers: use `queue/qtests.SendPushWebhook`, `SendPushEvent`, or `SendPushTo`.
- Gotchas: `DeliveryAttempt` is top-level on `PushRequest`, not inside `PushMessage`. `MaxAttempts` returns HTTP 204 when dropping and rewinds the body for downstream handlers otherwise.

### `queue/qtests`

- Import path: `github.com/global-torque/go-common/queue/v2/qtests`.
- Use for: Pub/Sub emulator integration-test fixtures and actions.
- Do not use for: production Pub/Sub code.
- Key APIs: `NewFixture`, `NewFixturesManager`, `FixturesManager.CleanAndApply`, `Delete`, `Clean`, `Close`, `SendPubSubEvent`, `SendPushWebhook`, `SendPushEvent`, `SendPushTo`.
- Configuration: `.env` via `configurator.LoadDotEnv`, `PUBSUB_PROJECT_ID`, `PUBSUB_EMULATOR_HOST`; push helpers default host/port from `HOST` and `PORT`.
- Wiring pattern: pass qtests fixture managers into `tests.RunTableTest`; use send actions inside scenarios.
- Testing helpers: generated fixture names in tests should be unique to avoid emulator state collisions.
- Gotchas: fixture `filePath` is currently stored but not loaded; `CleanAndApply` creates topic/subscription only.

### `tests`

- Import path: `github.com/global-torque/go-common/tests/v2`.
- Use for: table-driven integration scenarios with reusable actions and JSON response comparison.
- Do not use for: plain unit tests where standard `testing` and `assert` are clearer.
- Key APIs: `RunTableTest`, `TableTest`, `TestScenario`, `SomeAction`, `TestContext`, `ExpectedResponse`, `ExpectedResult`, `SendHTTPRequest`, `SendHTTPRequestFiles`, `Sleep`, `CompareJSONBody`, `AllowAny`, `AllowDictAny`, `IsNil`.
- Configuration: test flag `-name` filters scenarios by case-insensitive substring.
- Wiring pattern: compose fixture managers and actions, then call `tests.RunTableTest(t, ctx, fixtures, tableTest)`.
- Testing helpers: expected JSON can use string `%any%` for any non-zero actual value; expected integer `math.MinInt` also accepts any actual value.
- Gotchas: `SendHTTPRequst` is deprecated and misspelled; use `SendHTTPRequest`. Fixtures are reapplied per scenario.

### `misc/round`

- Import path: `github.com/global-torque/go-common/misc/v2/round`.
- Use for: rounding percentages or weights to integers while preserving a required total.
- Do not use for: currency or precision-sensitive decimal arithmetic.
- Key APIs: `Value`, `Values`, `SmartRound`, `ErrRound`.
- Configuration: none.
- Wiring pattern: implement `GetFloatValue` and `SetFloatValue` on each item, then call `round.SmartRound(values, 100)`.
- Testing helpers: table tests cover equal-value groups and impossible inputs.
- Gotchas: inputs must be non-empty, non-nil, finite numbers, and the truncated sum cannot already exceed the required sum.

### `verser`

- Import path: `github.com/global-torque/go-common/verser/v2`.
- Use for: process-global service metadata read by logger/server middleware.
- Do not use for: per-request or tenant-specific metadata.
- Key APIs: `SetServiceVersionRepositoryRevision`, `GetService`, `GetVersion`, `GetRepository`, `GetRevisionID`.
- Configuration: usually populated from build-time `-ldflags` in `main`.
- Wiring pattern: call `verser.SetServiceVersionRepositoryRevision(service, version, repository, revisionID)` once during startup.
- Testing helpers: set known metadata before middleware/logger assertions.
- Gotchas: `SetServiVersRepoRevis` is deprecated; use `SetServiceVersionRepositoryRevision`. State is global but protected by a mutex.

### `docker`

- Import path: do not import as a library; the module path is currently `github.com/global-torque/go-common/docker/v2`.
- Use for: building the shared `cr.webdevelop.pro/global-torque/go-common` image and copying common `etc` files into service images.
- Do not use for: application package imports.
- Key APIs: `docker/Dockerfile`, `docker/etc/make.sh`, `docker/etc/golangci.yml`, `docker/etc/air.toml`, `docker/build-deploy.sh`.
- Configuration: Docker build args include `GIT_COMMIT`, `BUILD_DATE`, `SERVICE_NAME`, `REPOSITORY`, `VERSION`; root CI builds Go 1.25.8 image variants.
- Wiring pattern: dependent Dockerfiles can use `FROM cr.webdevelop.us/global-torque/go-common:latest-dev AS builder` and run `./make.sh build`.
- Testing helpers: root `.github/workflows/ci.yaml` runs module vet/tests with PostgreSQL and Pub/Sub emulator services before image build.
- Gotchas: this directory is a build seed with blank imports to pre-download heavy dependencies; its module path is not a go-common package path.

## Cross-Cutting Patterns

- Prefer `*response.Error` for expected app failures at HTTP boundaries, and plain wrapped errors for unexpected internal failures until the boundary maps them.
- Prefer shared context keys from `context/keys` so logger, server, db, queue, and httputils agree on request/message metadata.
- Prefer `validator.New()` for request DTO validation and `response.ErrorMessages` for JSON-safe error bodies.
- Prefer package constructors that return errors in libraries (`New`, `NewServer`, `db.New`, `pclient.New`); reserve `Must...` constructors for `main`.
- In multi-module edits, run validation from each affected module directory. Root `go list ./...` is not equivalent to all module package lists unless using `go.work` intentionally.

## Verification Notes

- Source inspected: all `go.mod` files, package READMEs, exported Go declarations, tests, `go.work`, `make.sh`, Dockerfile, and task notes.
- Lightweight verification run: `go list ./...` from every module directory succeeded.
- Go tests were not run for this documentation-only update; several packages have integration tests that require PostgreSQL or a Pub/Sub emulator.
