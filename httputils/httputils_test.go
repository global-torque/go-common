package httputils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestSendRequest(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=ISO-8859-1")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("cannot parse test server URL: %v", err)
	}

	req, err := CreateDefaultRequest(
		ctx,
		Request{
			Host:   serverURL.Host,
			Scheme: serverURL.Scheme,
			Method: "GET",
			Path:   "/",
			Body:   []byte{},
		},
	)
	if err != nil {
		t.Errorf("cannot create default request: %s", err.Error())
		t.FailNow()
	}

	_, resp, err := SendRequest(req)
	if err != nil {
		t.Errorf("cannot send request: %s", err.Error())
		t.FailNow()
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 code, got %d", resp.StatusCode)
		t.FailNow()
	}

	if resp.Header.Get("Content-Type") != "text/html; charset=ISO-8859-1" {
		t.Errorf("expected text/html; charset=ISO-8859-1, got %s", resp.Header.Get("Content-Type"))
		t.FailNow()
	}
}

func TestCreateRequestWithFilesIncludesFormFields(t *testing.T) {
	type ctxKey string

	ctx := context.WithValue(context.Background(), ctxKey("request-id"), "test-request")
	filePath := filepath.Join(t.TempDir(), "upload.txt")
	if err := os.WriteFile(filePath, []byte("file-body"), 0o600); err != nil {
		t.Fatalf("cannot write temp upload file: %v", err)
	}

	req, err := CreateRequestWithFiles(
		ctx,
		Request{
			Host:   "example.test",
			Scheme: "http",
			Method: http.MethodPost,
			Path:   "/upload",
		},
		map[string]any{"name": "sample"},
		map[string]string{"file": filePath},
	)
	if err != nil {
		t.Fatalf("cannot create multipart request: %v", err)
	}

	if got := req.Context().Value(ctxKey("request-id")); got != "test-request" {
		t.Fatalf("request did not preserve context value: %#v", got)
	}

	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("cannot parse multipart request: %v", err)
	}
	if got := req.MultipartForm.Value["name"]; len(got) != 1 || got[0] != "sample" {
		t.Fatalf("expected normal form field, got %#v", got)
	}
	if got := req.MultipartForm.File["file"]; len(got) != 1 {
		t.Fatalf("expected uploaded file, got %#v", got)
	}
}
