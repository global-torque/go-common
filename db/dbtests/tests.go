package dbtests

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/global-torque/go-common/db"
	"github.com/global-torque/go-common/tests"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

const (
	sqlRetryInterval = 500
	maxQueryRetries  = 60
)

func getDBPool(t tests.TestContext) *db.DB {
	//nolint:forcetypeassert
	return t.Ctx.Value(dbKey).(*db.DB)
}

func RawSQL(query string) tests.SomeAction {
	return func(t tests.TestContext) error {
		query = strings.ReplaceAll(query, "\t", " ")
		query = strings.ReplaceAll(query, "\n", " ")
		query = strings.ReplaceAll(query, "  ", " ")

		_, err := getDBPool(t).Pool.Exec(context.Background(), query)
		return err
	}
}

func SQL(query string, expected ...tests.ExpectedResult) tests.SomeAction {
	return func(t tests.TestContext) error {
		var res map[string]interface{}

		query = strings.ReplaceAll(query, "\t", " ")
		query = strings.ReplaceAll(query, "\n", " ")
		query = strings.ReplaceAll(query, "  ", " ")
		rowQuery := "select row_to_json(q)::jsonb from (" + query + ") as q"

		// Retry until the query succeeds AND the expected result matches, or
		// the budget is exhausted. Worker assertions are asynchronous: the
		// row may not exist yet, or a stale/older row may briefly be the
		// "latest" one (order by id desc limit 1) before this test's own
		// event is processed. Retrying only on query error (the previous
		// behaviour) made such tests flaky; this restores the retry-until-
		// match semantics callers rely on.
		var err error
		for try := 0; ; try++ {
			err = getDBPool(t).Pool.QueryRow(context.Background(), rowQuery).Scan(&res)
			if err == nil && resultsMatch(res, expected...) {
				break
			}
			if try >= maxQueryRetries {
				break
			}
			time.Sleep(sqlRetryInterval * time.Millisecond)
		}
		if err != nil {
			return errors.Wrapf(err, "for sql: %s", query)
		}

		for _, exp := range expected {
			for key, value := range exp {
				// ToDo:
				// Find library to have colorful compare for maps
				expValue, ok := res[key]
				if assert.True(t.T, ok, fmt.Sprintf("Expected column %s not exist in result", key)) {
					value = tests.AllowAny(expValue, value)
					assert.Equal(t.T, value, expValue, key)
				}
			}
		}

		return nil
	}
}

// resultsMatch reports whether every expected key is present in res and equal
// under tests.AllowAny (the same comparison SQL asserts with). Used to decide
// whether to keep retrying; it never fails the test itself.
func resultsMatch(res map[string]interface{}, expected ...tests.ExpectedResult) bool {
	for _, exp := range expected {
		for key, value := range exp {
			actual, ok := res[key]
			if !ok {
				return false
			}
			if !reflect.DeepEqual(tests.AllowAny(actual, value), actual) {
				return false
			}
		}
	}
	return true
}
