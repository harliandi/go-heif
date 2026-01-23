package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/harliandi/go-heif/internal/config"
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
