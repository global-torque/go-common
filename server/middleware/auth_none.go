package middleware

import (
	"context"

	"github.com/global-torque/go-common/logger/v2"
	"github.com/labstack/echo/v4"
)

type AuthNoneMiddleware struct {
	log logger.Logger
}

func NewAuthNoneMW() *AuthNoneMiddleware {
	l := logger.NewComponentLogger(context.TODO(), "auth_tool")

	return &AuthNoneMiddleware{
		log: l,
	}
}

func (m *AuthNoneMiddleware) Validate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(c)
	}
}
