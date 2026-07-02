package middleware

import (
	"context"
	"net/http"

	"github.com/global-torque/go-common/context/keys"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/global-torque/go-common/logger"
)

type AuthIdentityHeaderMiddleware struct {
	log logger.Logger
}

func NewAuthIdentityHeaderMW() *AuthIdentityHeaderMiddleware {
	l := logger.NewComponentLogger(context.TODO(), "auth_tool")

	return &AuthIdentityHeaderMiddleware{
		log: l,
	}
}

func (m *AuthIdentityHeaderMiddleware) Validate(next echo.HandlerFunc) echo.HandlerFunc {
	return CheckIdentityID(next)
}

func CheckIdentityID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		identityID := c.Request().Header.Get("Authorization")

		if identityID == "" {
			return c.JSON(http.StatusUnauthorized, map[string][]string{
				"__error__": {"empty identity_id"},
			})
		}

		ctx := context.WithValue(c.Request().Context(), keys.IdentityID, identityID)
		// Q
		// Should we name it identity_id?
		l := zerolog.Ctx(ctx).With().Str("identity_id", identityID).Logger()
		ctx = l.WithContext(ctx)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}
