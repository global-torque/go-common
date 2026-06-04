---
name: go-common
description: Shared Webdevelop Pro Go library guidance. Use when working in Go services that import github.com/webdevelop-pro/go-common packages for env configuration, structured logging, Echo/Fx HTTP servers and routes, response.Error handling, validation, pgx database access, Google Pub/Sub queue clients/listeners/push handlers, HTTP/test helpers, context keys, version metadata, rounding helpers, or the go-common Docker base image.
---

# Go Common

Use this skill to reuse `github.com/webdevelop-pro/go-common/*` packages instead
of reimplementing service plumbing. This repo is a multi-module library: import
packages by their module path, for example `github.com/webdevelop-pro/go-common/db`,
not a root `go-common` module.

When changing this library, run tests from the affected module directory because
each top-level package has its own `go.mod`.

```bash
cd ../go-common/db
go test ./...
```

## Package Map

- `configurator`: env loading through `envconfig`; loads `.env` or `ENV_FILE`.
- `logger`: `zerolog` logger with component, stack traces, severity, context hooks.
- `context/keys`: shared context keys for request id, IP address, message id, identity id.
- `response`: app-facing error type and default HTTP error messages.
- `validator`: `go-playground/validator` wrapper returning `response.Error`.
- `server`: Echo HTTP server wired for Fx, common middleware, healthcheck, metrics.
- `server/route`: route registration contract for HTTP handler groups.
- `server/middleware`: auth, request id/IP/time, logger, body dump, JWT helpers.
- `db`: pgx pool/connection setup, retry, UTC session timezone, query logging.
- `db/dbtests`: PostgreSQL fixture loading and cleanup for integration tests.
- `queue`: Pub/Sub pull listener router with optional message deduplication.
- `queue/pclient`: Google Pub/Sub client for topics, subscriptions, publish/listen.
- `queue/pubsubpush`: Pub/Sub push envelope DTOs and max-attempts Echo middleware.
- `queue/qtests`: Pub/Sub test fixtures and actions.
- `httputils`: HTTP request builders/senders and forwarded-IP extraction.
- `tests`: table-test runner, HTTP test actions, JSON body comparison with `%any%`.
- `misc/round`: largest-remainder rounding helper that preserves required sum.
- `verser`: process-global service/version/repository/revision metadata.
- `docker`: shared Go builder image and common CI/build support files.

## Configuration

Use `configurator.Parse[T]` for simple startup config and `Configurator` when
multiple components share a parsed config instance. `NewConfiguration` reads
`.env` by default or `ENV_FILE` when set.

```go
type Config struct {
	Endpoint string `required:"true" split_words:"true"`
	Timeout  int    `default:"30" split_words:"true"`
}

cfg, err := configurator.Parse[Config]("SCANNER")
if err != nil {
	return err
}
```

For strings that must be present and must reject whitespace-only values, use
`configurator.RequiredString`. It is required by type, so do not add a redundant
`required:"true"` tag.

```go
type PubSubConfig struct {
	TopicWebhook        configurator.RequiredString `split_words:"true"`
	SubscriptionWebhook configurator.RequiredString `split_words:"true"`
}

cfg := configurator.NewConfigurator().
	New("pubsub", &PubSubConfig{}, "pubsub").
	(*PubSubConfig)

topic := cfg.TopicWebhook.String()
```

Common env names:

- `HOST`, `PORT` for `server.Config`.
- `LOG_LEVEL`, `LOG_CONSOLE` for `logger`.
- `DB_TYPE`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_DATABASE`,
  `DB_APP_NAME`, `DB_SSL_MODE`, `DB_MIN_CONNECTIONS`, `DB_MAX_CONNECTIONS`,
  `DB_MAX_RETRIES`, `DB_LOG_LEVEL` for `db`.
- `PUBSUB_PROJECT_ID`, `PUBSUB_SERVICE_ACCOUNT_CREDENTIALS`,
  `PUBSUB_EMULATOR_HOST`, topic/subscription names such as
  `PUBSUB_TOPIC_WEBHOOK` and `PUBSUB_SUBSCRIPTION_WEBHOOK` for Pub/Sub code.
- `HTTP_HEALTHCHECK`, `HTTP_BODY_LIMIT`, `HTTP_PROMETHEUS`, `HTTP_BODY_DUMP`,
  `HTTP_REQUEST_LOGGER`, `HTTP_REQUEST_RECOVER` for HTTP middleware toggles.

## Logging And Context

Prefer component loggers and pass request or message context through the call
chain. Use `.Stack()` when logging wrapped errors that should appear in error
reporting.

```go
log := logger.NewComponentLogger(ctx, "scanner")
log.Info().Str("object", objectName).Msg("scan started")

if err != nil {
	log.Error().Stack().Err(err).Msg("scan failed")
	return err
}
```

Use shared context keys instead of ad hoc string keys.

```go
ctx = keys.SetCtxValue(ctx, keys.IdentityID, identityID)
requestID := keys.GetAsString(ctx, keys.RequestID)
```

Initialize version metadata once in `main` when build flags provide it.

```go
verser.SetServiVersRepoRevis(service, version, repository, revisionID)
```

## Errors And Validation

Application code should return `*response.Error` for user-facing failures. The
HTTP server knows how to serialize these errors and logs 5xx responses with a
stack.

```go
if err := repo.Save(ctx, item); err != nil {
	return response.InternalError(err, "")
}

if item == nil {
	return response.NotFound(nil, "file not found")
}
```

Use `validator.New()` directly or attach it to Echo through `server.NewServer`.
Validation errors are returned as `{field: [messages...]}`.

```go
type Request struct {
	Email string `json:"email" validate:"required,email"`
	Path  string `json:"path" validate:"required,path"`
}

if err := validator.New().Verify(req, http.StatusPreconditionFailed); err != nil {
	return err
}
```

In Echo handlers, bind, validate, delegate to app code, then use the server
error helpers.

```go
func (h *Handler) Create(c echo.Context) error {
	var req Request
	if err := c.Bind(&req); err != nil {
		return server.ErrorBadRequestResponse(c, err)
	}
	if err := c.Validate(&req); err != nil {
		return server.ErrorResponse(c, err)
	}
	if err := h.app.Create(c.Request().Context(), req); err != nil {
		return server.ErrorResponse(c, err)
	}
	return c.NoContent(http.StatusCreated)
}
```

## HTTP Server And Routes

Use `server.InitAndRun()` for Fx applications. Define route configurators that
return `[]route.Route`, then register constructors with `server.NewHandlerGroups`.

```go
type Routes struct {
	handler *Handler
}

func NewRoutes(handler *Handler) *Routes {
	return &Routes{handler: handler}
}

func (r *Routes) GetRoutes() []route.Route {
	return []route.Route{
		{
			Method:  http.MethodGet,
			Path:    "/files/:id",
			Handler: r.handler.GetFile,
		},
	}
}

var Module = fx.Options(
	server.InitAndRun(),
	server.NewHandlerGroups(NewRoutes),
)
```

Add route-level middleware from `server/middleware` or Echo middleware through
the `Middlewares` field.

```go
route.Route{
	Method:      http.MethodGet,
	Path:        "/auth/files",
	Handler:     h.List,
	Middlewares: []echo.MiddlewareFunc{middleware.CheckIdentityID},
}
```

## Database

Use `db.New(ctx)` for the standard pgx pool configured from env, or
`db.NewDB(pool, log)` when a test or custom module already owns the pool.
`db.DB` embeds `*pgxpool.Pool`, so call pgx pool methods directly.

```go
type Repository struct {
	db *db.DB
}

func NewRepository(ctx context.Context) *Repository {
	return &Repository{db: db.New(ctx)}
}

func (r *Repository) GetName(ctx context.Context, id int) (string, error) {
	var name string
	if err := r.db.QueryRow(ctx, "select name from files where id=$1", id).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}
```

For PostgreSQL notifications, use `Subscribe`.

```go
msgs, err := repo.db.Subscribe(ctx, "file_events")
if err != nil {
	return err
}
for msg := range msgs {
	log.Info().Bytes("payload", *msg).Msg("notification")
}
```

## Pub/Sub

For workers that consume pull subscriptions, return `[]queue.PubSubRoute`.
Route `Name` must be one of `webhooks`, `events`, or `messages`; it controls
which callback field is used.

```go
func NewPubSubRoutes(processor Processor, conf *configurator.Configurator) []queue.PubSubRoute {
	cfg := conf.New("pubsub", &PubSubConfig{}, "pubsub").(*PubSubConfig)

	return []queue.PubSubRoute{
		{
			Name:             "webhooks",
			Topic:            cfg.TopicWebhook.String(),
			Subscription:     cfg.SubscriptionWebhook.String(),
			WebhooksListener: processor.ProcessWebhook,
		},
	}
}
```

Start a listener from the routes. Use `NewWithDeduper` when processing must be
effectively at-most-once across redeliveries.

```go
listener := queue.New(routes)
listener.Start()

deduped := queue.NewWithDeduper(routes, "filer-scanner-worker", deduper)
deduped.Start()
```

Publish with `queue/pclient` when a service needs direct Pub/Sub access.

```go
client, err := pclient.New(ctx)
if err != nil {
	return err
}
defer client.Close()

msg, err := client.PublishWebhook(ctx, topic, pclient.Webhook{
	Action:  "file.scan.clean",
	Object:  "file",
	Service: "i-filer-api",
	Data:    payload,
})
if err != nil {
	return err
}
log.Info().Str("message_id", msg.ID).Msg("webhook published")
```

For push subscriptions, decode `pubsubpush.PushRequest` in handlers and add
`pubsubpush.MaxAttempts(n)` as a route middleware to ack-and-drop deliveries
that have exceeded a code-level attempt ceiling.

```go
route.Route{
	Method:      http.MethodPost,
	Path:        "/pubsub/storage",
	Handler:     h.HandlePush,
	Middlewares: []echo.MiddlewareFunc{pubsubpush.MaxAttempts(10)},
}
```

## HTTP And Integration Tests

Use `tests.RunTableTest` and `httputils.Request` for HTTP integration tests.
Use `%any%` in expected JSON values or headers to accept any non-zero actual
value.

```go
func TestHTTP_GetFile(t *testing.T) {
	ctx := context.Background()
	fixtures := []tests.FixturesManager{
		dbtests.NewFixturesManager(ctx, dbtests.NewFixture("files", "fixtures/files.json")),
	}

	tests.RunTableTest(t, ctx, fixtures, tests.TableTest{
		Description: "GET /files/:id",
		Scenarios: []tests.TestScenario{
			{
				Description: "success",
				TestActions: []tests.SomeAction{
					tests.SendHTTPRequest(
						httputils.Request{
							Host:   "localhost",
							Port:   "8081",
							Method: http.MethodGet,
							Path:   "/files/1",
						},
						tests.ExpectedResponse{
							Code: http.StatusOK,
							Body: []byte(`{"id":1,"url":"%any%"}`),
						},
					),
				},
			},
		},
	})
}
```

Use `db/dbtests` for SQL fixture cleanup/load and `queue/qtests` for Pub/Sub
fixture setup and publish/push test actions. Avoid the deprecated
`tests.SendHTTPRequst`; use `tests.SendHTTPRequest`.

## HTTP Utilities

Use `httputils.CreateDefaultRequest` and `httputils.SendRequest` for direct
HTTP calls. Use `CreateRequestWithFiles` for multipart upload tests.

```go
req, err := httputils.CreateDefaultRequest(ctx, httputils.Request{
	Host:   "localhost",
	Port:   "8081",
	Method: http.MethodPost,
	Path:   "/files",
	Body:   []byte(`{"name":"report.pdf"}`),
})
if err != nil {
	return err
}

body, resp, err := httputils.SendRequest(req)
```

Use `httputils.GetIPAddress(headers)` when normalizing proxy headers; it checks
`X-Original-Forwarded-For`, `X-Forwarded-For`, then `X-Real-IP`.

## Misc Helpers

Use `misc/round.SmartRound` for percentages or weights that must round to a
specific sum.

```go
type Part struct {
	Percent float64
}

func (p *Part) GetFloatValue() float64      { return p.Percent }
func (p *Part) SetFloatValue(value float64) { p.Percent = value }

values := round.Values{&Part{33.3}, &Part{33.3}, &Part{33.4}}
if err := round.SmartRound(values, 100); err != nil {
	return err
}
```

Use the shared Docker image in service Dockerfiles when the project follows
the Webdevelop Pro build layout.

```Dockerfile
FROM cr.webdevelop.us/webdevelop-pro/go-common:latest-dev AS builder
RUN ./make.sh build
```
