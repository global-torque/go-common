package pubsubpush

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestMaxAttempts(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		wantCalled bool
		wantStatus int
	}{
		{"malformed passes through", `{`, true, http.StatusAccepted},
		{"attempt equal to limit passes through", `{"message":{},"subscription":"s","deliveryAttempt":3}`, true, http.StatusAccepted},
		{"attempt above limit is acknowledged", `{"message":{},"subscription":"s","deliveryAttempt":4}`, false, http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			called := false
			e.Use(MaxAttempts(3))
			e.POST("/", func(c *echo.Context) error {
				called = true
				body, err := io.ReadAll(c.Request().Body)
				require.NoError(t, err)
				require.Equal(t, tc.body, string(body))
				return c.NoContent(http.StatusAccepted)
			})
			recorder := httptest.NewRecorder()
			e.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body)))

			require.Equal(t, tc.wantCalled, called)
			require.Equal(t, tc.wantStatus, recorder.Code)
		})
	}
}

func TestMaxAttemptsPropagatesDownstreamErrors(t *testing.T) {
	sentinel := errors.New("handler failed")

	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing delivery attempt", `{"message":{},"subscription":"s"}`},
		{"delivery attempt below limit", `{"message":{},"subscription":"s","deliveryAttempt":2}`},
		{"delivery attempt equal to limit", `{"message":{},"subscription":"s","deliveryAttempt":3}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			c := e.NewContext(
				httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body)),
				httptest.NewRecorder(),
			)
			invoked := false
			handler := MaxAttempts(3)(func(*echo.Context) error {
				invoked = true
				return sentinel
			})

			err := handler(c)

			require.ErrorIs(t, err, sentinel)
			require.True(t, invoked)
		})
	}
}
