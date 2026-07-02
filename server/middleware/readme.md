# server/middleware

Import path: `github.com/global-torque/go-common/server/middleware`

Shared Echo middleware for auth, request metadata, logger context, JWT payloads,
and request/response body dumping.

## Use For

- Enriching request context with IP, request time, request ID, and log metadata.
- Auth0 token validation middleware.
- Identity-header checks for internal routes.
- JWT payload parsing and context storage.
- Body dump debugging.

## Do Not Use For

- Domain authorization rules.
- Non-Echo HTTP middleware.

## Key APIs

- `SetIPAddress`
- `SetRequestTime`
- `SetLogger`
- `CheckIdentityID`
- `NewAuth0MW`
- `NewAuthMiddleware`
- `MustNewAuthMiddleware`
- `NewAuthIdentityHeaderMW`
- `NewAuthNoneMW`
- `ParseJWTPayload`
- `SetJWTPayload`
- `GetJWTPayload`
- `ExtractTokenFromString`
- `FileAndHealtchCheckSkipper`
- `BodyDumpHandler`

## Configuration

Auth0 middleware config is read without a prefix:

- `AUTH_VALIDATE_URI`: required
- `AUTH_HTTP_TIMEOUT_SECONDS`: default `5`

HTTP middleware toggles are applied by `server.AddDefaultMiddlewares`.

## Wiring Pattern

Global middleware is normally installed by `server.AddDefaultMiddlewares`. For
route-level middleware:

```go
route.Route{
	Method: http.MethodGet,
	Path:   "/private",
	Handler: h.Private,
	Middlewares: []echo.MiddlewareFunc{
		middleware.CheckIdentityID,
	},
}
```

## Testing

Use Echo `httptest` contexts. Existing tests show `SetIPAddress` and
`SetLogger` assertions.

## Gotchas

- `CheckIdentityID` reads identity from the `Authorization` header.
- `SetLogger` builds `logger.ServiceContext` from `verser` metadata and request
  values already present in context.
- `Auth0Middleware` requires a well-formed `Bearer <token>` header before it
  calls the auth validation endpoint.
- `FileAndHealtchCheckSkipper` intentionally skips `/healthcheck`, `/metrics`,
  and multipart bodies.
