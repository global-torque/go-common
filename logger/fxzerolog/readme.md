# logger/fxzerolog

Import path: `github.com/global-torque/go-common/logger/v2/fxzerolog`

Adapter from `logger.Logger` to Uber Fx's `fxevent.Logger`.

## Use For

- Logging Fx lifecycle and dependency events with the shared zerolog format.

## Do Not Use For

- Application logs. Use `github.com/global-torque/go-common/logger/v2` directly.

## Key APIs

- `ZeroLogger`
- `Init() func(logger.Logger) fxevent.Logger`
- `(*ZeroLogger).LogEvent(event)`

## Configuration

Inherits the injected `logger.Logger`. Configure that logger through the parent
`logger` package.

## Wiring Pattern

```go
app := fx.New(
	fx.Provide(logger.NewDefaultLogger),
	fx.WithLogger(fxzerolog.Init()),
)
```

## Testing

No stable public testing helpers are provided.

## Gotchas

- Fx event handling is explicit in `LogEvent`; event types not handled there may
  produce no log.
