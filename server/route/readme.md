# server/route

Import path: `github.com/global-torque/go-common/server/v2/route`

Route declaration contract consumed by the shared server module.

## Use For

- Grouping Echo routes behind a small configurator object.
- Registering route groups through `server.NewHandlerGroups`.

## Do Not Use For

- Direct Echo registration outside go-common server wiring.

## Key APIs

- `Route`
- `Configurator`
- `ConfiguratorIn`

## Wiring Pattern

```go
type Routes struct {
	handler *Handler
}

func (r *Routes) GetRoutes() []route.Route {
	return []route.Route{
		{
			Method:  http.MethodGet,
			Path:    "/items/:id",
			Handler: r.handler.Get,
		},
	}
}
```

Register the constructor:

```go
server.NewHandlerGroups(NewRoutes)
```

## Route Fields

- `Method string`
- `Path string`
- `Handler echo.HandlerFunc`
- `Middlewares []echo.MiddlewareFunc`

## Gotchas

- The field name is `Handler`, not the older README's `Handle`.
- Route-level middleware is plain Echo middleware.
