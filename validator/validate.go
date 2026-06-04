package validator

import (
	"errors"
	"fmt"
	"net/http"

	valid "github.com/go-playground/validator/v10"
	pkgerrors "github.com/pkg/errors"

	"github.com/webdevelop-pro/go-common/response"
)

const (
	MsgRequired = "missing data for required field"
	MsgEmail    = "not a valid email address"
	MsgMin      = "shorter than minimum length %s"
	MsgMax      = "longer than maximum length %s"
	MsgGte      = "greater than or equal to %s"
	MsgGt       = "greater than %s"
	MsgLen      = "must be one %s size"
	MsgOneOf    = "must be one of: %s"
	MsgEq       = "must be equal to: %s"
	MsgSSN      = "is a valid social security number: %s"
	MsgPath     = "invalid path: %s"
)

type FieldError struct {
	Param   string
	Message []string
}

type Validator struct {
	validator *valid.Validate
}

func New() *Validator {
	v := valid.New()
	v.RegisterTagNameFunc(ParamName)

	err := v.RegisterValidation("path", isPath)
	if err != nil {
		panic(err)
	}

	return &Validator{
		validator: v,
	}
}

func beautifulMsg(fe valid.FieldError) string {
	switch fe.Tag() {
	case "required":
		return MsgRequired
	case "email":
		return MsgEmail
	case "len":
		return fmt.Sprintf(MsgLen, fe.Param())
	case "min":
		return fmt.Sprintf(MsgMin, fe.Param())
	case "max":
		return fmt.Sprintf(MsgMax, fe.Param())
	case "gt":
		return fmt.Sprintf(MsgGt, fe.Param())
	case "gte":
		return fmt.Sprintf(MsgGte, fe.Param())
	case "oneof":
		return fmt.Sprintf(MsgOneOf, fe.Param())
	case "eq":
		return fmt.Sprintf(MsgEq, fe.Param())
	case "ssn":
		return fmt.Sprintf(MsgSSN, fe.Value())
	case "dirpath", "path":
		return fmt.Sprintf(MsgPath, fe.Value())
	}
	return fe.Error() // default error
}

// Verify checks payloads and returns error list
func (va Validator) Verify(i interface{}, httpStatus int) error {
	// call the `Struct` function passing in your payload
	err := va.validator.Struct(i)
	if err != nil {
		fieldErrors := &response.Error{
			StatusCode: httpStatus,
			Err:        pkgerrors.Wrapf(err, "validator error"),
			Message:    response.NewErrorMessages(nil),
		}

		var ve valid.ValidationErrors

		strErr := "validator error:"

		if errors.As(err, &ve) {
			for _, fe := range ve {
				fieldName := fe.Field()

				_, ok := fieldErrors.GetMessageFromMap(fieldName)
				if ok {
					fieldErrors.AddMessageToMap(fieldName, beautifulMsg(fe))
				} else {
					fieldErrors.AddMessageToMap(fieldName, beautifulMsg(fe))
					strErr = fmt.Sprintf("%s %s %s,", strErr, fieldName, beautifulMsg(fe))
				}
			}
		} else {
			fieldErrors.StatusCode = http.StatusInternalServerError
		}
		strErr = strErr[0 : len(strErr)-1]
		// We change default validator error message cause it does not provide details we need
		fieldErrors.Err = errors.New(strErr)

		return fieldErrors
	}

	return nil
}

// Validate executes Verify with default BadRequest response
func (va Validator) Validate(i interface{}) error {
	return va.Verify(i, http.StatusBadRequest)
}
