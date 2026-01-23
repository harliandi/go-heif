package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSecurityHeaders tests security headers are set correctly
func TestSecurityHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Security(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	tests := []struct {
		header  string
		want    string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Content-Security-Policy", "default-src 'none'"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "no-referrer"},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := w.Header().Get(tt.header)
			if got != tt.want {
				t.Errorf("%s header = %s, want %s", tt.header, got, tt.want)
			}
		})
	}
}

// TestRateLimit_Basic tests basic rate limiting
func TestRateLimit_Basic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(2, 2)(next) // 2 requests/sec, burst 2

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	// First request should pass
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("First request should pass, got %d", w.Code)
	}

	// Second request should pass
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Second request should pass, got %d", w.Code)
	}

	// Third request should be rate limited
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Third request should be rate limited, got %d", w.Code)
	}
}

// TestRateLimit_DifferentIPs tests rate limiting is per IP
func TestRateLimit_DifferentIPs(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(1, 1)(next) // 1 request/sec, burst 1

	// IP 1
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	// IP 2 (different IP, should not be rate limited)
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "192.168.1.2:1234"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w1.Code != http.StatusOK {
		t.Errorf("IP 1 first request should pass, got %d", w1.Code)
	}
	if w2.Code != http.StatusOK {
		t.Errorf("IP 2 first request should pass, got %d", w2.Code)
	}
}

// TestRateLimit_Refill tests token bucket refills over time
func TestRateLimit_Refill(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(5, 1)(next) // 5 requests/sec, burst 1

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	// First request should pass
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("First request should pass, got %d", w.Code)
	}

	// Second request should be rate limited immediately
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Second request should be rate limited, got %d", w.Code)
	}

	// Wait for refill
	time.Sleep(250 * time.Millisecond)

	// After wait, should have tokens available
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Request after refill should pass, got %d", w.Code)
	}
}

// TestConcurrencyLimit_Basic tests concurrent request limiting
func TestConcurrencyLimit_Basic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow request
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	handler := ConcurrencyLimit(2)(next)

	// Track results
	var successCount, rejectedCount int32
	var wg sync.WaitGroup

	// Launch 5 concurrent requests
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				atomic.AddInt32(&successCount, 1)
			} else if w.Code == http.StatusServiceUnavailable {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}()
	}

	wg.Wait()

	// At most 2 should succeed, at least 3 should be rejected
	if successCount > 2 {
		t.Errorf("Expected at most 2 successful requests, got %d", successCount)
	}
	if rejectedCount < 3 {
		t.Errorf("Expected at least 3 rejected requests, got %d", rejectedCount)
	}
}

// TestConcurrencyLimit_Sequential tests sequential requests all pass
func TestConcurrencyLimit_Sequential(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := ConcurrencyLimit(2)(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// All sequential requests should pass
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Sequential request %d should pass, got %d", i, w.Code)
		}
	}
}

// TestRecovery tests panic recovery
func TestRecovery(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := Recovery(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic, return 500 instead
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 after panic, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

// TestRecovery_NoPanic tests normal requests pass through
func TestRecovery_NoPanic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	handler := Recovery(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "OK" {
		t.Errorf("Expected body 'OK', got %s", body)
	}
}

// TestLogger tests logging middleware records metrics
func TestLogger(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := Logger(next)
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	handler.ServeHTTP(w, req)

	// Status should be passed through
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}
}

// TestMiddlewareChaining tests multiple middleware work together
func TestMiddlewareChaining(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Chain: Security -> RateLimit -> ConcurrencyLimit -> Recovery -> Logger -> next
	handler := Security(
		RateLimit(100, 10)(
			ConcurrencyLimit(10)(
				Recovery(
					Logger(next),
				),
			),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Request should complete successfully
	if w.Code != http.StatusOK {
		t.Errorf("Chained middleware should pass, got %d", w.Code)
	}

	// Security headers should be set
	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Security headers should be set")
	}
}

// TestRateLimit_IPv6 tests IPv6 addresses work correctly
func TestRateLimit_IPv6(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(1, 1)(next)

	// IPv6 address
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "[2001:db8::1]:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("IPv6 address first request should pass, got %d", w.Code)
	}

	// Same IPv6, second request should be rate limited
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("IPv6 address second request should be rate limited, got %d", w.Code)
	}
}

// TestRateLimit_NoPort tests addresses without port work correctly
func TestRateLimit_NoPort(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimit(1, 1)(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1" // No port
	w := httptest.NewRecorder()

	// Should handle gracefully
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Address without port should work, got %d", w.Code)
	}
}

// TestConcurrencyLimit_Decrement tests semaphore is released after request
func TestConcurrencyLimit_Decrement(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := ConcurrencyLimit(1)(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// First request
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req)
	if w1.Code != http.StatusOK {
		t.Errorf("First request should pass, got %d", w1.Code)
	}

	// Second request immediately after (should also pass since first completed)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Errorf("Second request should pass after first completes, got %d", w2.Code)
	}
}

// TestRecovery_NilPanic tests recovery from nil panic
func TestRecovery_NilPanic(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil)
	})

	handler := Recovery(next)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Should handle nil panic gracefully
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500 after nil panic, got %d", w.Code)
	}
}
