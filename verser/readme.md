# verser

Import path: `github.com/global-torque/go-common/verser/v2`

Process-global service metadata used by logging and server middleware.

## Use For

- Service name, version, repository, and revision ID populated at startup.
- Build metadata injected through Go linker flags.
- Log enrichment through `logger.ServiceContext`.

## Do Not Use For

- Request-scoped metadata.
- Tenant/user data.
- Values that need multiple independent instances in one process.

## Key APIs

- `SetServiceVersionRepositoryRevision(service, version, repository, revisionID)`
- `GetService()`
- `GetVersion()`
- `GetRepository()`
- `GetRevisionID()`

## Wiring Pattern

Call once in `main`:

```go
verser.SetServiceVersionRepositoryRevision(service, version, repository, revisionID)
```

Then middleware such as `server/middleware.SetLogger` reads the values for log
context.

## Testing

Set deterministic metadata before middleware or logger assertions.

## Gotchas

- `SetServiVersRepoRevis` is deprecated. Use
  `SetServiceVersionRepositoryRevision`.
- Values are global, but access is protected by a mutex.
