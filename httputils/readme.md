# httputils

Import path: `github.com/global-torque/go-common/httputils/v2`

Small HTTP helpers for integration tests, direct requests, multipart uploads,
and forwarded IP extraction.

## Use For

- Building JSON requests from a compact struct.
- Building multipart upload requests for tests.
- Sending requests and reading full response bodies.
- Extracting a client IP from common proxy headers.

## Do Not Use For

- Production HTTP clients that need retries, tracing, backoff, or custom
  transport policies unless you inject the client explicitly.
- Complex multipart streaming.

## Key APIs

- `Request`
- `CreateDefaultRequest(ctx, req)`
- `CreateRequestWithFiles(ctx, req, body, files)`
- `SendRequest(req)`
- `SendRequestWithClient(client)`
- `GetIPAddress(headers)`

## Configuration

`CreateDefaultRequest` uses `Request.Host`, `Port`, and `Scheme`. If `Host` is
empty, it falls back to `HOST`; if `Port` is also empty, it falls back to `PORT`.
The default scheme is `http`, and the default content type is
`application/json`.

## Wiring Pattern

```go
req, err := httputils.CreateDefaultRequest(ctx, httputils.Request{
	Host:   "localhost",
	Port:   "8080",
	Method: http.MethodPost,
	Path:   "/items",
	Body:   []byte(`{"name":"demo"}`),
})
if err != nil {
	return err
}

body, resp, err := httputils.SendRequest(req)
```

## Testing

Use this package through `github.com/global-torque/go-common/tests/v2` helpers for
integration tests:

- `tests.SendHTTPRequest`
- `tests.SendHTTPRequestFiles`

## Gotchas

- `GetIPAddress` checks `X-Original-Forwarded-For`, then `X-Forwarded-For`,
  then `X-Real-IP`, and defaults to `127.0.0.1`.
- `CreateRequestWithFiles` expects file paths in the `files` map and normal form
  fields in the `body` map.
