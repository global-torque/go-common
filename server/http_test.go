package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestNewServerCORSAllowsRequiredRequestHeaders(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "8080")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://admin.example.com")

	server, err := NewServer()
	require.NoError(t, err)

	server.Echo.GET("/resource", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	request.Header.Set(echo.HeaderOrigin, "https://admin.example.com")
	request.Header.Set(echo.HeaderAccessControlRequestMethod, http.MethodGet)
	request.Header.Set(echo.HeaderAccessControlRequestHeaders, "X-API-Key")
	response := httptest.NewRecorder()

	server.Echo.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, "https://admin.example.com", response.Header().Get(echo.HeaderAccessControlAllowOrigin))
	allowHeaders := strings.Split(response.Header().Get(echo.HeaderAccessControlAllowHeaders), ",")
	require.Contains(t, normalizeHeaderNames(allowHeaders), strings.ToLower("X-API-Key"))
	require.Contains(t, normalizeHeaderNames(allowHeaders), strings.ToLower("Idempotency-Key"))
}

func normalizeHeaderNames(headers []string) []string {
	normalized := make([]string, 0, len(headers))
	for _, header := range headers {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(header)))
	}

	return normalized
}

/*
func TestHTTPCtx(t *testing.T) {
	ctx := context.Background()
	headers := map[string][]string{
		"X-Request-Id":    {"ZXCasdf123"},
		"X-Forwarded-For": {"31.6.1.12"},
	}

	ctx = keys.SetDefaultHTTPCtx(ctx, headers)

	assert.Equal(t, headers["X-Request-Id"][0], keys.GetCtxValue(ctx, keys.RequestID))
	assert.Equal(t, headers["X-Forwarded-For"][0], keys.GetCtxValue(ctx, keys.IPAddress))
}
*/

/*
ToDo:
	- make actuall request

// If Request Id header is empty we should automatically generate it
func TestEmptyRequestID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	echoCtx := e.NewContext(req, rec)
	echoCtx.Set(echo.HeaderXRequestID, "123123123")
	// Add middleware
	assert.Equal(t, len(keys.GetCtxValue(echoCtx, keys.RequestID).(string)), 9)
}
*/
