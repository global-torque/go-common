# server

Import path: `github.com/global-torque/go-common/server`

Echo HTTP server wired for Uber Fx lifecycle with shared middleware, validation,
healthcheck, metrics, and response error serialization.

## Use For

- HTTP services that use Echo and Fx.
- Common server middleware setup.
- Route groups registered through Fx.
- Serializing `*response.Error` values consistently.

## Do Not Use For

- Non-Echo HTTP servers.
- Services that need a fully custom server lifecycle or middleware order.

## Key APIs

- `HTTPServer`
- `InitAndRun() fx.Option`
- `NewServer() (*HTTPServer, error)`
- `MustNewServer() *HTTPServer`
- `AddDefaultMiddlewares(srv)`
- `StartServer(lc, srv)`
- `ErrorResponse(c, err)`
- `ErrorBadRequestResponse(c, err)`
- `NewHandlerGroups(groups...)`

## Configuration

Server config is read without a prefix:

- `HOST`: required
- `PORT`: required
- `CORS_ALLOWED_ORIGINS`: required, comma-separated allowlist
- `READ_TIMEOUT_SECONDS`: default `15`
- `READ_HEADER_TIMEOUT_SECONDS`: default `5`
- `WRITE_TIMEOUT_SECONDS`: default `30`
- `IDLE_TIMEOUT_SECONDS`: default `120`
- `STARTUP_GRACE_MILLISECONDS`: default `100`

Middleware flags:

- `HTTP_HEALTHCHECK=false` disables `/healthcheck`
- `HTTP_BODY_LIMIT`: default `20M`
- `HTTP_PROMETHEUS=false` disables metrics middleware and `/metrics`
- `HTTP_BODY_DUMP=false` disables body dumping
- `HTTP_REQUEST_LOGGER=true` enables request logger middleware
- `HTTP_REQUEST_RECOVER=false` disables recover middleware

## Wiring Pattern

```go
var Module = fx.Options(
	server.InitAndRun(),
	server.NewHandlerGroups(NewRoutes),
)
```

Handlers should bind, validate, call app code, and pass errors through the
server helpers:

```go
if err := c.Validate(&req); err != nil {
	return server.ErrorResponse(c, err)
}
```

## Testing

Use direct Echo handler tests for small handlers or
`github.com/global-torque/go-common/tests.SendHTTPRequest` for integration
tests.

## Gotchas

- `NewServer` returns an error when `CORS_ALLOWED_ORIGINS` is blank.
- `ErrorResponse` serializes `*response.Error`; non-response errors become HTTP
  501 with an `__error__` body.
- `MustNewServer` logs fatal on setup error; use `NewServer` outside `main`.
