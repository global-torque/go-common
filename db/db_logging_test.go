//nolint:paralleltest,thelper
package db

import (
	"bufio"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/webdevelop-pro/go-common/context/keys"
	"github.com/webdevelop-pro/go-common/logger"
	"github.com/webdevelop-pro/go-common/tests"
)

type stdOut struct {
	r *os.File
	w *os.File
}

func ConnectToStdout() *stdOut {
	r, w, _ := os.Pipe()
	os.Stdout = w

	return &stdOut{
		r: r,
		w: w,
	}
}

func ReadStdout(stdOut *stdOut) []string {
	result := make([]string, 0)
	scanner := bufio.NewScanner(stdOut.r)
	done := make(chan struct{})

	go func() {
		// Read line by line and process it
		for scanner.Scan() {
			line := scanner.Text()
			result = append(result, line)
		}

		// We're all done, unblock the channel
		done <- struct{}{}
	}()

	stdOut.w.Close()
	<-done

	return result
}

func CreateTestContext(ctx context.Context) context.Context {
	logInfo := logger.ServiceContext{
		Service: "test-db",
		Version: "v1.0.0",
		SourceReference: &logger.SourceReference{
			Repository: "https://github",
			RevisionID: "1111",
		},
		User:      "test-test-test",
		RequestID: "req-1",
		MSGID:     "msg-1",
		HTTPRequest: &logger.HTTPRequestContext{
			Method:    "POST",
			RemoteIP:  "0.0.0.0",
			URL:       "https://test",
			UserAgent: "test-agent",
			Referrer:  "test-Referrer",
		},
	}

	return keys.SetCtxValue(ctx, keys.LogInfo, logInfo)
}

func TestLogger_DBQuery(t *testing.T) {
	stdout := ConnectToStdout()

	ctx := context.TODO()
	ctx = CreateTestContext(ctx)
	resultInt, resultStr, resultTime := 0, "", time.Time{}

	os.Setenv("LOG_LEVEL", "info")
	os.Setenv("DB_LOG_LEVEL", "info")

	db, err := New(ctx)
	assert.Nil(t, err)
	err = db.QueryRow(ctx, "select 1, 'b', now();").Scan(&resultInt, &resultStr, &resultTime)
	assert.Nil(t, err)

	actualLogs := ReadStdout(stdout)
	expected := `
		{
			"level": "info",
			"component": "db",
			"data": {
				"args": [],
				"commandTag": "SELECT 1",
				"pid": "%any%",
				"sql": "select 1, 'b', now();",
				"time": "%any%"
			},
			"severity": "INFO",
			"serviceContext": {
				"service": "test-db",
				"version": "v1.0.0",
				"user": "test-test-test",
				"request_id": "req-1",
				"msg_id": "msg-1",
				"httpRequest": {
					"method": "POST",
					"url": "https://test",
					"userAgent": "test-agent",
					"referrer": "test-Referrer",
					"responseStatusCode": 0,
					"remoteIp": "0.0.0.0"
				},
				"sourceReference": {
					"repository": "https://github",
					"revisionId": "1111"
				}
			},
			"time": "%any%",
			"message": "Query"
		}
	`
	actual := actualLogs[len(actualLogs)-1]

	tests.CompareJSONBody(t, []byte(actual), []byte(expected))
}

func TestLogger_DBExec(t *testing.T) {
	ctx := context.TODO()
	ctx = CreateTestContext(ctx)

	os.Setenv("LOG_LEVEL", "info")
	os.Setenv("DB_LOG_LEVEL", "info")

	stdout := ConnectToStdout()

	db, err := New(ctx)
	assert.Nil(t, err)
	_, err = db.Exec(ctx, "SET TIME ZONE 'UTC';")
	assert.Nil(t, err)

	actualLogs := ReadStdout(stdout)
	expected := `
		{
			"level": "info",
			"component": "db",
			"data": {
				"args": [],
				"commandTag": "SET",
				"pid": "%any%",
				"sql": "SET TIME ZONE 'UTC';",
				"time": "%any%"
			},
			"severity": "INFO",
			"serviceContext": {
				"service": "test-db",
				"version": "v1.0.0",
				"user": "test-test-test",
				"request_id": "req-1",
				"msg_id": "msg-1",
				"httpRequest": {
					"method": "POST",
					"url": "https://test",
					"userAgent": "test-agent",
					"referrer": "test-Referrer",
					"responseStatusCode": 0,
					"remoteIp": "0.0.0.0"
				},
				"sourceReference": {
					"repository": "https://github",
					"revisionId": "1111"
				}
			},
			"time": "%any%",
			"message": "Query"
		}
	`
	actual := actualLogs[len(actualLogs)-1]

	tests.CompareJSONBody(t, []byte(actual), []byte(expected))
}

func TestLogger_DBQuery_ERROR(t *testing.T) {
	ctx := context.TODO()
	ctx = CreateTestContext(ctx)
	resultInt, resultStr, resultTime := 0, "", time.Time{}

	os.Setenv("LOG_LEVEL", "info")
	os.Setenv("DB_LOG_LEVEL", "info")

	stdout := ConnectToStdout()

	db, err := New(ctx)
	assert.Nil(t, err)
	err = db.QueryRow(ctx, "select asd;").Scan(&resultInt, &resultStr, &resultTime)
	assert.NotNil(t, err)

	actualLogs := ReadStdout(stdout)
	expected := `
		{
			"component": "db",
			"data": {
				"args": [],
				"err": {
					"ParseComplete": false
				},
				"pid": "%any%",
				"sql": "select asd;",
				"time": "%any%"
			},
			"error": "ERROR: column \"asd\" does not exist (SQLSTATE 42703)",
			"level": "error",
			"message": "ERROR: column \"asd\" does not exist (SQLSTATE 42703)",
			"serviceContext": {
				"httpRequest": {
					"method": "POST",
					"referrer": "test-Referrer",
					"remoteIp": "0.0.0.0",
					"responseStatusCode": 0,
					"url": "https://test",
					"userAgent": "test-agent"
				},
				"msg_id": "msg-1",
				"request_id": "req-1",
				"service": "test-db",
				"sourceReference": {
					"repository": "https://github",
					"revisionId": "1111"
				},
				"user": "test-test-test",
				"version": "v1.0.0"
			},
			"severity": "ERROR",
			"time": "%any%"
		}
	`
	actual := actualLogs[len(actualLogs)-1]

	tests.CompareJSONBody(t, []byte(actual), []byte(expected))
}
