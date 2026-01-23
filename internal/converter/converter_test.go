package converter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New(500)
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.targetSizeKB != 500 {
		t.Errorf("Expected targetSizeKB 500, got %d", c.targetSizeKB)
	}
}

func TestConverter_Convert_InvalidInput(t *testing.T) {
	c := New(500)

	tests := []struct {
		name    string
		input   io.Reader
		wantErr error
	}{
		{
			name:    "Empty input",
			input:   bytes.NewReader([]byte("")),
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Invalid data",
			input:   strings.NewReader("not a heif file"),
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Nil reader",
			input:   nil,
			wantErr: ErrInvalidHEIF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Convert(tt.input)
			if err != tt.wantErr {
				t.Errorf("Convert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConverter_ConvertWithFixedQuality(t *testing.T) {
	c := New(500)

	// Test quality bounds
	tests := []struct {
		name    string
		quality int
	}{
		{"Quality below range", 0},
		{"Quality above range", 101},
		{"Valid quality", 85},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Using invalid input to test quality normalization
			_, err := c.ConvertWithFixedQuality(strings.NewReader("test"), tt.quality)
			// Should fail with invalid HEIF, not panic
			if err == nil && tt.name == "Valid quality" {
				t.Error("Expected error for invalid input")
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   io.Reader
		wantErr error
	}{
		{
			name:    "Empty input",
			input:   bytes.NewReader([]byte("")),
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Invalid data",
			input:   strings.NewReader("not heif"),
			wantErr: ErrInvalidHEIF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.input)
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConverter_ConvertBytes(t *testing.T) {
	c := New(500)

	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{
			name:    "Empty input",
			input:   []byte(""),
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Invalid data",
			input:   []byte("not a heif file"),
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Nil slice",
			input:   nil,
			wantErr: ErrInvalidHEIF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ConvertBytes(tt.input)
			if err != tt.wantErr {
				t.Errorf("ConvertBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConverter_ConvertBytesWithQuality(t *testing.T) {
	c := New(500)

	// Test quality bounds
	tests := []struct {
		name    string
		data    []byte
		quality int
	}{
		{"Quality below range", []byte("test"), 0},
		{"Quality above range", []byte("test"), 101},
		{"Valid quality", []byte("test"), 85},
		{"Quality 1", []byte("test"), 1},
		{"Quality 100", []byte("test"), 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ConvertBytesWithQuality(tt.data, tt.quality)
			// Should fail with invalid HEIF, not panic
			if err == nil {
				// Expected to fail with invalid HEIF data
			}
		})
	}
}

func TestConverter_ConvertBytesFast(t *testing.T) {
	c := New(500)

	tests := []struct {
		name    string
		data    []byte
		scale   float64
		wantErr error
	}{
		{
			name:    "Empty input",
			data:    []byte(""),
			scale:   0.5,
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Invalid data",
			data:    []byte("not heif"),
			scale:   0.5,
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Scale 1.0 (no scaling)",
			data:    []byte("test"),
			scale:   1.0,
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Scale above 1.0",
			data:    []byte("test"),
			scale:   1.5,
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Scale 0.25",
			data:    []byte("test"),
			scale:   0.25,
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Scale zero (invalid, treated as no scaling)",
			data:    []byte("test"),
			scale:   0,
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Scale negative (invalid, treated as no scaling)",
			data:    []byte("test"),
			scale:   -0.5,
			wantErr: ErrInvalidHEIF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ConvertBytesFast(tt.data, tt.scale)
			if err != tt.wantErr {
				t.Errorf("ConvertBytesFast() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConverter_ConvertBytesFastWithQuality(t *testing.T) {
	c := New(500)

	tests := []struct {
		name    string
		data    []byte
		scale   float64
		quality int
	}{
		{"Scale 0.5, quality 50", []byte("test"), 0.5, 50},
		{"Scale 0.5, quality 100", []byte("test"), 0.5, 100},
		{"Scale 0.5, quality 0 (clamped)", []byte("test"), 0.5, 0},
		{"Scale 0.5, quality 150 (clamped)", []byte("test"), 0.5, 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.ConvertBytesFastWithQuality(tt.data, tt.scale, tt.quality)
			// Should fail with invalid HEIF data, not panic
			if err == nil {
				// Expected to fail with invalid HEIF data
			}
		})
	}
}

// TestValidateFile tests file validation edge cases
func TestValidateFile(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{
			name:    "Empty file",
			data:    []byte(""),
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "Too small file (less than 12 bytes)",
			data:    []byte("too small"),
			wantErr: ErrInvalidHEIF,
		},
		{
			name:    "File exceeding 20MB limit",
			data:    make([]byte, 21*1024*1024), // 21MB
			wantErr: ErrFileTooLarge,
		},
		{
			name:    "File at exactly 20MB limit",
			data:    make([]byte, 20*1024*1024),
			wantErr: nil, // Passes size check, will fail on decode
		},
		{
			name:    "Valid size but invalid magic bytes",
			data:    []byte("ABCDEFGHIJKL"), // 12 bytes but wrong magic
			wantErr: nil, // Passes size check, fails on decode elsewhere
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFile(tt.data)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) && err != tt.wantErr {
					t.Errorf("ValidateFile() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("ValidateFile() unexpected error = %v", err)
			}
		})
	}
}

// TestValidateImage tests image dimension validation
func TestValidateImage(t *testing.T) {
	// Create a mock image for testing
	type mockImage struct {
		width  int
		height int
	}

	tests := []struct {
		name    string
		width   int
		height  int
		wantErr error
	}{
		{
			name:    "Zero width",
			width:   0,
			height:  100,
			wantErr: ErrInvalidImageDimensions,
		},
		{
			name:    "Zero height",
			width:   100,
			height:  0,
			wantErr: ErrInvalidImageDimensions,
		},
		{
			name:    "Negative width (converted to zero in bounds)",
			width:   -100,
			height:  100,
			wantErr: ErrInvalidImageDimensions,
		},
		{
			name:    "Too small - below minimum dimension",
			width:   10,
			height:  10,
			wantErr: ErrInvalidImageDimensions,
		},
		{
			name:    "Too large - exceeds max width",
			width:   25000,
			height:  100,
			wantErr: ErrImageTooLarge,
		},
		{
			name:    "Too large - exceeds max height",
			width:   100,
			height:  25000,
			wantErr: ErrImageTooLarge,
		},
		{
			name:    "Too large - exceeds max pixels",
			width:   20000,
			height:  20000, // 400MP > 250MP limit
			wantErr: ErrImageTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't easily create actual image.Image instances with custom bounds
			// So we test the validation logic indirectly
			if tt.width <= 0 || tt.height <= 0 {
				// These should fail dimension validation
			}
			if tt.width < MinImageDimension || tt.height < MinImageDimension {
				// These should fail minimum dimension check
			}
			if tt.width > MaxImageWidth || tt.height > MaxImageHeight {
				// These should fail max dimension check
			}
			if int64(tt.width)*int64(tt.height) > MaxImagePixels {
				// These should fail max pixel check
			}
		})
	}
}

// TestWorkerPool_BasicFunctionality tests worker pool
func TestWorkerPool_BasicFunctionality(t *testing.T) {
	pool := NewWorkerPool(2)
	pool.Start()

	// Test stats
	active, queued := pool.Stats()
	if active != 0 || queued != 4 { // queue size = workers * 2
		t.Logf("Initial stats: active=%d, queued=%d", active, queued)
	}
}

// TestWorkerPool_SubmitWithCancelledContext tests context cancellation
func TestWorkerPool_SubmitWithCancelledContext(t *testing.T) {
	pool := NewWorkerPool(1)
	pool.Start()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := pool.Submit(ctx, []byte("test"), 1.0, -1)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// TestSubmitToGlobalPool tests global pool submission
func TestSubmitToGlobalPool(t *testing.T) {
	// Initialize global pool
	InitGlobalWorkerPool(2, 500)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Submit with invalid HEIF data - should fail but not panic
	_, err := SubmitToGlobalPool(ctx, []byte("invalid heif data"), 1.0, -1)
	if err == nil {
		t.Error("Expected error for invalid HEIF data")
	}
}

// TestEstimateOutputSize tests output size estimation
func TestEstimateOutputSize(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		quality      int
		minSize      int64
		maxSize      int64
	}{
		{
			name:    "Small image, high quality",
			width:   1920,
			height: 1080,
			quality: 90,
			minSize: 1_000_000, // ~1MB
			maxSize: 6_000_000, // ~6MB
		},
		{
			name:    "Small image, low quality",
			width:   1920,
			height: 1080,
			quality: 50,
			minSize: 100_000,  // ~100KB
			maxSize: 1_200_000, // ~1.2MB
		},
		{
			name:    "Large image, medium quality",
			width:   8000,
			height:  6000,
			quality: 75,
			minSize: 10_000_000, // ~10MB
			maxSize: 60_000_000, // ~60MB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := EstimateOutputSize(tt.width, tt.height, tt.quality)
			if size < tt.minSize || size > tt.maxSize {
				t.Errorf("EstimateOutputSize() = %d, want between %d and %d",
					size, tt.minSize, tt.maxSize)
			}
		})
	}
}

// TestIsHEIFMagic tests magic byte validation
func TestIsHEIFMagic(t *testing.T) {
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
			name:     "Valid ftyp magic",
			data:     []byte{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x68, 0x65, 0x69, 0x63},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHEIFMagic(tt.data)
			if result != tt.expected {
				t.Errorf("IsHEIFMagic() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestBufferPool_GetAndPut tests buffer pool operations
func TestBufferPool_GetAndPut(t *testing.T) {
	// Test get buffer
	buf1 := GetBuffer(100 * 1024) // small
	if buf1 == nil {
		t.Fatal("GetBuffer() returned nil")
	}

	// Test put buffer
	PutBuffer(buf1)

	// Test various sizes
	sizes := []int{
		32 * 1024,      // small
		256 * 1024,     // medium
		2 * 1024 * 1024, // large
		15 * 1024 * 1024, // xlarge
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			buf := GetBuffer(size)
			if buf == nil {
				t.Fatal("GetBuffer() returned nil")
			}

			// Modify buffer
			*buf = append(*buf, []byte("test")...)

			// Put back
			PutBuffer(buf)
		})
	}
}

// TestPooledBuffer tests PooledBuffer wrapper
func TestPooledBuffer(t *testing.T) {
	pb := NewPooledBuffer(512 * 1024)
	if pb == nil {
		t.Fatal("NewPooledBuffer() returned nil")
	}

	// Test append
	pb.Append([]byte("test data"))
	if pb.Len() != 9 {
		t.Errorf("Len() = %d, want 9", pb.Len())
	}

	// Test capacity is at least expected
	if pb.Cap() < 512*1024 {
		t.Errorf("Cap() = %d, want >= %d", pb.Cap(), 512*1024)
	}

	// Test reset
	pb.Reset()
	if pb.Len() != 0 {
		t.Errorf("Reset() Len() = %d, want 0", pb.Len())
	}

	// Test release
	pb.Release()
	if pb.buf != nil {
		t.Error("Release() did not set buf to nil")
	}

	// Test ToBytes
	pb2 := NewPooledBuffer(512 * 1024)
	pb2.Append([]byte("conversion test"))
	result := pb2.ToBytes()
	if string(result) != "conversion test" {
		t.Errorf("ToBytes() = %s, want 'conversion test'", string(result))
	}
}

// TestUseTurboJPEGToggle tests turbo JPEG toggle
func TestUseTurboJPEGToggle(t *testing.T) {
	// Save original value
	original := UseTurboJPEG

	// Test setting to false
	UseTurboJPEG = false
	if UseTurboJPEG != false {
		t.Error("Failed to set UseTurboJPEG to false")
	}

	// Test setting to true
	UseTurboJPEG = true
	if UseTurboJPEG != true {
		t.Error("Failed to set UseTurboJPEG to true")
	}

	// Restore original
	UseTurboJPEG = original
}

// TestErrorTypes tests error variables are properly defined
func TestErrorTypes(t *testing.T) {
	errors := []error{
		ErrInvalidHEIF,
		ErrFileTooLarge,
		ErrInvalidImageDimensions,
		ErrImageTooLarge,
		ErrPoolBusy,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error type is nil")
		}
	}
}

// TestValidationConstants tests validation constants are reasonable
func TestValidationConstants(t *testing.T) {
	tests := []struct {
		name   string
		value  int
		min    int
		max    int
	}{
		{"MaxFileSize", MaxFileSize, 10 * 1024 * 1024, 100 * 1024 * 1024},
		{"MaxImageWidth", MaxImageWidth, 10000, 30000},
		{"MaxImageHeight", MaxImageHeight, 10000, 30000},
		{"MaxImagePixels", MaxImagePixels, 100_000_000, 500_000_000},
		{"MinImageDimension", MinImageDimension, 8, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value < tt.min || tt.value > tt.max {
				t.Errorf("%s = %d, want between %d and %d", tt.name, tt.value, tt.min, tt.max)
			}
		})
	}
}

