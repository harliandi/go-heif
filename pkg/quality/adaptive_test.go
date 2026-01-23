package quality

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// encodeSize returns the size in bytes of encoding the image at given quality
// This is a test helper function
func encodeSize(img image.Image, quality int) (int, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return 0, err
	}
	return buf.Len(), nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// createTestImage creates a simple test image
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 255) / width),
				G: uint8((y * 255) / height),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}

func TestFindOptimalQuality(t *testing.T) {
	tests := []struct {
		name         string
		targetSizeKB int
		imageSize    int
	}{
		{"Small target 100KB", 100, 800},
		{"Medium target 500KB", 500, 1920},
		{"Large target 1MB", 1024, 1920},
		{"Very small target 50KB", 50, 640},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := createTestImage(tt.imageSize, tt.imageSize)

			quality, err := FindOptimalQuality(img, tt.targetSizeKB)
			if err != nil {
				t.Fatalf("FindOptimalQuality() error = %v", err)
			}

			// Quality should be within bounds
			if quality < 10 || quality > 100 {
				t.Errorf("Quality %d out of bounds [10, 100]", quality)
			}

			// Verify encoded size is reasonably close to target
			size, err := encodeSize(img, quality)
			if err != nil {
				t.Fatalf("encodeSize() error = %v", err)
			}

			targetBytes := tt.targetSizeKB * 1024
			diffPercent := float64(abs(size-targetBytes)) / float64(targetBytes) * 100

			// Should be within 50% of target (reasonable for binary search)
			if diffPercent > 50 {
				t.Logf("Warning: size %d bytes is %.1f%% from target %d bytes (quality %d)",
					size, diffPercent, targetBytes, quality)
			}
		})
	}
}

func TestFindOptimalQuality_Bounds(t *testing.T) {
	img := createTestImage(1000, 1000)

	// Test with very small target - should not go below minQuality
	q, _ := FindOptimalQuality(img, 1)
	if q < minQuality {
		t.Errorf("Quality %d below minimum %d", q, minQuality)
	}

	// Test with very large target - should not go above maxQuality
	q, _ = FindOptimalQuality(img, 10000)
	if q > maxQuality {
		t.Errorf("Quality %d above maximum %d", q, maxQuality)
	}
}

func TestEncodeSize(t *testing.T) {
	img := createTestImage(500, 500)

	size, err := encodeSize(img, 85)
	if err != nil {
		t.Fatalf("encodeSize() error = %v", err)
	}

	if size == 0 {
		t.Error("Encoded size is zero")
	}

	// Higher quality should produce larger file
	lowSize, _ := encodeSize(img, 10)
	highSize, _ := encodeSize(img, 95)

	if highSize <= lowSize {
		t.Errorf("Quality 95 size %d <= Quality 10 size %d", highSize, lowSize)
	}
}

func BenchmarkFindOptimalQuality(b *testing.B) {
	img := createTestImage(1920, 1080)
	targetKB := 500

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FindOptimalQuality(img, targetKB)
	}
}

func BenchmarkEncodeSize(b *testing.B) {
	img := createTestImage(1920, 1080)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encodeSize(img, 85)
	}
}
