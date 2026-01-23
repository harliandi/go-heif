package jpeg

import (
	"bytes"
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
