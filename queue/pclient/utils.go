package pclient

import (
	"context"
	"net/http"

	"github.com/global-torque/go-common/context/keys"
	"github.com/global-torque/go-common/httputils"
	"github.com/global-torque/go-common/logger"
	"github.com/global-torque/go-common/verser"
	"github.com/rs/zerolog"
)

func SetDefaultEventCtx(ctx context.Context, event Event) context.Context {
	ctx = keys.SetCtxValue(ctx, keys.RequestID, event.RequestID)
	ctx = keys.SetCtxValue(ctx, keys.IPAddress, event.IPAddress)
	ctx = keys.SetCtxValue(ctx, keys.MSGID, event.ID)

	logInfo := logger.ServiceContext{
		Service: verser.GetService(),
		Version: verser.GetVersion(),
		SourceReference: &logger.SourceReference{
			Repository: verser.GetRepository(),
			RevisionID: verser.GetRevisionID(),
		},
		MSGID: event.ID,
	}

	// create logger with serviceContext
	log := logger.NewComponentLogger(ctx, "event")
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Interface("serviceContext", logInfo)
	})
	// add logger to context
	ctx = log.WithContext(ctx)
	// ctx = keys.SetCtxValue(ctx, keys.LogInfo, logInfo)

	return ctx
}

func SetDefaultWebhookCtx(ctx context.Context, webhook Webhook) context.Context {
	headers := http.Header(webhook.Headers)

	requestID := headers.Get(string(keys.RequestIDStr))
	IP := httputils.GetIPAddress(headers)

	ctx = keys.SetCtxValue(ctx, keys.RequestID, requestID)
	ctx = keys.SetCtxValue(ctx, keys.IPAddress, IP)
	ctx = keys.SetCtxValue(ctx, keys.MSGID, webhook.ID)

	logInfo := logger.ServiceContext{
		Service: verser.GetService(),
		Version: verser.GetVersion(),
		SourceReference: &logger.SourceReference{
			Repository: verser.GetRepository(),
			RevisionID: verser.GetRevisionID(),
		},
		MSGID: webhook.ID,
	}

	// create logger with serviceContext
	log := logger.NewComponentLogger(ctx, "webhook")
	log.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Interface("serviceContext", logInfo)
	})
	// ADD logger to context
	ctx = log.WithContext(ctx)
	// ctx = keys.SetCtxValue(ctx, keys.LogInfo, logInfo)

	return ctx
}
