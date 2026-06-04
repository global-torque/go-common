package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/webdevelop-pro/go-common/response"
)

func TestValidator(t *testing.T) {
	valid := New()
	type testCases struct {
		description    string
		input          interface{}
		expectedResult map[string][]string
		expectedError  bool
	}

	for _, scenario := range []testCases{
		{
			description: "Required field",
			input: struct {
				Name string `json:"name" validate:"required,gte=2"`
			}{""},
			expectedResult: map[string][]string{"name": {"missing data for required field"}},
			expectedError:  true,
		},
		{
			description: "Incorrect length",
			input: struct {
				Name string `json:"name" validate:"required,gte=2"`
			}{"a"},
			expectedResult: map[string][]string{"name": {"greater than or equal to 2"}},
			expectedError:  true,
		},
		{
			description: "Pass if empty but error if less than two",
			input: struct {
				Name string `json:"name,omitempty" validate:"omitempty,gte=2"`
			}{""},
			expectedResult: map[string][]string{},
			expectedError:  false,
		},
		{
			description: "Email",
			input: struct {
				Name string `json:"email" validate:"email"`
			}{"a"},
			expectedResult: map[string][]string{"email": {"not a valid email address"}},
			expectedError:  true,
		},
		{
			description: "OneOf",
			input: struct {
				Name string `json:"status" validate:"required,oneof=a b"`
			}{"c"},
			expectedResult: map[string][]string{"status": {"must be one of: a b"}},
			expectedError:  true,
		},
		{
			description: "SSN",
			input: struct {
				Name string `json:"ssn" validate:"required,ssn"`
			}{"123"},
			expectedResult: map[string][]string{"ssn": {"is a valid social security number: 123"}},
			expectedError:  true,
		},
	} {
		t.Run(scenario.description, func(t *testing.T) {
			err := valid.Validate(scenario.input)
			if scenario.expectedError && err == nil {
				t.Error("error should not be nil")
			}
			if !scenario.expectedError && err != nil {
				t.Error("error should be nil")
			}
			if scenario.expectedError {
				assert.EqualValues(t, scenario.expectedResult, err.(*response.Error).Message.Map())
			}
		})
	}
}
