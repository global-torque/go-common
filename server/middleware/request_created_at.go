// Package to add timestamp with milliseconds to track latency
package middleware

import (
	"context"
	"time"

	"github.com/global-torque/go-common/context/v2/keys"
	"github.com/labstack/echo/v4"
)

// SetRequestTime sets an initial request time
func SetRequestTime(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		currentTime := time.Now()

		ctx := context.WithValue(c.Request().Context(), keys.RequestTimeContextStr, currentTime)
		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
	}
}
