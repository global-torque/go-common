# logger/tests

Import path: `github.com/global-torque/go-common/logger/v2/tests`

Internal test package for logger behavior.

## Use For

- Maintaining the logger module's own tests.
- Seeing expected JSON output shapes for info and error logs.

## Do Not Use For

- Reusable service test helpers. Prefer
  `github.com/global-torque/go-common/tests/v2`.

## Key APIs

There is no stable public helper API. Functions such as `ReadStdout` are local
test utilities.

## Gotchas

- This package exists for module tests and should not be imported by dependent
  services.
