package middleware

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
)

type atomicBuffer struct {
	header   http.Header
	snapshot http.Header
	body     bytes.Buffer
	status   int
	limit    int64
	err      error
}

func newAtomicBuffer(header http.Header, limit int64) *atomicBuffer {
	return &atomicBuffer{header: header.Clone(), limit: limit}
}

func (b *atomicBuffer) Header() http.Header { return b.header }

func (b *atomicBuffer) WriteHeader(status int) {
	if b.status != 0 || b.err != nil {
		return
	}

	if status < 200 || status > 599 {
		b.err = fmt.Errorf("%w: status %d", errAtomicResponse, status)
		return
	}

	b.status = status
	b.snapshot = b.header.Clone()
}

func (b *atomicBuffer) Write(data []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}

	if b.status == 0 {
		b.WriteHeader(http.StatusOK)
	}

	if (b.status == http.StatusNoContent || b.status == http.StatusNotModified) && len(data) > 0 {
		return 0, b.fail(http.ErrBodyNotAllowed)
	}

	if int64(len(data)) > b.limit-int64(b.body.Len()) {
		return 0, b.fail(fmt.Errorf("%w: exceeds MaxResponseBytes", errAtomicResponse))
	}

	return b.body.Write(data)
}

func (b *atomicBuffer) Flush() {
	_ = b.FlushError() // Failure is latched because http.Flusher cannot return errors.
}

func (b *atomicBuffer) FlushError() error {
	return b.fail(fmt.Errorf("%w: flushing", errAtomicResponse))
}

func (b *atomicBuffer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, b.fail(fmt.Errorf("%w: hijacking", errAtomicResponse))
}

func (b *atomicBuffer) Push(string, *http.PushOptions) error {
	return b.fail(fmt.Errorf("%w: server push", errAtomicResponse))
}

func (b *atomicBuffer) fail(err error) error {
	if b.err == nil {
		b.err = err
	}

	return b.err
}

func (b *atomicBuffer) validate() error {
	if b.err != nil {
		return b.err
	}

	for _, header := range []http.Header{b.snapshot, b.header} {
		for key := range header {
			if strings.EqualFold(key, "Trailer") || strings.HasPrefix(strings.ToLower(key), "trailer:") {
				return b.fail(fmt.Errorf("%w: trailers", errAtomicResponse))
			}
		}
	}

	if strings.HasPrefix(strings.ToLower(b.snapshot.Get("Content-Type")), "text/event-stream") ||
		b.snapshot.Get("Transfer-Encoding") != "" {
		return b.fail(fmt.Errorf("%w: streaming", errAtomicResponse))
	}

	values := b.snapshot.Values("Content-Length")
	if len(values) > 0 {
		length, err := strconv.ParseInt(values[0], 10, 64)
		if len(values) != 1 || err != nil || strings.Trim(values[0], "0123456789") != "" || length < 0 ||
			(b.status != http.StatusNotModified && length != int64(b.body.Len())) ||
			b.status == http.StatusNoContent {
			return b.fail(fmt.Errorf("%w: invalid Content-Length", errAtomicResponse))
		}
	}

	return nil
}

func copyAtomicHeaders(target, source http.Header) {
	clear(target)

	for key, values := range source {
		target[key] = append([]string(nil), values...)
	}
}
