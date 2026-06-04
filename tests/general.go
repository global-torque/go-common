package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// -name filters scenarios by Description (case-insensitive substring match).
// Usage: go test -run TestX -name "Successfull case"
var scenarioNameFilter = flag.String("name", "", "filter scenarios by Description (case-insensitive substring)")

type TestScenario struct {
	Description string
	TestActions []SomeAction
}

type TableTest struct {
	Description string

	Scenarios []TestScenario
	Context   TestContext
}

func (tbl *TableTest) SetContext(t *testing.T, ctx context.Context) {
	tbl.Context = TestContext{t, ctx}
}

/*
func GetPointer(str string) *string {
	return &str
}
*/

func IsNil(v interface{}) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)

	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer,
		reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return val.IsNil()
	}

	return false
}

func RunTableTest(t *testing.T, ctx context.Context, fixtureMngrs []FixturesManager, tableTest TableTest) {
	t.Helper()

	tableTest.SetContext(t, ctx)

	scenarios := filterScenarios(tableTest.Scenarios, *scenarioNameFilter)
	if len(scenarios) == 0 {
		t.Skipf("no scenarios match -name=%q", *scenarioNameFilter)
		return
	}

	// ToDo
	// Run in parallel
	for _, s := range scenarios {
		scenario := s
		t.Run(tableTest.Description+": "+scenario.Description, func(t *testing.T) {
			scenarioCtx := ctx
			for _, fixtures := range fixtureMngrs {
				if err := fixtures.CleanAndApply(); err != nil {
					t.Fatalf("clean and apply fixtures: %v", err)
				}
				scenarioCtx = fixtures.SetCTX(scenarioCtx)
			}

			for _, action := range scenario.TestActions {
				err := action(TestContext{t, scenarioCtx})
				assert.NoError(t, err, "test failed")
			}
		})
	}
}

func filterScenarios(scenarios []TestScenario, name string) []TestScenario {
	if name == "" {
		return scenarios
	}
	needle := strings.ToLower(name)
	out := make([]TestScenario, 0, len(scenarios))
	for _, s := range scenarios {
		if strings.Contains(strings.ToLower(s.Description), needle) {
			out = append(out, s)
		}
	}
	return out
}

func AllowDictAny(src, dst map[string]interface{}) map[string]interface{} {
	if src == nil {
		return dst
	}
	res, ok := AllowAny(src, dst).(map[string]interface{})
	if !ok {
		return dst
	}
	return res
}

func AllowAny(src, dst interface{}) interface{} {
	switch expected := dst.(type) {
	case map[string]any:
		actualMap, ok := src.(map[string]any)
		if !ok {
			return dst
		}
		res := make(map[string]any, len(expected))
		for key, expectedValue := range expected {
			actualValue, ok := actualMap[key]
			if ok {
				res[key] = AllowAny(actualValue, expectedValue)
				continue
			}
			res[key] = expectedValue
		}
		return res
	case []any:
		actualSlice, ok := src.([]any)
		if !ok {
			return dst
		}
		res := make([]any, len(expected))
		for idx, expectedValue := range expected {
			if idx < len(actualSlice) {
				res[idx] = AllowAny(actualSlice[idx], expectedValue)
				continue
			}
			res[idx] = expectedValue
		}
		return res
	case string:
		if expected == "%any%" && src != nil && !reflect.ValueOf(src).IsZero() {
			return src
		}
	case int:
		if expected == math.MinInt {
			return src
		}
	}

	return dst
}

// ToDo
// use sprew or other library to better show different in maps
func CompareJSONBody(t *testing.T, actual, expected []byte) {
	t.Helper()

	var actualBody, expectedBody any

	if len(actual) == 0 {
		assert.Fail(t, "server return no data, nothing to compare")
		return
	}

	// Remove tabs and double spaces
	actual = bytes.ReplaceAll(actual, []byte("\t"), []byte(""))
	expected = bytes.ReplaceAll(expected, []byte("\t"), []byte(""))
	actual = bytes.ReplaceAll(actual, []byte("  "), []byte(" "))
	expected = bytes.ReplaceAll(expected, []byte("  "), []byte(" "))

	err := json.Unmarshal(actual, &actualBody)
	if err != nil {
		assert.Failf(t, "failed unmarshal actualBody", "%s, %s", err.Error(), actual)
		return
	}

	err = json.Unmarshal(expected, &expectedBody)
	if err != nil {
		assert.Failf(t, "failed unmarshal expectedBody", "%s %s", err.Error(), expected)
		return
	}

	expectedBody = AllowAny(actualBody, expectedBody)

	ok := assert.EqualValuesf(t, expectedBody, actualBody, "responses not equal")
	if !ok {
		t.Logf("expected: %s", expected)
		t.Logf("actual: %s", actual)
	}

	//assert.JSONEq(t, string(expected), string(actual), "responses not equal")
}
