package middleware

import (
	"context"

	"github.com/global-torque/go-common/context/v2/keys"
	"github.com/global-torque/go-common/httputils/v2"
	"github.com/labstack/echo/v4"
)

func SetIPAddress(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ip := httputils.GetIPAddress(c.Request().Header)

		ctx := context.WithValue(c.Request().Context(), keys.IPAddressStr, ip)
		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
	}
}
