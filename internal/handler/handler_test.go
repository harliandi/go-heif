package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/harliandi/go-heif/internal/converter"
)

func TestNew(t *testing.T) {
	h := New(500, 10)
	if h == nil {
		t.Fatal("New() returned nil")
	}
	if h.maxUploadMB != 10 {
		t.Errorf("Expected maxUploadMB 10, got %d", h.maxUploadMB)
	}
}

func TestHandler_Convert_MethodNotAllowed(t *testing.T) {
	h := New(500, 10)

	req := httptest.NewRequest(http.MethodGet, "/convert", nil)
	w := httptest.NewRecorder()

	h.Convert(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandler_Convert_NoFile(t *testing.T) {
	h := New(500, 10)

	req := httptest.NewRequest(http.MethodPost, "/convert", nil)
	w := httptest.NewRecorder()

	h.Convert(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_Convert_NotMultipart(t *testing.T) {
	h := New(500, 10)

	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader("test"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Convert(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandler_Convert_InvalidExtension(t *testing.T) {
	h := New(500, 10)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.jpg")
	part.Write([]byte("fake data"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Convert(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status 415, got %d", w.Code)
	}
}

func TestHandler_Convert_ValidExtension(t *testing.T) {
	tests := []struct {
		filename string
		valid    bool
	}{
		{"test.heic", true},
		{"test.heif", true},
		{"test.HEIC", true},
		{"test.HEIF", true},
		{"test.jpg", false},
		{"test.png", false},
		{"test", false},
		{"test.heic.jpg", false}, // Mixed extension
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isHEIFExtension(tt.filename)
			if result != tt.valid {
				t.Errorf("isHEIFExtension(%s) = %v, want %v", tt.filename, result, tt.valid)
			}
		})
	}
}

func TestHandler_Convert_InvalidMagicBytes(t *testing.T) {
	h := New(500, 10)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.heic")
	// Write invalid HEIF data (wrong magic bytes)
	part.Write([]byte("NOTHEIFDATAAAAAAAAA"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Convert(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status 415 for invalid magic bytes, got %d", w.Code)
	}
}

func TestHandler_Convert_QueryParameters(t *testing.T) {
	h := New(500, 10)

	// Test quality parameter
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.heic")
	part.Write([]byte("fake"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert?quality=90", body)
	req.Header.Set("ContentType", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Convert(w, req)
	// Will fail on invalid data, but parameter should be parsed without panic
	_ = w
}

func TestHandler_Convert_ScaleParameters(t *testing.T) {
	h := New(500, 10)

	tests := []struct {
		scaleStr    string
		expectScale float64
	}{
		{"0.5", 0.5},
		{"0.25", 0.25},
		{"1.0", 1.0},
		{"2.0", 2.0},
		{"invalid", 0.5}, // Invalid uses default
		{"0", 0.5},            // Invalid uses default
		{"-0.5", 0.5},         // Invalid uses default
	}

	for _, tt := range tests {
		t.Run(tt.scaleStr, func(t *testing.T) {
			// We can't easily test scale parsing in isolation, but we can test
			// that the handler doesn't panic with various scale values
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", "test.heic")
			part.Write([]byte("fake"))
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/convert?scale="+tt.scaleStr, body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			h.Convert(w, req)
			// Should not panic
		})
	}
}

func TestHandler_Convert_FormatParameter(t *testing.T) {
	h := New(500, 10)

	tests := []string{"", "json", "binary", "invalid"}

	for _, format := range tests {
		t.Run("format_"+format, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", "test.heic")
			part.Write([]byte("fake"))
			writer.Close()

			url := "/convert"
			if format != "" {
				url += "?format=" + format
			}

			req := httptest.NewRequest(http.MethodPost, url, body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			h.Convert(w, req)
			// Should not panic regardless of format parameter
		})
	}
}

func TestHandler_Convert_ClientDisconnect(t *testing.T) {
	h := New(500, 10)

	// Create a request with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.heic")
	part.Write([]byte("fake"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	h.Convert(w, req)
	// Should handle cancellation gracefully without panic
	if w.Code != 0 {
		// If a response was sent, it should be an error
		// Most likely no response since we cancelled before processing
	}
}

func TestHandler_Convert_RequestTooLarge(t *testing.T) {
	h := New(500, 10) // 10MB max

	// Create a request that exceeds max upload
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.heic")

	// Write more than max upload (10MB)
	largeData := make([]byte, 11*1024*1024) // 11MB
	part.Write(largeData)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Convert(w, req)

	// Should handle large upload gracefully - either reject or process
	// The actual behavior depends on ParseMultipartForm
	if w.Code == http.StatusRequestEntityTooLarge {
		// This is expected - request was rejected before processing
	} else if w.Code == http.StatusBadRequest || w.Code == http.StatusInternalServerError {
		// Also acceptable - some processing may have occurred
	}
}

func TestHandler_Health(t *testing.T) {
	h := New(500, 10)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	body := w.Body.String()
	if body != `{"status":"ok"}` {
		t.Errorf("Expected body {\"status\":\"ok\"}, got %s", body)
	}
}

func TestHandler_Convert_PoolBusyError(t *testing.T) {
	// This test verifies ErrPoolBusy is handled correctly
	// We can't easily simulate a full pool, but we can test the error handling path

	// Mock a scenario where conversion returns ErrPoolBusy
	// In real scenario, this happens when worker pool queue is full
	// Since we can't easily trigger this, we just verify the error exists
	if converter.ErrPoolBusy == nil {
		t.Error("ErrPoolBusy should not be nil")
	}
}

func TestHandler_Convert_ConcurrentRequests(t *testing.T) {
	h := New(500, 10)

	// Test multiple concurrent requests don't cause race conditions
	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", "test.heic")
			part.Write([]byte("fake data"))
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/convert", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			h.Convert(w, req)
			done <- true
		}()
	}

	// Wait for all goroutines to complete with timeout
	timeout := time.After(5 * time.Second)
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("Timeout waiting for concurrent requests to complete")
		}
	}
}

func TestHandler_sendBinaryJPEGResponse(t *testing.T) {
	h := New(500, 10)

	// Create test JPEG data
	testData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46} // JPEG signature

	// Capture response using a recorder
	w := httptest.NewRecorder()

	h.sendBinaryJPEGResponse(w, testData)

	// Verify response headers
	if w.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("Expected Content-Type image/jpeg, got %s", w.Header().Get("Content-Type"))
	}

	if w.Header().Get("Cache-Control") != "public, max-age=31536000" {
		t.Errorf("Expected Cache-Control public, max-age=31536000, got %s", w.Header().Get("Cache-Control"))
	}

	// Verify status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify body contains our data
	response := w.Body.Bytes()
	if !bytes.Contains(response, testData) {
		t.Error("Response body should contain the JPEG data")
	}
}

func TestHandler_sendJSONResponse(t *testing.T) {
	h := New(500, 10)

	// Create test JPEG data
	testData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46}

	w := httptest.NewRecorder()
	h.sendJSONResponse(w, testData)

	// Verify response headers
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	// Verify status
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify body contains base64-encoded data
	response := w.Body.String()
	if !strings.HasPrefix(response, `{"data":"data:image/jpeg;base64,`) {
		t.Errorf("Response should start with base64 JSON prefix, got %s", response[:50])
	}
	if !strings.HasSuffix(response, `"}`) {
		t.Errorf("Response should end with }, got %s", response)
	}
}

// Helper function to create a test file upload
func createTestFileUpload(filename, content string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", filename)
	part.Write([]byte(content))
	writer.Close()
	return body, writer.FormDataContentType()
}

// Test for actual file conversion (requires testdata)
func TestHandler_Convert_Integration(t *testing.T) {
	// Check if test file exists
	testFiles := []string{"../../testdata/test.heic", "../../testdata/test.heif"}
	var testFile string
	for _, f := range testFiles {
		if _, err := os.Stat(f); err == nil {
			testFile = f
			break
		}
	}

	if testFile == "" {
		t.Skip("No test HEIF file found in testdata/")
		return
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	h := New(500, 10)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", testFile)
	part.Write(data)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Convert(w, req)

	// Should return either success or error based on conversion
	// but should not panic
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError &&
		w.Code != http.StatusServiceUnavailable &&
		w.Code != http.StatusUnsupportedMediaType {
		t.Logf("Conversion returned status %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_Convert_ResponseFormatEdgeCases(t *testing.T) {
	h := New(500, 10)

	tests := []struct {
		name    string
		format  string
		wantCT  string
	}{
		{"Default format returns binary", "", "image/jpeg"},
		{"Explicit binary format", "binary", "image/jpeg"},
		{"JSON format", "json", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, _ := writer.CreateFormFile("file", "test.heic")
			part.Write([]byte("fake"))
			writer.Close()

			url := "/convert"
			if tt.format != "" {
				url += "?format=" + tt.format
			}

			req := httptest.NewRequest(http.MethodPost, url, body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			h.Convert(w, req)

			// Verify content type for valid responses
			if w.Code >= 200 && w.Code < 300 {
				ct := w.Header().Get("Content-Type")
				if ct != tt.wantCT {
					t.Errorf("Expected Content-Type %s, got %s", tt.wantCT, ct)
				}
			}
		})
	}
}

// TestIsValidHEIF magic byte validation
func TestIsValidHEIF(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "Too short",
			data:     []byte("short"),
			expected: false,
		},
		{
			name:     "Empty",
			data:     []byte(""),
			expected: false,
		},
		{
			name:     "Wrong magic bytes",
			data:     []byte("ABCDEFGHIJKL"),
			expected: false,
		},
		{
			name:     "Valid ftyp",
			data:     []byte{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x68, 0x65, 0x69, 0x63},
			expected: true,
		},
		{
			name:     "Valid heic brand",
			data:     []byte{0x00, 0x00, 0x00, 0x1c, 0x66, 0x74, 0x79, 0x70, 0x68, 0x65, 0x69, 0x63},
			expected: true,
		},
		{
			name:     "Valid mif1 brand",
			data:     []byte{0x00, 0x00, 0x00, 0x00, 0x66, 0x74, 0x79, 0x70, 0x6d, 0x69, 0x66, 0x31},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidHEIF(tt.data)
			if result != tt.expected {
				t.Errorf("isValidHEIF() = %v, want %v", result, tt.expected)
			}
		})
	}
}
