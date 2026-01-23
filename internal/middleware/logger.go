package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/harliandi/go-heif/pkg/metrics"
)

// Logger is a middleware that logs HTTP requests and records metrics
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWrapper{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()
		statusCode := fmt.Sprintf("%d", wrapped.status)

		// Log the request
		log.Printf("%s %s %d %v",
			r.Method,
			r.URL.Path,
			wrapped.status,
			time.Since(start),
		)

		// Record metrics (excluding /metrics endpoint to avoid recursion)
		if r.URL.Path != "/metrics" {
			metrics.RecordRequest(r.Method, r.URL.Path, statusCode, duration)
		}
	})
}

// Recovery is a middleware that recovers from panics and returns HTTP 500
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				log.Printf("PANIC recovered: %v\n%s", err, stack)

				// Try to send error response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, `{"error":"Internal server error","message":"Request failed unexpectedly"}`)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

type responseWrapper struct {
	http.ResponseWriter
	status int
}

func (w *responseWrapper) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
