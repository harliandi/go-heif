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
