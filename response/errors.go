package response

import (
	"fmt"
)

var (
	DefaultErrBadRequest = NewError(
		fmt.Errorf("bad request"), StatusBadRequest, MsgBadRequest,
	)
	DefaultErrUnauthorized = NewError(
		fmt.Errorf("unauthorized"), StatusUnauthorized, MsgUnauthorized,
	)
	DefaultErrNotFound = NewError(
		fmt.Errorf("not found"), StatusNotFound, MsgNotFound,
	)
	DefaultErrInternalError = NewError(
		fmt.Errorf("internal error"), StatusInternalError, MsgInternalErr,
	)
)

func ErrBadRequest(err error) *Error {
	return NewError(err, StatusBadRequest, SingleErrorMessage(errorText(err)))
}

func ErrUnauthorized(err error) *Error {
	return NewError(err, StatusUnauthorized, SingleErrorMessage(errorText(err)))
}

func ErrInternalError(err error) *Error {
	return NewError(err, StatusInternalError, SingleErrorMessage(errorText(err)))
}

func errorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
