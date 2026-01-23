package quality

import (
	"bytes"
	"image"
	"image/jpeg"
)

const (
	minQuality = 10
	maxQuality = 100
	maxIterations = 4
	startQuality = 85
)

// FindOptimalQuality finds the JPEG quality setting that produces
// an output closest to the target size in KB.
// Uses binary search approach with maxIterations to balance speed and accuracy.
func FindOptimalQuality(img image.Image, targetSizeKB int) (int, error) {
	targetBytes := targetSizeKB * 1024

	low, high := minQuality, maxQuality
	bestQuality := startQuality
	bestSizeDiff := -1

	for i := 0; i < maxIterations; i++ {
		mid := (low + high) / 2
		size, err := encodeSize(img, mid)
		if err != nil {
			return startQuality, err
		}

		sizeDiff := abs(size - targetBytes)

		// Track best quality found
		if bestSizeDiff == -1 || sizeDiff < bestSizeDiff {
			bestSizeDiff = sizeDiff
			bestQuality = mid
		}

		// Early exit if we're close enough (within 5% of target)
		if sizeDiff < targetBytes/20 {
			break
		}

		// Binary search adjustment
		if size > targetBytes {
			high = mid - 1
		} else {
			low = mid + 1
		}

		// Prevent infinite loop
		if low > high {
			break
		}
	}

	return bestQuality, nil
}

// encodeSize returns the size in bytes of encoding the image at given quality
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
