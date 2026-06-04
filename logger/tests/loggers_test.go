package tests

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/pkg/errors"

	"github.com/webdevelop-pro/go-common/context/keys"
	"github.com/webdevelop-pro/go-common/logger"
	echo_google_cloud "github.com/webdevelop-pro/go-common/logger/echo_google_cloud"
	"github.com/webdevelop-pro/go-common/tests"
)

func ReadStdout(r *os.File, w *os.File) string {
	outC := make(chan string)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()
	w.Close()
	out := <-outC

	return out
}

func testEchoLogger(t *testing.T, ctx context.Context, expected string, logF func(log logger.Logger)) {
	r, w, _ := os.Pipe()
	os.Stdout = w

	os.Setenv("LOG_LEVEL", "info")
	log := logger.NewComponentLogger(ctx, "test")

	logF(log)

	actual := ReadStdout(r, w)

	tests.CompareJSONBody(t, []byte(actual), []byte(expected))
}

func googleCloudExpected(expected string) string {
	if !strings.Contains(expected, `"level": "error"`) {
		return expected
	}

	return strings.Replace(
		expected,
		"{",
		`{"@type":"type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent",`,
		1,
	)
}

func testBaseLogger(t *testing.T, ctx context.Context, expected string, logF func(log logger.Logger)) {
	r, w, _ := os.Pipe()
	os.Stdout = w

	os.Setenv("LOG_LEVEL", "info")
	log := echo_google_cloud.NewComponentLogger(ctx, "test")

	logF(log)

	actual := ReadStdout(r, w)

	expected = googleCloudExpected(expected)
	tests.CompareJSONBody(t, []byte(actual), []byte(expected))
}

func TestLog_Info(t *testing.T) {
	ctx := keys.SetCtxValue(context.Background(), keys.LogInfo, logger.ServiceContext{
		Service:   "test-service",
		Version:   "v0.1",
		User:      "0001",
		RequestID: "asd-asd-asd",
		MSGID:     "ttt-tttt-ttt",
		HTTPRequest: &logger.HTTPRequestContext{
			Method:             "GET",
			URL:                "/test",
			UserAgent:          "testagent",
			Referrer:           "testReferrer",
			ResponseStatusCode: 200,
			RemoteIP:           "0.0.0.0",
		},
	})

	expected := `
		{
			"level": "info",
			"component": "test",
			"info": "details",
			"severity": "INFO",
			"serviceContext": {
				"service": "test-service",
				"version": "v0.1",
				"user": "0001",
				"request_id": "asd-asd-asd",
				"msg_id": "ttt-tttt-ttt",
				"httpRequest": {
					"method": "GET",
					"url": "/test",
					"userAgent": "testagent",
					"referrer": "testReferrer",
					"responseStatusCode": 200,
					"remoteIp": "0.0.0.0"
				}
			},
			"time": "%any%",
			"message": "test message"
		}
	`

	testBaseLogger(t, nil, expected, func(log logger.Logger) {
		log.Info().Ctx(ctx).Str("info", "details").Msg("test message")
	})

	testBaseLogger(t, ctx, expected, func(log logger.Logger) {
		log.Info().Str("info", "details").Msg("test message")
	})

	testEchoLogger(t, nil, expected, func(log logger.Logger) {
		log.Info().Ctx(ctx).Str("info", "details").Msg("test message")
	})

	testEchoLogger(t, ctx, expected, func(log logger.Logger) {
		log.Info().Str("info", "details").Msg("test message")
	})
}

func TestLog_ErrorWithoutStack(t *testing.T) {
	ctx := keys.SetCtxValue(context.Background(), keys.LogInfo, logger.ServiceContext{
		Service:   "test-service",
		Version:   "v0.1",
		User:      "0001",
		RequestID: "asd-asd-asd",
		MSGID:     "ttt-tttt-ttt",
		HTTPRequest: &logger.HTTPRequestContext{
			Method:             "GET",
			URL:                "/test",
			UserAgent:          "testagent",
			Referrer:           "testReferrer",
			ResponseStatusCode: 200,
			RemoteIP:           "0.0.0.0",
		},
	})

	expected := `
		{
			"level": "error",
			"component": "test",
			"error": "some critical error",
			"severity": "ERROR",
			"serviceContext": {
				"service": "test-service",
				"version": "v0.1",
				"user": "0001",
				"request_id": "asd-asd-asd",
				"msg_id": "ttt-tttt-ttt",
				"httpRequest": {
					"method": "GET",
					"url": "/test",
					"userAgent": "testagent",
					"referrer": "testReferrer",
					"responseStatusCode": 200,
					"remoteIp": "0.0.0.0"
				}
			},
			"time": "%any%",
			"message": "test message"
		}
	`

	testBaseLogger(t, nil, expected, func(log logger.Logger) {
		log.Error().Ctx(ctx).Err(errors.New("some critical error")).Msg("test message")
	})

	testBaseLogger(t, ctx, expected, func(log logger.Logger) {
		log.Error().Err(errors.New("some critical error")).Msg("test message")
	})

	testEchoLogger(t, nil, expected, func(log logger.Logger) {
		log.Error().Ctx(ctx).Err(errors.New("some critical error")).Msg("test message")
	})

	testEchoLogger(t, ctx, expected, func(log logger.Logger) {
		log.Error().Err(errors.New("some critical error")).Msg("test message")
	})
}

func TestLog_ErrorWithStack(t *testing.T) {
	ctx := keys.SetCtxValue(context.Background(), keys.LogInfo, logger.ServiceContext{
		Service:   "test-service",
		Version:   "v0.1",
		User:      "0001",
		RequestID: "asd-asd-asd",
		MSGID:     "ttt-tttt-ttt",
		HTTPRequest: &logger.HTTPRequestContext{
			Method:             "GET",
			URL:                "/test",
			UserAgent:          "testagent",
			Referrer:           "testReferrer",
			ResponseStatusCode: 200,
			RemoteIP:           "0.0.0.0",
		},
	})

	err := errors.Wrap(errors.New("error message"), "from error")

	expected := `
		{
			"level": "error",
			"component": "test",
			"stack": [
				{
					"func": "TestLog_ErrorWithStack",
					"line": "%any%",
					"source": "loggers_test.go"
				},
				{
					"func": "tRunner",
					"line": "%any%",
					"source": "testing.go"
				},
				{
					"func": "goexit",
					"line": "%any%",
					"source": "asm_amd64.s"
				}
			],
			"error": "from error: error message",
			"severity": "ERROR",
			"serviceContext": {
				"service": "test-service",
				"version": "v0.1",
				"user": "0001",
				"request_id": "asd-asd-asd",
				"msg_id": "ttt-tttt-ttt",
				"httpRequest": {
					"method": "GET",
					"url": "/test",
					"userAgent": "testagent",
					"referrer": "testReferrer",
					"responseStatusCode": 200,
					"remoteIp": "0.0.0.0"
				}
			},
			"time": "%any%",
			"message": "test message"
		}
	`

	testBaseLogger(t, nil, expected, func(log logger.Logger) {
		log.Error().Ctx(ctx).Stack().Err(err).Msg("test message")
	})

	testBaseLogger(t, ctx, expected, func(log logger.Logger) {
		log.Error().Stack().Err(err).Msg("test message")
	})

	testEchoLogger(t, nil, expected, func(log logger.Logger) {
		log.Error().Ctx(ctx).Stack().Err(err).Msg("test message")
	})

	testEchoLogger(t, ctx, expected, func(log logger.Logger) {
		log.Error().Stack().Err(err).Msg("test message")
	})
}
