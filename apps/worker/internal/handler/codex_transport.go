package handler

import (
	"io"
	"net/http"
)

// inMemoryTransport implements http.RoundTripper by dispatching requests
// directly to an http.Handler (the embedded Codex runtime gin.Engine)
// without going through a real TCP connection.
//
// Streaming is fully supported: the handler writes into a pipe and the
// caller reads the http.Response.Body from the other end.
type inMemoryTransport struct {
	handler http.Handler
}

// NewInMemoryTransport creates a RoundTripper that routes every request
// to the given http.Handler in-process.
func NewInMemoryTransport(h http.Handler) http.RoundTripper {
	return &inMemoryTransport{handler: h}
}

// RoundTrip satisfies http.RoundTripper. It creates an io.Pipe so that
// the http.Handler can write the response (including streaming SSE) in a
// goroutine while the caller reads it as a normal http.Response.
func (t *inMemoryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	pr, pw := io.Pipe()

	// pipeResponseWriter captures the status code and headers, then streams
	// the body through the pipe.
	rw := &pipeResponseWriter{
		pw:         pw,
		header:     make(http.Header),
		headerSent: make(chan struct{}),
	}

	go func() {
		defer pw.Close()
		t.handler.ServeHTTP(rw, req)
	}()

	// Wait until the handler has written the status line + headers.
	<-rw.headerSent

	resp := &http.Response{
		StatusCode: rw.statusCode,
		Header:     rw.header,
		Body:       pr,
		Request:    req,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	return resp, nil
}

// pipeResponseWriter is an http.ResponseWriter that sends the body
// through an io.PipeWriter for streaming support.
type pipeResponseWriter struct {
	pw         *io.PipeWriter
	header     http.Header
	statusCode int
	headerSent chan struct{} // closed after WriteHeader
	headerDone bool
}

var _ http.ResponseWriter = (*pipeResponseWriter)(nil)
var _ http.Flusher = (*pipeResponseWriter)(nil)

func (w *pipeResponseWriter) Header() http.Header {
	return w.header
}

func (w *pipeResponseWriter) WriteHeader(code int) {
	if w.headerDone {
		return
	}
	w.headerDone = true
	w.statusCode = code
	close(w.headerSent)
}

func (w *pipeResponseWriter) Write(b []byte) (int, error) {
	if !w.headerDone {
		w.WriteHeader(http.StatusOK)
	}
	return w.pw.Write(b)
}

func (w *pipeResponseWriter) Flush() {
	// The pipe already makes bytes available immediately; nothing extra needed.
}
