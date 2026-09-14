package middleware

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"time"

	"github.com/global-torque/go-common/logger/v2"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"
)

// TransactionStarter begins a transaction and exposes it through the returned context.
type TransactionStarter interface {
	BeginScope(ctx context.Context, options pgx.TxOptions) (context.Context, pgx.Tx, error)
}

const (
	defaultAtomicMaxResponseBytes = 1 << 20
	defaultAtomicRollbackTimeout  = 5 * time.Second
)

var (
	errAtomicConfig   = errors.New("atomic: invalid configuration")
	errAtomicScope    = errors.New("atomic: response committed or scope already exists")
	errAtomicResponse = errors.New("atomic: unsupported response")
)

// AtomicConfig controls which requests run in a database transaction.
// Zero response and cleanup limits select the documented defaults.
type AtomicConfig struct {
	Methods          []string
	Skipper          echomiddleware.Skipper
	TxOptions        pgx.TxOptions
	MaxResponseBytes int64
	RollbackTimeout  time.Duration
}

type atomicScopeKey struct{}

// Echo's Stream method otherwise hides its io.Copy behind ordinary response
// writes, making it indistinguishable from a fully buffered response writer.
type atomicContext struct {
	echo.Context

	buffer *atomicBuffer
}

func (c *atomicContext) Stream(int, string, io.Reader) error {
	return c.buffer.fail(fmt.Errorf("%w: streaming", errAtomicResponse))
}

// NewAtomic buffers participating responses until their database transaction completes.
// Install recovery outside this middleware. Handlers must retain the request context,
// return failures, and finish all database work before returning.
func NewAtomic(database TransactionStarter, config AtomicConfig) (echo.MiddlewareFunc, error) {
	// An interface containing a nil pointer is also a missing database.
	if database == nil || nilStarter(database) {
		return nil, fmt.Errorf("%w: database is required", errAtomicConfig)
	}

	if config.MaxResponseBytes < 0 || config.RollbackTimeout < 0 {
		return nil, fmt.Errorf("%w: response and rollback limits must not be negative", errAtomicConfig)
	}

	if config.MaxResponseBytes == 0 {
		config.MaxResponseBytes = defaultAtomicMaxResponseBytes
	}

	if config.RollbackTimeout == 0 {
		config.RollbackTimeout = defaultAtomicRollbackTimeout
	}

	if len(config.Methods) == 0 {
		config.Methods = []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	}

	methods := make(map[string]bool, len(config.Methods))
	for _, method := range config.Methods {
		methods[method] = true
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !methods[c.Request().Method] || (config.Skipper != nil && config.Skipper(c)) {
				return next(c)
			}

			return runAtomic(c, next, database, config)
		}
	}, nil
}

func nilStarter(database TransactionStarter) bool {
	value := reflect.ValueOf(database)
	switch value.Kind() { //nolint:exhaustive // Only nil-capable kinds need inspection.
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func runAtomic(c echo.Context, next echo.HandlerFunc, database TransactionStarter, config AtomicConfig) error {
	request := c.Request()

	response := c.Response()
	if response.Committed || request.Context().Value(atomicScopeKey{}) != nil {
		return atomicFailure(c, errAtomicScope)
	}

	ctx, tx, err := database.BeginScope(request.Context(), config.TxOptions)
	if err != nil {
		return atomicFailure(c, fmt.Errorf("atomic begin: %w", err))
	}
	// Copy the entire value, including Echo's private hook slices. Keep the same
	// response pointer so hooks which captured it continue observing staged state.
	original := *response
	buffer := newAtomicBuffer(response.Header(), config.MaxResponseBytes)
	response.Writer = buffer

	c.SetRequest(request.WithContext(context.WithValue(ctx, atomicScopeKey{}, true)))

	finished := false

	defer func() {
		*response = original
		c.SetResponse(response)
		c.SetRequest(request)

		if !finished {
			cleanupErr := atomicRollback(request.Context(), tx, config.RollbackTimeout)
			if cleanupErr != nil {
				atomicLog(c, cleanupErr, "atomic panic cleanup failed")
			}
		}
	}()

	handlerErr := next(&atomicContext{Context: c, buffer: buffer})
	if handlerErr == nil && !response.Committed && buffer.err == nil {
		if response.Status == 0 {
			response.Status = http.StatusOK
		}

		response.WriteHeader(response.Status)
	}

	validationErr := buffer.validate()
	cancelErr := ctx.Err()

	rollback := handlerErr != nil || validationErr != nil || cancelErr != nil || buffer.status >= http.StatusBadRequest
	if rollback {
		err = atomicRollback(request.Context(), tx, config.RollbackTimeout)
		finished = true

		if err != nil {
			return atomicFailure(c, fmt.Errorf("atomic rollback: %w", err))
		}

		if validationErr != nil || cancelErr != nil {
			return atomicFailure(c, errors.Join(validationErr, cancelErr))
		}

		if handlerErr != nil {
			return handlerErr
		}
	} else {
		err = tx.Commit(ctx)
		finished = true

		if err != nil {
			cleanupErr := atomicRollback(request.Context(), tx, config.RollbackTimeout)
			return atomicFailure(c, errors.Join(fmt.Errorf("atomic commit: %w", err), cleanupErr))
		}
	}

	// Replay through the original writer (including outer BodyDump wrappers),
	// bypassing Echo's Write methods so response hooks run only during staging.
	*response = original
	copyAtomicHeaders(response.Header(), buffer.snapshot)
	response.Status = buffer.status
	response.Committed = true
	response.Writer.WriteHeader(buffer.status)
	n, deliveryErr := response.Writer.Write(buffer.body.Bytes())

	response.Size = int64(n)
	if deliveryErr == nil && n != buffer.body.Len() {
		deliveryErr = io.ErrShortWrite
	}

	if deliveryErr != nil {
		atomicLog(c, deliveryErr, "atomic response delivery after transaction completion failed")
	}
	// The deferred restoration retains final metadata for outer middleware.
	original = *response

	return nil
}

func atomicRollback(ctx context.Context, tx pgx.Tx, timeout time.Duration) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	err := tx.Rollback(cleanup)
	if errors.Is(err, pgx.ErrTxClosed) {
		return nil
	}

	return err
}

func atomicFailure(c echo.Context, err error) error {
	atomicLog(c, err, "atomic request failed")
	return echo.NewHTTPError(http.StatusInternalServerError)
}

func atomicLog(c echo.Context, err error, message string) {
	log := zerolog.Ctx(c.Request().Context())
	if log.GetLevel() == zerolog.Disabled {
		fallback := logger.NewComponentLogger(c.Request().Context(), "atomic")
		log = &fallback.Logger
	}

	log.Error().Err(err).Str("component", "atomic").Msg(message)
}
