# logger

Import path: `github.com/global-torque/go-common/logger`

Zerolog wrapper for service logs with component names, severity mapping,
optional caller fields, stack traces through `pkg/errors`, and request/service
context hooks.

## Use For

- Structured JSON logs in Go services.
- Component-scoped loggers.
- Adding request ID, message ID, service/version, repository, and HTTP request
  data to logs.
- Stack traces on wrapped errors.

## Do Not Use For

- Echo native logging output. The server package disables Echo logger output.
- Plain `fmt.Println` debugging in service code.

## Key APIs

- `Logger`
- `NewLogger(ctx, component, logLevel, output)`
- `NewComponentLogger(ctx, component)`
- `NewComponentLoggerE(ctx, component)`
- `NewDefaultLogger()`
- `NewDefaultLoggerE()`
- `DefaultStdoutLogger(ctx, logLevel)`
- `FromCtx(ctx, component)`
- `ServiceContext`
- `SourceReference`
- `HTTPRequestContext`

## Configuration

- `LOG_LEVEL`: parsed by zerolog; invalid or empty values fall back to `info`.
- `LOG_CONSOLE`: when true, writes console output for local development.

## Wiring Pattern

```go
log := logger.NewComponentLogger(ctx, "payments")
log.Info().Str("payment_id", id).Msg("payment started")

if err != nil {
	log.Error().Stack().Err(err).Msg("payment failed")
}
```

To enrich logs with service context, place `logger.ServiceContext` under
`keys.LogInfo`:

```go
ctx = keys.SetCtxValue(ctx, keys.LogInfo, logger.ServiceContext{
	Service: "api",
	Version: "v1",
})
```

## Testing

The logger module tests compare stdout JSON with
`github.com/global-torque/go-common/tests.CompareJSONBody`.

## Gotchas

- Caller fields are enabled only for debug and trace levels.
- `Logger.Ctx(ctx)` returns `zerolog.Ctx(ctx)`. For service context hooks, pass
  context into events with `.Ctx(ctx)` or create the logger with context.
- Use `.Stack()` only with errors that carry stack information, such as errors
  wrapped by `github.com/pkg/errors`.
