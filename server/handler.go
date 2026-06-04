package server

import (
	"github.com/webdevelop-pro/go-common/server/route"
	"go.uber.org/fx"
)

func InitHandlerGroups(srv *HTTPServer, rg route.ConfiguratorIn) {
	for _, group := range rg.Configurators {
		srv.InitRoutes(group)
	}
}

func NewHandlerGroups(groups ...any) fx.Option {
	//nolint:prealloc
	var annotates []any

	for _, group := range groups {
		annotates = append(
			annotates,
			fx.Annotate(
				group,
				fx.ResultTags(`group:"route_configurator"`),
				fx.As(new(route.Configurator)),
			),
		)
	}

	return fx.Provide(annotates...)
}
