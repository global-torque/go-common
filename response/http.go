package response

import (
	"net/http"

	"github.com/pkg/errors"
)

// BadRequest shortcut to return http.StatusBadRequest with custom error and msg
func BadRequest(err error, msg string) *Error {
	if err == nil {
		err = errors.New("")
	}
	finalMsg := SingleErrorMessage(msg)
	if msg == "" {
		finalMsg = MsgBadRequest
	}
	return NewError(
		err,
		http.StatusBadRequest,
		finalMsg,
	)
}

// NotFound shortcut to return http.StatusNotFound with custom error and msg
func NotFound(err error, msg string) *Error {
	if err == nil {
		err = errors.New("")
	}
	finalMsg := SingleErrorMessage(msg)
	if msg == "" {
		finalMsg = MsgNotFound
	}
	return NewError(
		err,
		http.StatusNotFound,
		finalMsg,
	)
}

// InternalError shortcut to return http.StatusInternalServerError with custom error and msg
func InternalError(err error, msg string) *Error {
	if err == nil {
		err = errors.New("")
	}
	finalMsg := SingleErrorMessage(msg)
	if msg == "" {
		finalMsg = MsgInternalErr
	}
	return NewError(
		err,
		http.StatusInternalServerError,
		finalMsg,
	)
}
