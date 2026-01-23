package middleware

import (
	"log"
	"net/http"
	"sync"

	"github.com/harliandi/go-heif/pkg/metrics"
)

// ConcurrencyLimiter limits the number of concurrent requests
type ConcurrencyLimiter struct {
	semaphore chan struct{}
	mu        sync.RWMutex
	active    int
	max       int
}

// NewConcurrencyLimiter creates a new concurrency limiter
func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		semaphore: make(chan struct{}, max),
		max:       max,
	}
}

// Acquire tries to acquire a slot. Returns false if limit is reached
func (cl *ConcurrencyLimiter) Acquire() bool {
	select {
	case cl.semaphore <- struct{}{}:
		cl.mu.Lock()
		cl.active++
		active := cl.active
		cl.mu.Unlock()
		log.Printf("Concurrency: %d/%d active", active, cl.max)
		metrics.UpdateConcurrency(active)
		return true
	default:
		return false
	}
}

// Release releases a slot
func (cl *ConcurrencyLimiter) Release() {
	<-cl.semaphore
	cl.mu.Lock()
	cl.active--
	active := cl.active
	cl.mu.Unlock()
	log.Printf("Concurrency: %d/%d active", active, cl.max)
	metrics.UpdateConcurrency(active)
}

// ConcurrencyLimit returns middleware that enforces concurrency limits
func ConcurrencyLimit(max int) func(http.Handler) http.Handler {
	cl := NewConcurrencyLimiter(max)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cl.Acquire() {
				log.Printf("Concurrency limit reached: %d", max)
				metrics.RecordConcurrencyLimitExceeded()
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"error":"Service busy, please try again"}`))
				return
			}

			defer cl.Release()
			next.ServeHTTP(w, r)
		})
	}
}
