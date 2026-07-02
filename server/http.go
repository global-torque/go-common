package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/global-torque/go-common/context/keys"
	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	echoMW "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"go.uber.org/fx"

	"github.com/global-torque/go-common/validator"

	"github.com/global-torque/go-common/configurator"
	"github.com/global-torque/go-common/logger"

	"github.com/global-torque/go-common/server/healthcheck"
	"github.com/global-torque/go-common/server/middleware"
	"github.com/global-torque/go-common/server/route"
)

const pkgName = "http_server"

type HTTPServer struct {
	Echo       *echo.Echo
	log        logger.Logger
	config     *Config
	httpServer *http.Server
}

func InitAndRun() fx.Option {
	return fx.Module(pkgName,
		// Init http server
		fx.Provide(NewServer),
		fx.Invoke(
			//
			AddDefaultMiddlewares,
			// Registration routes and handlers for http server
			InitHandlerGroups,
			// Run HTTP server
			StartServer,
		),
	)
}

func (s *HTTPServer) InitRoutes(rg route.Configurator) {
	for _, r := range rg.GetRoutes() {
		//nolint:gosec,scopelint
		s.AddRoute(&r)
	}
}

// AddRoute adds route to the router.
func (s *HTTPServer) AddRoute(route *route.Route) {
	s.Echo.Add(route.Method, route.Path, route.Handler, route.Middlewares...)
}

// NewServer returns new API instance.
func NewServer() (*HTTPServer, error) {
	var (
		cfg = &Config{}
		l   = logger.NewComponentLogger(context.TODO(), pkgName)
	)

	if err := configurator.NewConfiguration(cfg); err != nil {
		return nil, fmt.Errorf("load http server configuration: %w", err)
	}
	if strings.TrimSpace(cfg.CORSAllowedOrigins) == "" {
		return nil, fmt.Errorf("CORS allowlist is required: set CORS_ALLOWED_ORIGINS")
	}

	e := echo.New()
	allowedOrigins := originAllowlist(cfg.CORSAllowedOrigins)
	// sets CORS headers if Origin is present.
	e.Use(
		echoMW.CORSWithConfig(echoMW.CORSConfig{
			Skipper: func(ctx echo.Context) bool {
				// Get the query parameter value
				schemaData := ctx.QueryParam("schema")
				// skip OPTIONS request if we already defined them in application
				for _, route := range ctx.Echo().Router().Routes() {
					// FixMe
					// Route might have dynamic attributes like :id
					if route.Method == http.MethodOptions && schemaData != "" {
						return true
					}
				}
				return false
			},
			AllowOriginFunc:  allowedOrigins,
			AllowCredentials: true,
			AllowMethods: []string{
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodOptions,
				http.MethodDelete,
				http.MethodPatch,
			},
			AllowHeaders: []string{
				echo.HeaderAuthorization,
				echo.HeaderContentType,
				echo.HeaderXRequestedWith,
				echo.HeaderXRequestID,
				echo.HeaderVary,
				"X-PINGOTHER",
			},
		}),
	)

	if os.Getenv("HTTP_HEALTHCHECK") != "false" {
		// Add the healthcheck endpoint
		e.GET(`/healthcheck`, healthcheck.Healthcheck)
	}

	// get an instance of a validator
	e.Validator = validator.New()

	// avoid any native logging of echo, because we use custom library for logging
	e.HideBanner = true        // don't log the banner on startup
	e.HidePort = true          // hide log about port server started on
	e.Logger.SetLevel(log.OFF) // disable echo#Logger

	newSrv := &HTTPServer{
		Echo:   e,
		config: cfg,
		log:    l,
	}

	// add HTTPErrorHandler
	newSrv.Echo.HTTPErrorHandler = newSrv.httpErrorHandler

	return newSrv, nil
}

// MustNewServer is NewServer with fatal-on-error semantics for app main packages.
func MustNewServer() *HTTPServer {
	srv, err := NewServer()
	if err != nil {
		log := logger.NewComponentLogger(context.TODO(), pkgName)
		log.Fatal().Err(err).Msg("failed to create http server")
	}

	return srv
}

func AddDefaultMiddlewares(srv *HTTPServer) {
	limit := os.Getenv("HTTP_BODY_LIMIT")
	if limit == "" {
		limit = "20M"
	}

	srv.Echo.Use(echoMW.BodyLimit(limit))
	srv.Echo.Use(middleware.SetIPAddress)
	srv.Echo.Use(middleware.SetRequestTime)

	// Trace ID middleware generates a unique id for a request.
	srv.Echo.Use(echoMW.RequestIDWithConfig(echoMW.RequestIDConfig{
		RequestIDHandler: func(c echo.Context, requestID string) {
			c.Set(echo.HeaderXRequestID, requestID)

			ctx := context.WithValue(c.Request().Context(), keys.RequestID, requestID)
			c.SetRequest(c.Request().WithContext(ctx))
		},
	}))

	// Set context logger after request/IP/request-id enrichment.
	srv.Echo.Use(middleware.SetLogger)

	if os.Getenv("HTTP_PROMETHEUS") != "false" {
		srv.Echo.Use(echoprometheus.NewMiddleware(pkgName))
		srv.Echo.GET("/metrics", echoprometheus.NewHandler())
	}

	if os.Getenv("HTTP_BODY_DUMP") != "false" {
		srv.Echo.Use(echoMW.BodyDumpWithConfig(echoMW.BodyDumpConfig{
			Skipper: middleware.FileAndHealtchCheckSkipper,
			Handler: middleware.BodyDumpHandler,
		}))
	}

	if os.Getenv("HTTP_REQUEST_LOGGER") == "true" {
		srv.Echo.Use(echoMW.RequestLoggerWithConfig(echoMW.RequestLoggerConfig{
			LogURI:       true,
			LogStatus:    true,
			LogMethod:    true,
			LogLatency:   true,
			LogURIPath:   true,
			LogError:     true,
			LogRequestID: true,
			HandleError:  true,

			Skipper: middleware.FileAndHealtchCheckSkipper,
			LogValuesFunc: func(c echo.Context, v echoMW.RequestLoggerValues) error {
				srv.log.Info().
					Str("method", v.Method).
					Str("URI", v.URI).
					Int("status", v.Status).
					Str("request_id", v.RequestID).
					Str("latency", v.Latency.String()).
					Msg("request")

				return nil
			},
		}))
	}

	if os.Getenv("HTTP_REQUEST_RECOVER") != "false" {
		srv.Echo.Use(echoMW.RecoverWithConfig(echoMW.RecoverConfig{
			StackSize: 10 << 10, // 10 KB
			LogLevel:  log.ERROR,
			LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
				srv.log.Error().Err(err).Bytes("stacktrace", stack).Msg("panic recover")

				return err
			},
		}))
	}
}

// StartServer is function that registers start of http server in lifecycle
func StartServer(lc fx.Lifecycle, srv *HTTPServer) {
	lc.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				on := fmt.Sprintf("%s:%s", srv.config.Host, srv.config.Port)

				srv.log.Info().Msgf("starting server on %s", on)

				listener, err := new(net.ListenConfig).Listen(ctx, "tcp", on)
				if err != nil {
					return fmt.Errorf("listen on %s: %w", on, err)
				}

				httpSrv := &http.Server{
					Addr:              on,
					Handler:           srv.Echo,
					ReadTimeout:       seconds(srv.config.ReadTimeoutSeconds),
					ReadHeaderTimeout: seconds(srv.config.ReadHeaderTimeoutSeconds),
					WriteTimeout:      seconds(srv.config.WriteTimeoutSeconds),
					IdleTimeout:       seconds(srv.config.IdleTimeoutSeconds),
				}
				srv.httpServer = httpSrv

				startErr := make(chan error, 1)
				go func() {
					if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
						srv.log.Error().Err(err).Msgf("stop server %s", on)
						startErr <- err
					}
				}()

				grace := time.Duration(srv.config.StartupGraceMilliseconds) * time.Millisecond
				select {
				case err := <-startErr:
					return fmt.Errorf("start http server on %s: %w", on, err)
				case <-time.After(grace):
					return nil
				case <-ctx.Done():
					if err := httpSrv.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
						srv.log.Error().Err(err).Msg("close server after startup cancellation")
					}
					return ctx.Err()
				}
			},
			OnStop: func(ctx context.Context) error {
				if srv.httpServer == nil {
					return nil
				}

				err := srv.httpServer.Shutdown(ctx)
				if err != nil {
					srv.log.Info().Err(err).Msg("couldn't stop server")
				}

				return nil
			},
		},
	)
}

func originAllowlist(value string) func(string) (bool, error) {
	parts := strings.Split(value, ",")
	allowed := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			allowed[part] = struct{}{}
		}
	}

	return func(origin string) (bool, error) {
		_, ok := allowed[origin]
		return ok, nil
	}
}

func seconds(value int) time.Duration {
	if value <= 0 {
		return 0
	}

	return time.Duration(value) * time.Second
}
