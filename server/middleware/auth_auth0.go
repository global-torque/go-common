package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/global-torque/go-common/configurator/v2"
	"github.com/global-torque/go-common/logger/v2"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type AuthMiddleware interface {
	Validate(next echo.HandlerFunc) echo.HandlerFunc
}

// AuthMiddleware is struct which store instance of auth middleware
type Auth0Middleware struct {
	validateURI string
	httpClient  *http.Client
	log         logger.Logger
}

// Config is a struct to config auth middleware
type Config struct {
	AuthValidateURI        string `required:"true" split_words:"true"`
	AuthHTTPTimeoutSeconds int    `default:"5" split_words:"true"`
}

// NewAuthMW is a constructor of AuthMiddleware
func NewAuth0MW(cfg *Config, clients ...*http.Client) *Auth0Middleware {
	if cfg == nil {
		cfg = &Config{}
	}

	client := defaultAuthHTTPClient(cfg.AuthHTTPTimeoutSeconds)
	if len(clients) > 0 && clients[0] != nil {
		client = clients[0]
	}

	return &Auth0Middleware{
		validateURI: cfg.AuthValidateURI,
		httpClient:  client,
		log:         logger.NewComponentLogger(context.TODO(), "auth_tool"),
	}
}

// NewAuthMiddleware returns a new instance of AuthMiddleware
func NewAuthMiddleware() (*Auth0Middleware, error) {
	cfg := &Config{}
	l := logger.NewComponentLogger(context.TODO(), "auth_tool")

	if err := configurator.NewConfiguration(cfg); err != nil {
		return nil, fmt.Errorf("load auth0 middleware configuration: %w", err)
	}

	return &Auth0Middleware{
		validateURI: cfg.AuthValidateURI,
		httpClient:  defaultAuthHTTPClient(cfg.AuthHTTPTimeoutSeconds),
		log:         l,
	}, nil
}

// MustNewAuthMiddleware is NewAuthMiddleware with fatal-on-error semantics for app main packages.
func MustNewAuthMiddleware() *Auth0Middleware {
	mw, err := NewAuthMiddleware()
	if err != nil {
		log := logger.NewComponentLogger(context.TODO(), "auth_tool")
		log.Fatal().Err(err).Msg("failed to create auth0 middleware")
	}

	return mw
}

// ToDo
// Transfer headers ..
// Validate is middleware that extracts data from Authorization header and validates it
func (m *Auth0Middleware) Validate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var (
			ctx = c.Request().Context()
			l   = zerolog.Ctx(ctx)
		)

		token, err := bearerToken(c.Request().Header.Get("Authorization"))
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string][]string{"__error__": {err.Error()}})
		}

		// make request to auth service
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.validateURI, nil)
		if err != nil {
			l.Error().Err(err).Interface("req", req).Msg("Couldn't form request")
			return c.JSON(http.StatusForbidden, map[string][]string{"__error__": {"couldn't check authenticity"}})
		}

		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := m.httpClient.Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
		if err != nil {
			l.Error().Err(err).Interface("req", req).Msg("Couldn't do request")
			return c.JSON(http.StatusForbidden, map[string][]string{"__error__": {"couldn't check authenticity"}})
		}

		// if status code is not 2xx
		if !(resp.StatusCode >= 200 && resp.StatusCode <= 299) {
			return c.JSON(
				http.StatusUnauthorized,
				map[string][]string{"__error__": {"not valid token in Authorization header"}},
			)
		}

		jwtPayload, err := ParseJWTPayload(token)
		if err != nil {
			l.Error().Err(err).Interface("req", req).Msg("failed to decode token")
			return c.JSON(
				http.StatusBadRequest,
				map[string][]string{"__error__": {"failed to decode token"}},
			)
		}

		if jwtPayload.UserID == "" {
			return c.JSON(
				http.StatusNotFound,
				map[string][]string{"__error__": {"wrong user"}},
			)
		}

		SetJWTPayload(c, jwtPayload)

		return next(c)
	}
}

func defaultAuthHTTPClient(timeoutSeconds int) *http.Client {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 5
	}

	return &http.Client{
		Timeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

func bearerToken(header string) (string, error) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", fmt.Errorf("missing or malformed Authorization bearer token")
	}

	return parts[1], nil
}
