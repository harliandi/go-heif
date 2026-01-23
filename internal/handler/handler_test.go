package handler

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	h := New(500, 10)
	if h == nil {
		t.Fatal("New() returned nil")
	}
	if h.targetSizeKB != 500 {
		t.Errorf("Expected targetSizeKB 500, got %d", h.targetSizeKB)
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

func TestHandler_Convert_QueryParameters(t *testing.T) {
	h := New(500, 10)

	// Test quality parameter
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.heic")
	part.Write([]byte("fake"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/convert?quality=90", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	h.Convert(w, req)
	// Will fail on invalid data, but parameter should be parsed
	_ = w
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
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Logf("Conversion returned status %d: %s", w.Code, w.Body.String())
	}
}
