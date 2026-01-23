package middleware

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/harliandi/go-heif/pkg/metrics"
)

// RateLimiter implements token bucket rate limiting per IP address
type RateLimiter struct {
	mu     sync.Mutex
	limits map[string]*bucket
	rate   int           // tokens per second
	burst  int           // max burst size
	ttl    time.Duration // cleanup interval for stale entries
}

type bucket struct {
	tokens  float64
	lastRef time.Time
}

// NewRateLimiter creates a new rate limiter
// rate: requests per second allowed
// burst: maximum burst size (tokens can accumulate to this)
func NewRateLimiter(rate, burst int) *RateLimiter {
	rl := &RateLimiter{
		limits: make(map[string]*bucket),
		rate:   rate,
		burst:  burst,
		ttl:    5 * time.Minute,
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, exists := rl.limits[ip]
	now := time.Now()

	if !exists {
		b = &bucket{
			tokens:  float64(rl.burst) - 1, // Consume one token
			lastRef: now,
		}
		rl.limits[ip] = b
		return true
	}

	// Calculate tokens to add based on time passed
	elapsed := now.Sub(b.lastRef).Seconds()
	tokensToAdd := elapsed * float64(rl.rate)
	b.tokens += tokensToAdd
	b.lastRef = now

	// Cap tokens at burst size
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}

	// Check if we have enough tokens
	if b.tokens >= 1 {
		b.tokens -= 1
		return true
	}

	return false
}

// cleanup removes stale entries to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, b := range rl.limits {
			if now.Sub(b.lastRef) > rl.ttl {
				delete(rl.limits, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// getIP extracts the client IP from the request
func getIP(r *http.Request) string {
	// Check X-Forwarded-For header (for proxies/load balancers)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP
		for i, c := range xff {
			if c == ' ' || c == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// getIPPrefix extracts the first octet of an IP for privacy-preserving metrics
func getIPPrefix(ip string) string {
	// Remove port if present
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	// Extract first octet
	if idx := strings.Index(ip, "."); idx != -1 {
		return ip[:idx] + ".0.0.0"
	}
	// For IPv6, just return first part
	if idx := strings.Index(ip, ":"); idx != -1 {
		return ip[:idx] + ":"
	}
	return "unknown"
}

// RateLimit returns middleware that enforces rate limiting
func RateLimit(rate, burst int) func(http.Handler) http.Handler {
	rl := NewRateLimiter(rate, burst)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)

			if !rl.Allow(ip) {
				log.Printf("Rate limit exceeded for IP: %s", ip)
				// Record metric for rate limit exceeded
				metrics.RecordRateLimitExceeded(getIPPrefix(ip))
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"Rate limit exceeded"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
