package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/devonartis/agentwrit/internal/obs"
	"github.com/devonartis/agentwrit/internal/problemdetails"
)

// responseWriter is a wrapper around http.ResponseWriter that captures
// the status code so it can be logged.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// LoggingMiddleware logs HTTP request details including method, path,
// status code, latency, and request ID.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r)

		latency := time.Since(start)
		id := problemdetails.GetRequestID(r.Context())

		obs.Ok("HTTP", "handler", "request completed",
			fmt.Sprintf("method=%s", r.Method),
			fmt.Sprintf("path=%s", r.URL.Path),
			fmt.Sprintf("status=%d", rw.status),
			fmt.Sprintf("latency=%s", latency),
			fmt.Sprintf("request_id=%s", id),
		)
	})
}
