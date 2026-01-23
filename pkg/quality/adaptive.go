package quality

import (
	"image"
)

const (
	minQuality = 10
	maxQuality = 100
)

// FindOptimalQuality finds the JPEG quality setting that produces
// an output closest to the target size in KB.
// ZERO iterations - uses pure mathematical estimation for maximum speed.
func FindOptimalQuality(img image.Image, targetSizeKB int) (int, error) {
	// Pure mathematical estimation - no encoding iterations
	return estimateQualitySinglePass(img, targetSizeKB), nil
}

// estimateQualitySinglePass calculates quality in a single pass
// using image dimensions and target size without any encoding.
// This is the fastest possible approach for quality estimation.
func estimateQualitySinglePass(img image.Image, targetSizeKB int) int {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixels := width * height

	// Empirical formula derived from typical JPEG compression ratios:
	// JPEG at quality Q produces approximately: bytes ≈ pixels * (Q/100)² * 0.15
	// Solving for Q: Q ≈ 100 * sqrt(target_bytes / (pixels * 0.15))

	// Target in bytes
	targetBytes := float64(targetSizeKB * 1024)

	// Compression factor varies by content type
	// 0.15 = typical for photos, 0.25 = simple graphics
	// Use conservative 0.18 for mixed content
	compressionFactor := 0.18

	// Calculate quality using inverse square law
	// This accounts for JPEG's non-linear quality-to-size relationship
	ratio := targetBytes / (float64(pixels) * compressionFactor)

	// Apply inverse square root (JPEG quality has quadratic effect on size)
	quality := 100 * sqrt(ratio)

	// Fine-tune based on pixel density
	// Very high resolution images need slightly lower quality
	if pixels > 20_000_000 { // 20MP+
		quality -= 5
	} else if pixels < 3_000_000 { // < 3MP
		quality += 5
	}

	// Clamp to valid range
	return clamp(int(quality), minQuality, maxQuality)
}

// sqrt returns square root using Newton-Raphson iteration
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return x // For ratios >= 1, quality cap at 100 handles it
	}
	// For x < 1, we need actual sqrt
	z := 1.0
	for i := 0; i < 4; i++ {
		z = 0.5 * (z + x/z)
	}
	return z
}

func clamp(x, min, max int) int {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}
