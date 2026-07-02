package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/global-torque/go-common/response/v2"
	"github.com/global-torque/go-common/validator/v2"
	"github.com/labstack/echo/v4"
)

type User struct {
	Email string `json:"email" validate:"required,gt=2"`
}

type handler struct{}

func (h *handler) createUser(e echo.Context) error {
	return response.NotFound(fmt.Errorf("test"), "json error")
}

func (h *handler) createValidateError(e echo.Context) error {
	err := e.Validate(`"{'a':123}`)
	if err != nil {
		return ErrorResponse(e, err)
	}
	return nil
}

func TestErrorHandler(t *testing.T) {
	// Setup
	e := echo.New()
	// get an instance of a validator
	e.Validator = validator.New()
	req := httptest.NewRequest(http.MethodPost, "/user", strings.NewReader(""))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	h := &handler{}

	h.createUser(c)
	// res := h.createUser(c)
	// ToDo
	// finish tests
}
