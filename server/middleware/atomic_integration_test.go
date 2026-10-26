package middleware_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/global-torque/go-common/db/v2"
	"github.com/global-torque/go-common/logger/v2"
	"github.com/global-torque/go-common/server/v2"
	commonmiddleware "github.com/global-torque/go-common/server/v2/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestAtomicPostgres(t *testing.T) {
	for key, value := range map[string]string{
		"ENV_FILE": "", "HOST": "127.0.0.1", "PORT": "0",
		"CORS_ALLOWED_ORIGINS": "https://example.com", "HTTP_REQUEST_LOGGER": "true",
		"HTTP_PROMETHEUS": "false", "HTTP_BODY_DUMP": "true", "HTTP_REQUEST_RECOVER": "true",
	} {
		t.Setenv(key, value)
	}
	dsn := os.Getenv("ATOMIC_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set ATOMIC_TEST_DATABASE_URL to run real PostgreSQL atomic HTTP tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	database := db.NewDB(pool, logger.Logger{})
	// Published dependency validation must compile before the adapter is released.
	// Workspace verification requires this capability and runs every case below.
	starter, ok := any(database).(commonmiddleware.TransactionStarter)
	if !ok {
		t.Skip("published db lacks BeginScope; run with the local go-common workspace before release")
	}
	_, err = pool.Exec(ctx, `CREATE TEMP TABLE atomic_http (id integer, UNIQUE(id) DEFERRABLE INITIALLY DEFERRED)`)
	require.NoError(t, err)
	for _, outcome := range []string{"commit", "handler error", "error status", "commit constraint", "panic"} {
		t.Run(outcome, func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE atomic_http")
			require.NoError(t, err)
			srv, err := server.NewServer()
			require.NoError(t, err)
			server.AddDefaultMiddlewares(srv)
			e := srv.Echo
			atomic, err := commonmiddleware.NewAtomic(starter, commonmiddleware.AtomicConfig{})
			require.NoError(t, err)
			e.Use(atomic)
			e.POST("/", func(c echo.Context) error {
				requestContext := c.Request().Context()
				if _, err := database.Exec(requestContext, "INSERT INTO atomic_http VALUES (1)"); err != nil {
					return err
				}
				savepoint, err := database.Begin(requestContext)
				if err != nil {
					return err
				}
				if _, err = savepoint.Exec(requestContext, "INSERT INTO atomic_http VALUES (2)"); err != nil {
					return err
				}
				if err = savepoint.Commit(requestContext); err != nil {
					return err
				}
				if outcome == "commit constraint" {
					if _, err = database.Exec(requestContext, "INSERT INTO atomic_http VALUES (1)"); err != nil {
						return err
					}
				}
				if outcome == "error status" {
					return c.String(422, "invalid")
				}
				c.Response().Header().Set("X-Success", "yes")
				if err = c.String(201, "created"); err != nil {
					return err
				}
				if outcome == "handler error" {
					return errors.New("later repository operation failed")
				}
				if outcome == "panic" {
					panic("later repository operation panicked")
				}
				return nil
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
			request.Header.Set("Origin", "https://example.com")
			e.ServeHTTP(recorder, request)
			require.Equal(t, "https://example.com", recorder.Header().Get("Access-Control-Allow-Origin"))
			require.NotEmpty(t, recorder.Header().Get("X-Request-Id"))
			var count int
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM atomic_http").Scan(&count))
			if outcome == "commit" {
				require.Equal(t, 2, count)
				require.Equal(t, 201, recorder.Code)
				require.Equal(t, "created", recorder.Body.String())
			} else {
				require.Zero(t, count, fmt.Sprintf("%s must remove released savepoint writes too", outcome))
				require.GreaterOrEqual(t, recorder.Code, 400)
				require.Empty(t, recorder.Header().Get("X-Success"))
				require.NotContains(t, recorder.Body.String(), "created")
				if outcome != "error status" {
					require.Equal(t, 500, recorder.Code)
					require.Contains(t, recorder.Body.String(), "Internal Server Error")
					require.NotContains(t, recorder.Body.String(), "repository")
					require.NotContains(t, recorder.Body.String(), "constraint")
				}
			}
		})
	}
}
