package httputils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/pkg/errors"
)

type Request struct {
	HttpClient         *http.Client
	Host, Port, Scheme string
	Method, Path       string
	Body               []byte
	Headers            map[string]string
}

// CreateDefaultRequest default json request
func CreateDefaultRequest(ctx context.Context, req Request) (*http.Request, error) {
	req = defaultRequestHost(req)
	if req.Port != "" {
		req.Host = net.JoinHostPort(req.Host, req.Port)
	}
	if req.Scheme == "" {
		req.Scheme = "http"
	}

	res, err := http.NewRequestWithContext(
		ctx,
		req.Method,
		fmt.Sprintf("%s://%s%s", req.Scheme, req.Host, req.Path),
		bytes.NewBuffer((req.Body)),
	)
	if err != nil {
		return res, errors.Wrapf(err, "cannot create new request")
	}

	// if no content type set
	if ok := res.Header.Get("Content-Type"); ok == "" {
		res.Header.Add("Content-Type", "application/json")
	}

	for key, value := range req.Headers {
		res.Header.Add(key, value)
	}

	return res, nil
}

// CreateRequestWithFiles creates a multipart request with the provided context.
func CreateRequestWithFiles(
	ctx context.Context,
	req Request,
	body map[string]any,
	files map[string]string,
) (*http.Request, error) {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	req = defaultRequestHost(req)
	if req.Port != "" {
		req.Host = net.JoinHostPort(req.Host, req.Port)
	}
	if req.Scheme == "" {
		req.Scheme = "http"
	}

	if req.Body != nil {
		return nil, errors.New("req body should be empty, use body parameter")
	}

	values := map[string]io.Reader{}
	for k, v := range body {
		if vs, ok := v.(string); ok {
			if err := w.WriteField(k, vs); err != nil {
				_ = w.Close()
				return nil, errors.Wrapf(err, "cannot write form field %s", k)
			}
		}
	}

	for k, v := range files {
		f, err := os.Open(v)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot open file %s", v)
		}
		values[k] = f
	}

	for key, r := range values {
		var fw io.Writer
		var err error
		x, ok := r.(io.Closer)
		if !ok {
			continue
		}
		// upload a file
		if _, ok := r.(*os.File); ok {
			if fw, err = w.CreateFormFile(key, files[key]); err != nil {
				_ = w.Close()
				_ = x.Close()
				return nil, errors.Wrapf(err, "cannot CreateFormFile %s", key)
			}
		} else {
			// Add other fields
			if fw, err = w.CreateFormField(key); err != nil {
				_ = w.Close()
				_ = x.Close()
				return nil, errors.Wrapf(err, "cannot CreateFormField %s", key)
			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			_ = x.Close()
			return nil, errors.Wrapf(err, "cannot io.Copy %s", key)
		}

		_ = x.Close()
	}
	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	if err := w.Close(); err != nil {
		return nil, errors.Wrapf(err, "cannot close multipart writer")
	}

	// Now that you have a form, you can submit it to your handler.
	res, err := http.NewRequestWithContext(
		ctx,
		req.Method,
		fmt.Sprintf("%s://%s%s", req.Scheme, req.Host, req.Path),
		buf,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create new request")
	}

	// Don't forget to set the content type, this will contain the boundary.
	res.Header.Set("Content-Type", w.FormDataContentType())
	// Set up content length
	res.Header.Set("Content-Length", strconv.FormatInt(res.ContentLength, 10))

	return res, nil
}

func defaultRequestHost(req Request) Request {
	if req.Host == "" {
		req.Host = os.Getenv("HOST")
		if req.Port == "" {
			req.Port = os.Getenv("PORT")
		}
	}

	return req
}

func request(httpClient *http.Client, req *http.Request) ([]byte, *http.Response, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot read response body")
	}

	return bodyBytes, resp, nil
}

func SendRequest(req *http.Request) ([]byte, *http.Response, error) {
	httpClient := &http.Client{}

	body, resp, err := request(httpClient, req)
	if err != nil {
		return nil, resp, err
	}

	return body, resp, nil
}

func SendRequestWithClient(httpClient *http.Client) func(req *http.Request) ([]byte, *http.Response, error) {
	return func(req *http.Request) ([]byte, *http.Response, error) {
		return request(httpClient, req)
	}
}
