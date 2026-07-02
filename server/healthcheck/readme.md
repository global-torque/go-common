# server/healthcheck

Import path: `github.com/global-torque/go-common/server/v2/healthcheck`

Minimal Echo healthcheck handler.

## Use For

- The default `GET /healthcheck` endpoint registered by `server.NewServer`.

## Do Not Use For

- Readiness checks.
- Dependency status checks.
- Liveness endpoints with memory, database, or queue details.

## Key APIs

- `Healthcheck(c echo.Context) error`

## Wiring Pattern

Normally no manual wiring is needed. `server.NewServer` registers:

```go
e.GET("/healthcheck", healthcheck.Healthcheck)
```

unless `HTTP_HEALTHCHECK=false`.

## Gotchas

- The handler returns HTTP 200 with body `OK` only.
