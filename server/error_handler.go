package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/webdevelop-pro/go-common/logger"
	"github.com/webdevelop-pro/go-common/response"
)

func ErrorResponse(e echo.Context, err error) error {
	log := logger.FromCtx(e.Request().Context(), pkgName)

	respErr := &response.Error{}
	if errors.As(err, &respErr) {
		if respErr.StatusCode >= http.StatusInternalServerError {
			log.Error().Stack().Err(err).Msgf("system error happen")
		}

		log.Debug().Err(err).Msgf("error response")

		return e.JSON(respErr.StatusCode, respErr.Message)
	}

	// If we have not a response.Error but something else
	log.Warn().Stack().Err(err).Msgf("app return invalid error type")

	return e.JSON(
		http.StatusNotImplemented,
		map[string]interface{}{
			"__error__": err.Error(),
		},
	)
}

func ErrorBadRequestResponse(e echo.Context, err error) error {
	log := logger.FromCtx(e.Request().Context(), pkgName)
	log.Debug().Err(err).Msgf("cannot decode request")

	var resp any
	respErr := &response.Error{}

	if errors.As(err, &respErr) {
		resp = respErr.Message
	} else {
		var HTTPError *echo.HTTPError

		switch {
		case errors.As(err, &HTTPError):
			resp = map[string]interface{}{"__error__": []string{fmt.Sprint(HTTPError.Message)}}
		default:
			resp = map[string]interface{}{"__error__": []string{err.Error()}}
		}
	}

	return e.JSON(http.StatusBadRequest, resp)
}

func (s *HTTPServer) httpErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	var (
		he      *response.Error
		echoErr *echo.HTTPError
	)

	switch {
	case errors.As(err, &echoErr):
		he = &response.Error{
			StatusCode: echoErr.Code,
			Message:    response.MessagesFromAny(echoErr.Message),
		}
	case errors.As(err, &he):
		// do nothing
	default:
		he = &response.Error{
			StatusCode: http.StatusInternalServerError,
			Message:    response.SingleErrorMessage(http.StatusText(http.StatusInternalServerError)),
		}
	}

	code := he.StatusCode

	// Send response
	if c.Request().Method == http.MethodHead { // Issue #608
		err = c.NoContent(he.StatusCode)
	} else {
		err = c.JSON(code, he.Message)
	}

	if err != nil {
		s.log.Err(err).Msg("cannot send error response")
	}
}
