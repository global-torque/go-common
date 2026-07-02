# tests

Import path: `github.com/global-torque/go-common/tests`

Integration-test helpers for table scenarios, HTTP actions, fixture managers,
and JSON body comparison.

## Use For

- End-to-end or integration tests with reusable setup and actions.
- HTTP assertions using `httputils`.
- JSON body comparison with dynamic fields.

## Do Not Use For

- Small unit tests where standard `testing` and `assert` are clearer.
- Tests that require per-action custom control flow better expressed directly.

## Key APIs

- `RunTableTest`
- `TableTest`
- `TestScenario`
- `TestContext`
- `SomeAction`
- `ExpectedResponse`
- `ExpectedResult`
- `SendHTTPRequest`
- `SendHTTPRequestFiles`
- `Sleep`
- `CompareJSONBody`
- `AllowAny`
- `AllowDictAny`
- `IsNil`

## Configuration

The test flag `-name` filters scenarios by case-insensitive substring:

```bash
go test ./... -name success
```

## Wiring Pattern

```go
tests.RunTableTest(t, ctx, fixtures, tests.TableTest{
	Description: "GET /items",
	Scenarios: []tests.TestScenario{
		{
			Description: "success",
			TestActions: []tests.SomeAction{
				tests.SendHTTPRequest(req, expected),
			},
		},
	},
})
```

## Testing Helpers

`CompareJSONBody` supports `%any%` in expected JSON strings for any non-zero
actual value, including nested values and arrays.

## Gotchas

- `SendHTTPRequst` is deprecated and misspelled. Use `SendHTTPRequest`.
- Fixture managers are reapplied for each scenario.
- `ExpectedResponse.Body == nil` skips body comparison.
