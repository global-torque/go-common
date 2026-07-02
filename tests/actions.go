package tests

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/global-torque/go-common/httputils/v2"
)

type TestContext struct {
	T *testing.T
	//nolint:containedctx
	Ctx context.Context
}

type (
	SomeAction     func(t TestContext) error
	ExpectedResult map[string]interface{}
)

type ExpectedResponse struct {
	Code    int
	Body    []byte
	Headers http.Header
}

// DEPRECATED: use SendHTTPRequest instead
func SendHTTPRequst(req httputils.Request, expected ExpectedResponse) SomeAction {
	return SendHTTPRequest(req, expected)
}

func SendHTTPRequest(req httputils.Request, expected ExpectedResponse) SomeAction {
	return func(t TestContext) error {
		newReq, err := httputils.CreateDefaultRequest(t.Ctx, req)
		if !assert.NoError(t.T, err) {
			return err
		}

		doRequest := httputils.SendRequest
		if req.HttpClient != nil {
			doRequest = httputils.SendRequestWithClient(req.HttpClient)
		}

		result, resp, err := doRequest(newReq)
		if !assert.NoError(t.T, err) {
			return err
		}
		if !assert.NotNil(t.T, resp, "missing HTTP response") {
			return errors.New("missing HTTP response")
		}

		assert.Equal(t.T, expected.Code, resp.StatusCode, "Invalid response code")

		if expected.Headers != nil {
			asserts := assert.New(t.T)

			for key := range expected.Headers {
				expectedValue := expected.Headers[key][0]
				actualValue := resp.Header.Get(key)

				if expectedValue == "%any%" {
					continue
				}

				asserts.Equal(expectedValue, actualValue, "Invalid header value for %s", key)
			}
		}

		if expected.Body != nil {
			CompareJSONBody(t.T, result, expected.Body)
		}

		return nil
	}
}

func SendHTTPRequestFiles(req httputils.Request, body map[string]any, files map[string]string, expected ExpectedResponse) SomeAction {
	return func(t TestContext) error {
		newReq, err := httputils.CreateRequestWithFiles(t.Ctx, req, body, files)
		if !assert.NoError(t.T, err) {
			return err
		}

		doRequest := httputils.SendRequest
		if req.HttpClient != nil {
			doRequest = httputils.SendRequestWithClient(req.HttpClient)
		}

		result, resp, err := doRequest(newReq)
		if !assert.NoError(t.T, err) {
			return err
		}
		if !assert.NotNil(t.T, resp, "missing HTTP response") {
			return errors.New("missing HTTP response")
		}

		assert.Equal(t.T, expected.Code, resp.StatusCode, "Invalid response code")

		if expected.Headers != nil {
			asserts := assert.New(t.T)

			for key := range expected.Headers {
				expectedValue := expected.Headers[key][0]
				actualValue := resp.Header.Get(key)

				if expectedValue == "%any%" {
					continue
				}

				asserts.Equal(expectedValue, actualValue, "Invalid header value for %s", key)
			}
		}

		if expected.Body != nil {
			CompareJSONBody(t.T, result, expected.Body)
		}

		return nil
	}
}

func Sleep(d time.Duration) SomeAction {
	return func(_ TestContext) error {
		time.Sleep(d)

		return nil
	}
}
