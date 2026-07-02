# logger/example

Module path: `github.com/global-torque/go-common/logger/example/v2`

Executable examples for the logger package.

## Packages

- `github.com/global-torque/go-common/logger/example/v2/cli`
- `github.com/global-torque/go-common/logger/example/v2/web`

## Use For

- Seeing how `logger.NewComponentLogger` behaves in CLI and Echo examples.
- Checking stack trace output with `github.com/pkg/errors`.

## Do Not Use For

- Library imports in services. These are `main` packages.

## Configuration

Examples use the normal logger env:

- `LOG_LEVEL`
- `LOG_CONSOLE`

## Gotchas

- These packages are present in `go list ./...`, but they are examples, not
  framework components.
