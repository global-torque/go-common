# logger/echo_google_cloud

Import path: `github.com/global-torque/go-common/logger/echo_google_cloud`

Logger variant that adds the Google Cloud Error Reporting event type to error,
fatal, and panic logs.

## Use For

- Services deployed where Google Cloud Error Reporting should group logged
  errors.
- Echo services that need the same base logger behavior plus the GCP `@type`
  field.

## Do Not Use For

- Local console logs where the Error Reporting marker is not useful.
- General Fx logging; use `logger/fxzerolog` for Fx events.

## Key APIs

- `EchoGoogleCloud`
- `NewEchoGCLogger(ctx, component, logLevel, output)`
- `NewComponentLogger(ctx, component)`
- `NewComponentLoggerE(ctx, component)`
- `DefaultStdoutLogger(ctx, logLevel)`

## Configuration

Uses `logger.Config` loaded with prefix `logger`:

- `LOG_LEVEL`
- `LOG_CONSOLE`

## Wiring Pattern

```go
log := echogooglecloud.NewComponentLogger(ctx, "http")
log.Error().Stack().Err(err).Msg("request failed")
```

## Testing

The parent logger tests verify the `@type` field is added for error logs.

## Gotchas

- The `@type` field is skipped when output is `zerolog.ConsoleWriter`.
- This package name is `echogooglecloud`; the import path directory is
  `logger/echo_google_cloud`.
