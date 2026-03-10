package relay

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipResponseWriter wraps http.ResponseWriter with gzip compression.
type gzipResponseWriter struct {
	http.ResponseWriter
	Writer io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	// Flush the gzip writer
	if gw, ok := w.Writer.(*gzip.Writer); ok {
		gw.Flush()
	}
	// Flush the underlying response writer
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(nil, gzip.BestSpeed) // Fast compression for streaming
		return w
	},
}

// newGzipResponseWriter creates a gzip-compressed response writer if the client accepts gzip.
// Returns the writer and a cleanup function. If gzip is not accepted, returns the original writer.
func newGzipResponseWriter(w http.ResponseWriter, r *http.Request) (http.ResponseWriter, func()) {
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return w, func() {}
	}

	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(w)

	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Del("Content-Length")

	return &gzipResponseWriter{ResponseWriter: w, Writer: gz}, func() {
		gz.Close()
		gzipWriterPool.Put(gz)
	}
}
