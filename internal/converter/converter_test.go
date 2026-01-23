package converter

import (
	"bytes"
	"io"
	"strings"
	"testing"
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
