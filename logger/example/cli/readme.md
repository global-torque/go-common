# logger/example/cli

Import path: `github.com/global-torque/go-common/logger/example/v2/cli`

Executable CLI example for the logger package.

## Use For

- Seeing stack trace logging with `logger.NewComponentLogger`.
- Checking how service metadata can be placed in context before logging.

## Do Not Use For

- Library imports in services. This is a `main` package.

## Configuration

Uses normal logger env such as `LOG_LEVEL` and `LOG_CONSOLE`.
