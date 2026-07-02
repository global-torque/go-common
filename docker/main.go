package main

import (
	"context"
	"fmt"

	_ "github.com/ethereum/go-ethereum"
	_ "github.com/ethereum/go-ethereum/crypto"
	"github.com/global-torque/go-common/configurator"
	"github.com/global-torque/go-common/context/keys"
	_ "github.com/global-torque/go-common/db"
	_ "github.com/global-torque/go-common/httputils"
	"github.com/global-torque/go-common/logger"
	_ "github.com/global-torque/go-common/queue"
	_ "github.com/global-torque/go-common/response"
	_ "github.com/global-torque/go-common/tests"
	_ "github.com/global-torque/go-common/validator"
	"github.com/global-torque/go-common/verser"
	_ "github.com/labstack/echo/v4"
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
