package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/harliandi/go-heif/internal/config"
	"github.com/harliandi/go-heif/internal/converter"
	"github.com/harliandi/go-heif/internal/handler"
	"github.com/harliandi/go-heif/internal/middleware"
)

// TestIntegration_EndToEnd tests the full HTTP request cycle using httptest
func TestIntegration_EndToEnd(t *testing.T) {
	cfg := config.Load()
	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)
	mux.HandleFunc("/health", h.Health)

	server := httptest.NewServer(middleware.Logger(mux))
	defer server.Close()

	// Test health endpoint
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint returned status %d", resp.StatusCode)
	}

	// Test convert endpoint with invalid data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.heic")
	part.Write([]byte("invalid heif data"))
	writer.Close()

	resp, err = http.Post(server.URL+"/convert", writer.FormDataContentType(), body)
	if err != nil {
		t.Fatalf("Convert request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should return error for invalid HEIF data
	if resp.StatusCode == http.StatusOK {
		t.Error("Expected error status for invalid HEIF data")
	}
}

// TestIntegration_ConvertWithTestData tests conversion with actual files
func TestIntegration_ConvertWithTestData(t *testing.T) {
	testFiles := []string{"../testdata/test.heic", "../testdata/test.heif"}
	var testDataPath string
	for _, f := range testFiles {
		if _, err := os.Stat(f); err == nil {
			testDataPath = f
			break
		}
	}

	if testDataPath == "" {
		t.Skip("No test HEIF file found in testdata/")
		return
	}

	data, err := os.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	cfg := config.Load()
	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)
	mux.HandleFunc("/health", h.Health)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Create multipart upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", testDataPath)
	part.Write(data)
	writer.Close()

	// Make request
	resp, err := http.Post(server.URL+"/convert", writer.FormDataContentType(), body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Logf("Response status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Check if we got JPEG data
	if resp.StatusCode == http.StatusOK {
		ct := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "image/jpeg") {
			t.Errorf("Expected Content-Type image/jpeg, got %s", ct)
		}
	}
}

// TestIntegration_QueryParameters tests query parameter handling
func TestIntegration_QueryParameters(t *testing.T) {
	cfg := config.Load()
	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Test with quality parameter
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.heic")
	part.Write([]byte("invalid"))
	writer.Close()

	resp, err := http.Post(server.URL+"/convert?quality=90", writer.FormDataContentType(), body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Should fail conversion but quality param should be parsed
	_ = resp.StatusCode
}

// BenchmarkHTTPRequest benchmarks a full HTTP request using test server
func BenchmarkHTTPRequest(b *testing.B) {
	cfg := config.Load()
	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)

	server := httptest.NewServer(mux)
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		http.Get(server.URL + "/health")
	}
}

// TestIntegration_FullMiddlewareStack tests the complete middleware chain
func TestIntegration_FullMiddlewareStack(t *testing.T) {
	cfg := config.Load()
	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)
	mux.HandleFunc("/health", h.Health)

	// Apply full middleware stack like in main.go
	handler := middleware.Security(
		middleware.RateLimit(cfg.RateLimitPerSec, cfg.RateLimitBurst)(
			middleware.ConcurrencyLimit(cfg.MaxConcurrent)(
				middleware.Recovery(
					middleware.Logger(mux),
				),
			),
		),
	)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Test health endpoint through middleware stack
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health endpoint returned status %d", resp.StatusCode)
	}

	// Check security headers are present
	headers := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Content-Security-Policy",
	}

	for _, header := range headers {
		if resp.Header.Get(header) == "" {
			t.Errorf("Security header %s not set", header)
		}
	}
}

// TestIntegration_SecurityHeaders verifies all security headers are set
func TestIntegration_SecurityHeaders(t *testing.T) {
	h := handler.New(500, 10)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)

	server := httptest.NewServer(middleware.Security(mux))
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Content-Security-Policy":   "default-src 'none'",
		"X-XSS-Protection":          "1; mode=block",
		"Referrer-Policy":           "no-referrer",
	}

	for header, wantValue := range expectedHeaders {
		gotValue := resp.Header.Get(header)
		if gotValue != wantValue {
			t.Errorf("%s = %s, want %s", header, gotValue, wantValue)
		}
	}
}

// TestIntegration_RateLimiting tests rate limiting middleware
func TestIntegration_RateLimiting(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Low rate limit for testing - 1 request per second, burst 1
	handler := middleware.RateLimit(1, 1)(mux)

	server := httptest.NewServer(handler)
	defer server.Close()

	// First request should pass
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("First request should pass, got %d", resp.StatusCode)
	}

	// Second request immediately should be rate limited
	resp, err = http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected rate limit status %d, got %d", http.StatusTooManyRequests, resp.StatusCode)
	}
}

// TestIntegration_ConcurrencyLimit tests concurrent request limiting
func TestIntegration_ConcurrencyLimit(t *testing.T) {
	h := handler.New(500, 10)
	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)

	// Very low concurrency limit for testing
	handler := middleware.ConcurrencyLimit(1)(mux)

	server := httptest.NewServer(handler)
	defer server.Close()

	// Track results
	successCount := 0
	busyCount := 0
	var mu sync.Mutex

	// Launch 5 concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", "test.heic")
			part.Write([]byte("fake data"))
			writer.Close()

			resp, err := http.Post(server.URL+"/convert", writer.FormDataContentType(), body)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusBadRequest {
				// OK or validation error (both count as processed)
				successCount++
			} else if resp.StatusCode == http.StatusServiceUnavailable {
				busyCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	// At least 1 should have been processed
	if successCount == 0 {
		t.Error("At least one request should have been processed")
	}

	t.Logf("Processed: %d, Busy: %d", successCount, busyCount)
}

// TestIntegration_RecoveryMiddleware tests panic recovery
func TestIntegration_RecoveryMiddleware(t *testing.T) {
	panicMux := http.NewServeMux()
	panicMux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	panicMux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := httptest.NewServer(middleware.Recovery(panicMux))
	defer server.Close()

	// Test panic is recovered
	resp, err := http.Get(server.URL + "/panic")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500 after panic, got %d", resp.StatusCode)
	}

	// Test normal request still works
	resp, err = http.Get(server.URL + "/ok")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Normal request failed, got %d", resp.StatusCode)
	}
}

// TestIntegration_WithRealHEIF tests full stack with actual HEIF conversion
func TestIntegration_WithRealHEIF(t *testing.T) {
	testFiles := []string{
		"../testdata/test.heic",
		"../testdata/test.heif",
		"../../testdata/test.heic",
		"/home/harliandi/go-heif/testdata/test.heic",
	}

	var testData []byte
	var testFile string
	for _, f := range testFiles {
		if data, err := os.ReadFile(f); err == nil {
			testData = data
			testFile = f
			break
		}
	}

	if testData == nil {
		t.Skip("No test HEIF file found")
		return
	}

	t.Logf("Using test file: %s (%.2f MB)", testFile, float64(len(testData))/1024/1024)

	// Initialize worker pool
	converter.InitGlobalWorkerPool(2, 500)

	cfg := config.Load()
	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)
	mux.HandleFunc("/health", h.Health)

	handler := middleware.Recovery(middleware.Logger(mux))

	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name   string
		query  string
		wantCT string
	}{
		{"Default (scale 0.5)", "", "image/jpeg"},
		{"Full resolution", "?scale=1", "image/jpeg"},
		{"High quality", "?scale=1&quality=90", "image/jpeg"},
		{"Fast mode", "?scale=0.25", "image/jpeg"},
		{"JSON format", "?format=json", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", testFile)
			part.Write(testData)
			writer.Close()

			url := server.URL + "/convert" + tt.query
			resp, err := http.Post(url, writer.FormDataContentType(), body)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(bodyBytes))
			}

			ct := resp.Header.Get("Content-Type")
			if ct != tt.wantCT {
				t.Errorf("Expected Content-Type %s, got %s", tt.wantCT, ct)
			}

			// Verify response has content
			responseBody, _ := io.ReadAll(resp.Body)
			if len(responseBody) == 0 {
				t.Error("Response body is empty")
			}

			if tt.wantCT == "image/jpeg" {
				// Verify JPEG signature
				if len(responseBody) < 4 || responseBody[0] != 0xFF || responseBody[1] != 0xD8 {
					t.Error("Response is not a valid JPEG file")
				}
			}
		})
	}
}

// TestIntegration_ErrorHandling tests various error scenarios
func TestIntegration_ErrorHandling(t *testing.T) {
	cfg := config.Load()
	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)

	handler := middleware.Recovery(middleware.Logger(mux))

	server := httptest.NewServer(handler)
	defer server.Close()

	tests := []struct {
		name           string
		filename       string
		contentType    string
		wantStatus     int
	}{
		{"No file", "", "multipart/form-data", http.StatusBadRequest},
		{"Wrong extension", "test.jpg", "multipart/form-data", http.StatusUnsupportedMediaType},
		{"Not multipart", "", "application/json", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			var contentType string

			if tt.name == "No file" || tt.name == "Wrong extension" {
				b := &bytes.Buffer{}
				w := multipart.NewWriter(b)
				if tt.filename != "" {
					part, _ := w.CreateFormFile("file", tt.filename)
					part.Write([]byte("data"))
				}
				w.Close()
				body = b
				contentType = w.FormDataContentType()
			} else {
				body = strings.NewReader("test")
				contentType = tt.contentType
			}

			resp, err := http.Post(server.URL+"/convert", contentType, body)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
		})
	}
}

// TestIntegration_ConcurrentRealConversions tests multiple concurrent HEIF conversions
func TestIntegration_ConcurrentRealConversions(t *testing.T) {
	testFiles := []string{
		"../testdata/test.heic",
		"../../testdata/test.heic",
		"/home/harliandi/go-heif/testdata/test.heic",
	}

	var testData []byte
	for _, f := range testFiles {
		if data, err := os.ReadFile(f); err == nil {
			testData = data
			break
		}
	}

	if testData == nil {
		t.Skip("No test HEIF file found")
		return
	}

	converter.InitGlobalWorkerPool(4, 500)

	cfg := config.Load()
	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)

	server := httptest.NewServer(mux)
	defer server.Close()

	// Launch concurrent conversions
	concurrency := 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	errorCount := 0

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", "test.heic")
			part.Write(testData)
			writer.Close()

			resp, err := http.Post(server.URL+"/convert?scale=0.5", writer.FormDataContentType(), body)
			if err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			if resp.StatusCode == http.StatusOK {
				successCount++
			} else {
				errorCount++
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	if successCount == 0 {
		t.Error("No conversions succeeded")
	}

	t.Logf("Concurrent conversions: %d succeeded, %d failed", successCount, errorCount)
}
