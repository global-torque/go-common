# misc

Module path: `github.com/global-torque/go-common/misc`

Container module for small helpers that do not belong to a larger framework
component.

## Packages

- `github.com/global-torque/go-common/misc/round`: largest-remainder rounding
  that mutates values so their rounded sum matches a required total.

## Use For

- Small reusable helpers that are independent from HTTP, DB, queue, config, or
  logging concerns.

## Do Not Use For

- Service-specific utilities.
- New infrastructure components that deserve their own module.

## Testing

Run tests from this module:

```bash
cd misc
go test ./...
```
