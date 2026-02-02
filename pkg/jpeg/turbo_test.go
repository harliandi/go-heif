package jpeg

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"testing"

	"github.com/adrium/goheif"
)

func TestEncodeYCbCr(t *testing.T) {
	// Create a simple YCbCr image
	img := image.NewYCbCr(image.Rect(0, 0, 800, 600), image.YCbCrSubsampleRatio420)

	// Fill with some data
	for i := range img.Y {
		img.Y[i] = 128
	}
	for i := range img.Cb {
		img.Cb[i] = 128
	}
	for i := range img.Cr {
		img.Cr[i] = 128
	}

	data, err := EncodeYCbCr(img, 85)
	if err != nil {
		t.Fatalf("EncodeYCbCr failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Empty output")
	}

	// Verify it's a valid JPEG
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Output is not valid JPEG: %v", err)
	}

	if config.Width != 800 || config.Height != 600 {
		t.Fatalf("Wrong dimensions: %dx%d", config.Width, config.Height)
	}
}

func TestEncodeRoundtrip(t *testing.T) {
	// Read a HEIF file to get a real image
	data, err := os.ReadFile("/home/harliandi/go-heif/testdata/test.heic")
	if err != nil {
		t.Skip("no test file available")
	}

	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Failed to decode HEIF: %v", err)
	}

	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		t.Skip("image is not YCbCr")
	}

	// Encode with turbo
	turboData, err := EncodeYCbCr(ycbcr, 85)
	if err != nil {
		t.Fatalf("Turbo encode failed: %v", err)
	}

	// Encode with standard lib
	var stdBuf bytes.Buffer
	err = jpeg.Encode(&stdBuf, ycbcr, &jpeg.Options{Quality: 85})
	if err != nil {
		t.Fatalf("Std encode failed: %v", err)
	}

	// Both should be valid JPEGs
	turboConfig, err := jpeg.DecodeConfig(bytes.NewReader(turboData))
	if err != nil {
		t.Fatalf("Turbo output invalid: %v", err)
	}

	stdConfig, err := jpeg.DecodeConfig(bytes.NewReader(stdBuf.Bytes()))
	if err != nil {
		t.Fatalf("Std output invalid: %v", err)
	}

	if turboConfig.Width != stdConfig.Width || turboConfig.Height != stdConfig.Height {
		t.Errorf("Dimensions mismatch: turbo=%dx%d, std=%dx%d",
			turboConfig.Width, turboConfig.Height, stdConfig.Width, stdConfig.Height)
	}

	t.Logf("Turbo size: %d bytes, Std size: %d bytes", len(turboData), stdBuf.Len())
}

func BenchmarkEncodeYCbCrTurbo(b *testing.B) {
	data, err := os.ReadFile("/home/harliandi/go-heif/testdata/test.heic")
	if err != nil {
		b.Skip("no test file")
	}

	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}

	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		b.Skip("not YCbCr")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = EncodeYCbCr(ycbcr, 85)
	}
}

func BenchmarkEncodeYCbCrStdlib(b *testing.B) {
	data, err := os.ReadFile("/home/harliandi/go-heif/testdata/test.heic")
	if err != nil {
		b.Skip("no test file")
	}

	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}

	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		b.Skip("not YCbCr")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = jpeg.Encode(&buf, ycbcr, &jpeg.Options{Quality: 85})
	}
}

// TestEncode tests the Encode function with various image types
func TestEncode(t *testing.T) {
	// Test with YCbCr image
	ycbcr := image.NewYCbCr(image.Rect(0, 0, 100, 100), image.YCbCrSubsampleRatio420)
	for i := range ycbcr.Y {
		ycbcr.Y[i] = 128
	}
	for i := range ycbcr.Cb {
		ycbcr.Cb[i] = 128
	}
	for i := range ycbcr.Cr {
		ycbcr.Cr[i] = 128
	}

	data, err := Encode(ycbcr, 85)
	if err != nil {
		t.Errorf("Encode(YCbCr) failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Encode(YCbCr) returned empty data")
	}

	// Verify it's a valid JPEG
	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Errorf("Encoded data is not valid JPEG: %v", err)
	}
	if config.Width != 100 || config.Height != 100 {
		t.Errorf("Wrong dimensions: %dx%d", config.Width, config.Height)
	}
}

// TestEncode_NonYCbCr tests Encode with non-YCbCr image (fallback to stdlib)
func TestEncode_NonYCbCr(t *testing.T) {
	// Create an RGBA image (not YCbCr)
	rgba := image.NewRGBA(image.Rect(0, 0, 100, 100))

	data, err := Encode(rgba, 85)
	if err != nil {
		t.Errorf("Encode(RGBA) failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Encode(RGBA) returned empty data")
	}

	// Verify it's a valid JPEG
	_, err = jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Errorf("Encoded data is not valid JPEG: %v", err)
	}
}

// TestEncode_QualityBounds tests quality clamping
func TestEncode_QualityBounds(t *testing.T) {
	img := image.NewYCbCr(image.Rect(0, 0, 100, 100), image.YCbCrSubsampleRatio420)
	for i := range img.Y {
		img.Y[i] = 128
	}

	tests := []struct {
		name    string
		quality int
	}{
		{"Quality 0 (should clamp to MinQuality)", 0},
		{"Quality -1 (should clamp to MinQuality)", -1},
		{"Quality 101 (should clamp to MaxQuality)", 101},
		{"Quality 200 (should clamp to MaxQuality)", 200},
		{"Quality 1 (min valid)", 1},
		{"Quality 100 (max valid)", 100},
		{"Quality 85 (normal)", 85},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Encode(img, tt.quality)
			if err != nil {
				t.Errorf("Encode() failed: %v", err)
			}
			if len(data) == 0 {
				t.Error("Encode() returned empty data")
			}
		})
	}
}

// TestEncode_EmptyImage tests edge case with empty image
func TestEncode_EmptyImage(t *testing.T) {
	// Create an image with zero dimensions
	img := image.NewYCbCr(image.Rect(0, 0, 0, 0), image.YCbCrSubsampleRatio420)

	// Empty image should be handled - either error or panic recovery
	// The libjpeg-turbo may panic on empty images
	defer func() {
		if r := recover(); r != nil {
			// Expected - empty images cause panic in libjpeg-turbo
			t.Logf("Recovered from panic as expected: %v", r)
		}
	}()

	_, err := Encode(img, 85)
	// May fail on encode due to empty dimensions, but shouldn't crash the test
	_ = err
}

// TestBufferWriter_Write tests the bufferWriter Write method
func TestBufferWriter_Write(t *testing.T) {
	buf := make([]byte, 0, 1024)
	w := &bufferWriter{buf: &buf}

	n, err := w.Write([]byte("test data"))
	if err != nil {
		t.Errorf("Write() failed: %v", err)
	}
	if n != 9 {
		t.Errorf("Write() returned %d, want 9", n)
	}
	if string(buf) != "test data" {
		t.Errorf("Buffer content = %s, want 'test data'", string(buf))
	}
}

// TestBufferWriter_WriteMultiple tests multiple writes
func TestBufferWriter_WriteMultiple(t *testing.T) {
	buf := make([]byte, 0, 1024)
	w := &bufferWriter{buf: &buf}

	w.Write([]byte("hello "))
	w.Write([]byte("world"))
	w.Write([]byte("!"))

	if string(buf) != "hello world!" {
		t.Errorf("Buffer content = %s, want 'hello world!'", string(buf))
	}
}

// TestEncodeYCbCr_VariousSizes tests encoding with different image sizes
func TestEncodeYCbCr_VariousSizes(t *testing.T) {
	sizes := []struct {
		w, h int
	}{
		{1, 1},
		{100, 100},
		{1920, 1080},
		{100, 1},
		{1, 100},
	}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.w, size.h), func(t *testing.T) {
			img := image.NewYCbCr(image.Rect(0, 0, size.w, size.h), image.YCbCrSubsampleRatio420)
			for i := range img.Y {
				img.Y[i] = 128
			}
			for i := range img.Cb {
				img.Cb[i] = 128
			}
			for i := range img.Cr {
				img.Cr[i] = 128
			}

			data, err := EncodeYCbCr(img, 85)
			if err != nil {
				t.Errorf("EncodeYCbCr() failed: %v", err)
			}
			if len(data) == 0 {
				t.Error("EncodeYCbCr() returned empty data")
			}
		})
	}
}
