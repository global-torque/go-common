package middleware

import (
	"github.com/global-torque/go-common/context/keys"
	"github.com/global-torque/go-common/logger"
	"github.com/global-torque/go-common/verser"
	"github.com/labstack/echo/v4"
)

// SetLogger adds logger to context
func SetLogger(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// get request's context
		ctx := c.Request().Context()
		ipAddress := keys.GetAsString(ctx, keys.IPAddressStr)
		identityID := keys.GetAsString(ctx, keys.IdentityID)
		requestID := keys.GetAsString(ctx, keys.RequestID)
		msgID := keys.GetAsString(ctx, keys.MSGID)

		logInfo := logger.ServiceContext{
			Service: verser.GetService(),
			Version: verser.GetVersion(),
			SourceReference: &logger.SourceReference{
				Repository: verser.GetRepository(),
				RevisionID: verser.GetRevisionID(),
			},
			User:      identityID,
			RequestID: requestID,
			MSGID:     msgID,
			HTTPRequest: &logger.HTTPRequestContext{
				Method:    c.Request().Method,
				RemoteIP:  ipAddress,
				URL:       c.Request().RequestURI,
				UserAgent: c.Request().UserAgent(),
				Referrer:  c.Request().Referer(),
			},
		}

		ctx = keys.SetCtxValue(ctx, keys.LogInfo, logInfo)

		// create sub logger
		log := logger.NewComponentLogger(ctx, "echo")
		// add logger to context
		ctx = log.WithContext(ctx)
		// enrich context
		c.SetRequest(c.Request().WithContext(ctx))
		// next handler
		return next(c)
	}
}
