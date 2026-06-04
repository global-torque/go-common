package main

import (
	"context"
	"fmt"

	_ "github.com/ethereum/go-ethereum"
	_ "github.com/ethereum/go-ethereum/crypto"
	_ "github.com/labstack/echo/v4"
	"github.com/webdevelop-pro/go-common/configurator"
	"github.com/webdevelop-pro/go-common/context/keys"
	_ "github.com/webdevelop-pro/go-common/db"
	_ "github.com/webdevelop-pro/go-common/httputils"
	"github.com/webdevelop-pro/go-common/logger"
	_ "github.com/webdevelop-pro/go-common/queue"
	_ "github.com/webdevelop-pro/go-common/response"
	_ "github.com/webdevelop-pro/go-common/tests"
	_ "github.com/webdevelop-pro/go-common/validator"
	"github.com/webdevelop-pro/go-common/verser"
	"go.uber.org/fx"
)

var (
	//nolint gochecknoglobals
	service    string
	version    string
	repository string
	revisionID string
)

func main() {
	ctx := context.TODO()
	ctx = keys.SetCtxValue(ctx, keys.LogInfo, logger.ServiceContext{
		Service: service,
		Version: version,
		SourceReference: &logger.SourceReference{
			Repository: repository,
			RevisionID: revisionID,
		},
	})
	verser.SetServiceVersionRepositoryRevision(service, version, repository, revisionID)
	fx.New(
		fx.Provide(
			// logger
			logger.NewDefaultLogger,
			context.Background,
			configurator.NewConfigurator,
			// Dwolla
		),
		fx.Invoke(
			// Create dwolla webhook
			fmt.Println,
			// ToDo
			// Add kill signal after 5 seconds
		),
	).Run()
}
