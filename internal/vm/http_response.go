package vm

import (
	"bytes"
	"net/http"
	"time"
)

// Helpers for writing HTTP responses to clients. They are kept apart from the
// opcode dispatcher in vm.go: they grant no authority (res has never needed a
// capability) and only bound how long a slow client may hold a connection.

// responseWriteTimeout bounds how long writing one HTTP response to a client may
// take. A variable so tests can shorten it.
var responseWriteTimeout = 30 * time.Second

// bufferedResponse is an http.ResponseWriter that keeps the response in memory
// until flushTo copies it to the real writer.
type bufferedResponse struct {
	header http.Header
	status int
	wrote  bool
	body   bytes.Buffer
}

func newBufferedResponse(initial http.Header) *bufferedResponse {
	return &bufferedResponse{header: initial.Clone()}
}

func (b *bufferedResponse) Header() http.Header { return b.header }

func (b *bufferedResponse) WriteHeader(status int) {
	if !b.wrote {
		b.status, b.wrote = status, true
	}
}

func (b *bufferedResponse) Write(p []byte) (int, error) {
	if !b.wrote {
		b.status, b.wrote = http.StatusOK, true
	}
	return b.body.Write(p)
}

// flushTo writes the collected response to w with a write deadline, so a client
// that stops reading releases its connection when the deadline passes. It must
// run outside the serialization lock.
func (b *bufferedResponse) flushTo(w http.ResponseWriter) {
	dst := w.Header()
	for k, v := range b.header {
		dst[k] = v
	}
	setResponseWriteDeadline(w)
	if b.wrote {
		w.WriteHeader(b.status)
		_, _ = w.Write(b.body.Bytes())
	}
}

// setResponseWriteDeadline bounds the next write to the client.
func setResponseWriteDeadline(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(responseWriteTimeout))
}
