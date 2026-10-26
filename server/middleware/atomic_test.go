package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	em "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type atomicTestTx struct {
	pgx.Tx
	commits, rollbacks     int
	commitErr, rollbackErr error
	commit                 func()
	cleanupContext         context.Context
	cleanupErr             error
	cleanupRemaining       time.Duration
}

func (tx *atomicTestTx) Commit(context.Context) error {
	tx.commits++
	if tx.commit != nil {
		tx.commit()
	}
	return tx.commitErr
}

func (tx *atomicTestTx) Rollback(ctx context.Context) error {
	tx.rollbacks++
	tx.cleanupContext = ctx
	tx.cleanupErr = ctx.Err()
	if deadline, ok := ctx.Deadline(); ok {
		tx.cleanupRemaining = time.Until(deadline)
	}
	return tx.rollbackErr
}

type atomicTestDB struct {
	tx      *atomicTestTx
	err     error
	begins  int
	options pgx.TxOptions
}

type atomicTestScopeKey struct{}

func (db *atomicTestDB) BeginScope(ctx context.Context, options pgx.TxOptions) (context.Context, pgx.Tx, error) {
	db.begins++
	db.options = options
	return context.WithValue(ctx, atomicTestScopeKey{}, true), db.tx, db.err
}

func TestAtomicContextAndPanicRestoration(t *testing.T) {
	for _, panicHandler := range []bool{false, true} {
		t.Run(fmt.Sprint(panicHandler), func(t *testing.T) {
			tx := &atomicTestTx{}
			atomic, err := NewAtomic(&atomicTestDB{tx: tx}, AtomicConfig{})
			require.NoError(t, err)
			e := echo.New()
			writer := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/", nil)
			c := e.NewContext(request, writer)
			response := c.Response()
			originalStatus := response.Status
			handler := atomic(func(c echo.Context) error {
				require.Equal(t, true, c.Request().Context().Value(atomicTestScopeKey{}))
				require.Same(t, response, c.Response())
				require.NoError(t, c.String(201, "discard"))
				if panicHandler {
					panic("original panic")
				}
				return errors.New("returned error")
			})
			if panicHandler {
				require.PanicsWithValue(t, "original panic", func() { _ = handler(c) })
			} else {
				require.Error(t, handler(c))
			}
			require.Same(t, request, c.Request())
			require.Same(t, response, c.Response())
			require.Same(t, writer, c.Response().Writer)
			require.False(t, c.Response().Committed)
			require.Equal(t, originalStatus, c.Response().Status)
			require.Zero(t, c.Response().Size)
			require.Nil(t, c.Request().Context().Value(atomicTestScopeKey{}))
			require.Equal(t, 1, tx.rollbacks)
			require.NoError(t, tx.cleanupErr)
			require.Positive(t, tx.cleanupRemaining)
			require.LessOrEqual(t, tx.cleanupRemaining, 5*time.Second)
			require.Empty(t, writer.Body.String())
		})
	}
}

func TestAtomicLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name                       string
		handler                    echo.HandlerFunc
		status, commits, rollbacks int
		body                       string
	}{
		{"success", func(c echo.Context) error { return c.String(201, "created") }, 201, 1, 0, "created"},
		{"redirect", func(c echo.Context) error { return c.Redirect(303, "/done") }, 303, 1, 0, ""},
		{"not modified representation length", func(c echo.Context) error {
			c.Response().Header().Set("Content-Length", "127")
			return c.NoContent(304)
		}, 304, 1, 0, ""},
		{"explicit content length", func(c echo.Context) error {
			c.Response().Header().Set("Content-Length", "2")
			return c.String(200, "ok")
		}, 200, 1, 0, "ok"},
		{"empty", func(echo.Context) error { return nil }, 200, 1, 0, ""},
		{"returned error", func(c echo.Context) error { _ = c.String(201, "discard"); return echo.NewHTTPError(409) }, 409, 0, 1, ""},
		{"written error", func(c echo.Context) error { return c.String(422, "invalid") }, 422, 0, 1, "invalid"},
		{"serialization", func(c echo.Context) error { return c.JSON(200, make(chan int)) }, 500, 0, 1, ""},
		{"panic", func(c echo.Context) error { _ = c.String(200, "discard"); panic("boom") }, 500, 0, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &atomicTestTx{}
			db := &atomicTestDB{tx: tx}
			e := echo.New()
			e.Logger.SetOutput(&strings.Builder{})
			atomic, err := NewAtomic(db, AtomicConfig{})
			require.NoError(t, err)
			e.Use(em.Recover(), atomic)
			e.POST("/", tc.handler)
			recorder := httptest.NewRecorder()
			tx.commit = func() { require.Empty(t, recorder.Body.String()); require.Empty(t, recorder.Header()) }
			e.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", nil))
			require.Equal(t, tc.status, recorder.Code)
			require.Equal(t, tc.commits, tx.commits)
			require.Equal(t, tc.rollbacks, tx.rollbacks)
			require.NotContains(t, recorder.Body.String(), "discard")
			if tc.body != "" {
				require.Equal(t, tc.body, recorder.Body.String())
			}
		})
	}
}

func TestAtomicMethodsAndRegistration(t *testing.T) {
	for _, registration := range []string{"global", "group", "route"} {
		for _, method := range []string{"POST", "PUT", "PATCH", "DELETE", "GET", "HEAD", "OPTIONS"} {
			t.Run(registration+"/"+method, func(t *testing.T) {
				db := &atomicTestDB{tx: &atomicTestTx{}}
				atomic, err := NewAtomic(db, AtomicConfig{})
				require.NoError(t, err)
				e := echo.New()
				handler := func(c echo.Context) error { return c.NoContent(204) }
				switch registration {
				case "global":
					e.Use(atomic)
					e.Add(method, "/", handler)
				case "group":
					e.Group("", atomic).Add(method, "/", handler)
				case "route":
					e.Add(method, "/", handler, atomic)
				}
				e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, "/", nil))
				want := 0
				if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
					want = 1
				}
				require.Equal(t, want, db.begins)
				require.Equal(t, want, db.tx.commits)
			})
		}
	}
}

func TestAtomicRejectedResponses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler echo.HandlerFunc
	}{
		{"overflow ignored", func(c echo.Context) error { _, _ = c.Response().Write([]byte("12345")); return nil }},
		{"flush ignored", func(c echo.Context) error { c.Response().Flush(); return nil }},
		{"hijack ignored", func(c echo.Context) error { _, _, _ = c.Response().Hijack(); return nil }},
		{"push ignored", func(c echo.Context) error { _ = c.Response().Writer.(http.Pusher).Push("/", nil); return nil }},
		{"informational", func(c echo.Context) error { c.Response().WriteHeader(103); return nil }},
		{"invalid status", func(c echo.Context) error { c.Response().WriteHeader(600); return nil }},
		{"trailers", func(c echo.Context) error { c.Response().Header().Set("Trailer", "X-End"); return c.NoContent(200) }},
		{"late trailers", func(c echo.Context) error {
			c.Response().WriteHeader(200)
			c.Response().Header().Set("Trailer:X-End", "yes")
			return nil
		}},
		{"stream", func(c echo.Context) error { return c.Stream(200, "text/event-stream", strings.NewReader("hi")) }},
		{"binary stream ignored", func(c echo.Context) error {
			_ = c.Stream(200, "application/octet-stream", strings.NewReader("hi"))
			return nil
		}},
		{"transfer encoding", func(c echo.Context) error {
			c.Response().Header().Set("Transfer-Encoding", "chunked")
			return c.NoContent(200)
		}},
		{"short body", func(c echo.Context) error {
			c.Response().Header().Set("Content-Length", "4")
			return c.String(200, "hi")
		}},
		{"long body", func(c echo.Context) error {
			c.Response().Header().Set("Content-Length", "1")
			return c.String(200, "hi")
		}},
		{"bad length", func(c echo.Context) error {
			c.Response().Header().Set("Content-Length", "bad")
			return c.NoContent(200)
		}},
		{"signed length", func(c echo.Context) error {
			c.Response().Header().Set("Content-Length", "+1")
			return c.String(200, "x")
		}},
		{"negative zero length", func(c echo.Context) error { c.Response().Header().Set("Content-Length", "-0"); return c.NoContent(200) }},
		{"duplicate length", func(c echo.Context) error {
			c.Response().Header()["Content-Length"] = []string{"2", "2"}
			return c.String(200, "hi")
		}},
		{"204 body ignored", func(c echo.Context) error { _ = c.String(204, "hi"); return nil }},
		{"304 body ignored", func(c echo.Context) error { _ = c.String(304, "hi"); return nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &atomicTestDB{tx: &atomicTestTx{}}
			atomic, err := NewAtomic(db, AtomicConfig{MaxResponseBytes: 4})
			require.NoError(t, err)
			e := echo.New()
			e.Logger.SetOutput(&strings.Builder{})
			e.Use(atomic)
			e.POST("/", tc.handler)
			recorder := httptest.NewRecorder()
			e.ServeHTTP(recorder, httptest.NewRequest("POST", "/", nil))
			require.Equal(t, 500, recorder.Code)
			require.Equal(t, 0, db.tx.commits)
			require.Equal(t, 1, db.tx.rollbacks)
		})
	}
}

func TestAtomicConfigAndScopeGuards(t *testing.T) {
	var missing *atomicTestDB
	for _, database := range []TransactionStarter{nil, missing} {
		_, err := NewAtomic(database, AtomicConfig{})
		require.Error(t, err)
	}
	db := &atomicTestDB{tx: &atomicTestTx{}}
	for _, config := range []AtomicConfig{{MaxResponseBytes: -1}, {RollbackTimeout: -1}} {
		_, err := NewAtomic(db, config)
		require.Error(t, err)
	}
	e := echo.New()
	e.Logger.SetOutput(&strings.Builder{})
	options := pgx.TxOptions{IsoLevel: pgx.Serializable}
	atomic, err := NewAtomic(db, AtomicConfig{Methods: []string{"GET"}, TxOptions: options,
		Skipper: func(c echo.Context) bool { return c.QueryParam("skip") == "yes" }})
	require.NoError(t, err)
	e.Use(atomic)
	e.GET("/", func(c echo.Context) error { return c.NoContent(204) })
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?skip=yes", nil))
	require.Zero(t, db.begins)
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	require.Equal(t, options, db.options)
	defaultAtomic, err := NewAtomic(db, AtomicConfig{})
	require.NoError(t, err)
	c := e.NewContext(httptest.NewRequest("POST", "/", nil), httptest.NewRecorder())
	c.Response().WriteHeader(200)
	err = defaultAtomic(func(echo.Context) error { t.Fatal("committed request ran handler"); return nil })(c)
	require.Error(t, err)
	require.Equal(t, 1, db.begins)
	c = e.NewContext(httptest.NewRequest("POST", "/", nil), httptest.NewRecorder())
	err = defaultAtomic(defaultAtomic(func(echo.Context) error { t.Fatal("duplicate scope ran handler"); return nil }))(c)
	require.Error(t, err)
	require.Equal(t, 2, db.begins)
	require.Equal(t, 1, db.tx.rollbacks)
}

func TestAtomicLifecycleFailuresAndCancellation(t *testing.T) {
	for _, stage := range []string{"begin", "commit", "rollback", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			tx := &atomicTestTx{}
			db := &atomicTestDB{tx: tx}
			sensitive := errors.New("sensitive database error")
			switch stage {
			case "begin":
				db.err = sensitive
			case "commit":
				tx.commitErr = sensitive
			case "rollback":
				tx.rollbackErr = sensitive
			}
			e := echo.New()
			var logs strings.Builder
			e.Logger.SetOutput(&logs)
			atomic, err := NewAtomic(db, AtomicConfig{RollbackTimeout: time.Second})
			require.NoError(t, err)
			e.Use(atomic)
			ctx, cancel := context.WithCancel(context.Background())
			log := zerolog.New(&logs)
			ctx = log.WithContext(ctx)
			defer cancel()
			e.POST("/", func(c echo.Context) error {
				c.Response().Header().Set("X-Success", "yes")
				_ = c.String(201, "discard")
				if stage == "cancel" {
					cancel()
				}
				if stage == "rollback" {
					return errors.New("handler error")
				}
				return nil
			})
			recorder := httptest.NewRecorder()
			e.ServeHTTP(recorder, httptest.NewRequest("POST", "/", nil).WithContext(ctx))
			require.Equal(t, 500, recorder.Code)
			require.Empty(t, recorder.Header().Get("X-Success"))
			require.NotContains(t, recorder.Body.String(), "discard")
			require.NotContains(t, recorder.Body.String(), "sensitive")
			if stage == "cancel" {
				require.Zero(t, tx.commits)
				require.Equal(t, 1, tx.rollbacks)
				// Cleanup's own timeout is canceled after it runs; parent cancellation
				// must not propagate to the context passed to Rollback.
				require.NotNil(t, tx.cleanupContext)
				require.NoError(t, tx.cleanupErr)
				require.Positive(t, tx.cleanupRemaining)
				require.LessOrEqual(t, tx.cleanupRemaining, time.Second)
			} else {
				require.Contains(t, logs.String(), "sensitive database error")
			}
		})
	}
}

func TestAtomicEchoStackHeadersHooksAndMetadata(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			tx := &atomicTestTx{}
			if fail {
				tx.commitErr = errors.New("commit failed")
			}
			e := echo.New()
			e.Logger.SetOutput(&strings.Builder{})
			atomic, err := NewAtomic(&atomicTestDB{tx: tx}, AtomicConfig{})
			require.NoError(t, err)
			var dumped string
			var logged em.RequestLoggerValues
			before, after := 0, 0
			e.Use(em.Recover(), em.CORS(), em.RequestID(),
				em.BodyDump(func(_ echo.Context, _, body []byte) { dumped = string(body) }),
				em.RequestLoggerWithConfig(em.RequestLoggerConfig{LogStatus: true, LogResponseSize: true, HandleError: true,
					LogValuesFunc: func(_ echo.Context, v em.RequestLoggerValues) error { logged = v; return nil }}),
				func(next echo.HandlerFunc) echo.HandlerFunc {
					return func(c echo.Context) error {
						c.Response().Before(func() { before++; c.Response().Header().Set("X-Hook", "yes") })
						c.Response().After(func() { after++ })
						return next(c)
					}
				}, atomic)
			e.POST("/", func(c echo.Context) error {
				c.Response().Header().Set("X-First", "first")
				if err := c.String(201, "ok"); err != nil {
					return err
				}
				c.Response().Header().Set("X-First", "late")
				c.Response().WriteHeader(500)
				return nil
			})
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/", nil)
			request.Header.Set("Origin", "https://example.com")
			e.ServeHTTP(recorder, request)
			require.Equal(t, recorder.Code, logged.Status)
			require.Equal(t, int64(recorder.Body.Len()), logged.ResponseSize)
			require.Equal(t, recorder.Body.String(), dumped)
			require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
			require.NotEmpty(t, recorder.Header().Get("X-Request-Id"))
			if fail {
				require.Equal(t, 500, recorder.Code)
				require.Empty(t, recorder.Header().Get("X-First"))
				require.Equal(t, 2, before) // staged response and replacement error each have hooks
			} else {
				require.Equal(t, 201, recorder.Code)
				require.Equal(t, "first", recorder.Header().Get("X-First"))
				require.Equal(t, "yes", recorder.Header().Get("X-Hook"))
				require.Equal(t, 1, before)
				require.Equal(t, 1, after)
			}
		})
	}
}

type atomicFailWriter struct{ *httptest.ResponseRecorder }

func (w atomicFailWriter) Write([]byte) (int, error) { return 0, errors.New("delivery failed") }

func TestAtomicDeliveryFailureDoesNotRollback(t *testing.T) {
	tx := &atomicTestTx{}
	e := echo.New()
	var logs strings.Builder
	e.Logger.SetOutput(&logs)
	atomic, err := NewAtomic(&atomicTestDB{tx: tx}, AtomicConfig{})
	require.NoError(t, err)
	log := zerolog.New(&logs)
	c := e.NewContext(httptest.NewRequest("POST", "/", nil).WithContext(log.WithContext(context.Background())), atomicFailWriter{httptest.NewRecorder()})
	require.NoError(t, atomic(func(c echo.Context) error { return c.String(201, "ok") })(c))
	require.Equal(t, 1, tx.commits)
	require.Zero(t, tx.rollbacks)
	require.True(t, c.Response().Committed)
	require.Equal(t, 201, c.Response().Status)
	require.Zero(t, c.Response().Size)
	require.Contains(t, logs.String(), "delivery failed")
}
