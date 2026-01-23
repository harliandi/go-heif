package converter

import (
	"errors"
	"image"
	"log"
)

var (
	// ErrFileTooLarge is returned when the file exceeds the size limit
	ErrFileTooLarge = errors.New("file size exceeds limit")
	// ErrInvalidImageDimensions is returned when image dimensions are invalid
	ErrInvalidImageDimensions = errors.New("invalid image dimensions")
	// ErrImageTooLarge is returned when image dimensions exceed limits
	ErrImageTooLarge = errors.New("image dimensions exceed maximum allowed")
)

// Validation limits
const (
	MaxFileSize        = 20 * 1024 * 1024 // 20MB max file size
	MaxImageWidth      = 20000             // 20K pixels max width
	MaxImageHeight     = 20000             // 20K pixels max height
	MaxImagePixels     = 250_000_000       // 250 megapixels max total pixels
	MinImageDimension  = 16                // Minimum dimension for valid image
)

// ValidateFile checks the file size and basic structure before processing
func ValidateFile(data []byte) error {
	// Check file size
	if len(data) > MaxFileSize {
		log.Printf("File too large: %d bytes (max: %d)", len(data), MaxFileSize)
		return ErrFileTooLarge
	}

	if len(data) < 12 {
		return ErrInvalidHEIF
	}

	return nil
}

// ValidateImage checks decoded image dimensions are within acceptable limits
func ValidateImage(img image.Image) error {
	if img == nil {
		return ErrInvalidHEIF
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Check for zero or negative dimensions
	if width <= 0 || height <= 0 {
		log.Printf("Invalid dimensions: %dx%d", width, height)
		return ErrInvalidImageDimensions
	}

	// Check minimum dimension
	if width < MinImageDimension || height < MinImageDimension {
		log.Printf("Dimensions too small: %dx%d (min: %d)", width, height, MinImageDimension)
		return ErrInvalidImageDimensions
	}

	// Check maximum width/height
	if width > MaxImageWidth || height > MaxImageHeight {
		log.Printf("Dimensions too large: %dx%d (max: %dx%d)", width, height, MaxImageWidth, MaxImageHeight)
		return ErrImageTooLarge
	}

	// Check total pixel count (prevent decompression bomb attacks)
	totalPixels := int64(width) * int64(height)
	if totalPixels > MaxImagePixels {
		log.Printf("Too many pixels: %d (max: %d)", totalPixels, MaxImagePixels)
		return ErrImageTooLarge
	}

	return nil
}

// EstimateOutputSize estimates the JPEG output size based on input dimensions
// This helps detect potential decompression bombs early
func EstimateOutputSize(width, height, quality int) int64 {
	// Rough estimate: worst case JPEG at quality 100 is about 70% of uncompressed
	// Uncompressed RGB = width * height * 3
	// JPEG at 100 quality ~ width * height * 2 (conservative estimate)
	// JPEG at quality 50 ~ width * height * 0.5

	pixels := int64(width) * int64(height)

	// Use a conservative multiplier based on quality
	var multiplier float64
	switch {
	case quality >= 90:
		multiplier = 2.0
	case quality >= 70:
		multiplier = 1.0
	case quality >= 50:
		multiplier = 0.5
	default:
		multiplier = 0.3
	}

	estimatedSize := int64(float64(pixels) * multiplier)
	return estimatedSize
}

// SafeDecode wraps the HEIF decoding with validation
func SafeDecode(data []byte) (image.Image, error) {
	// First validate file size
	if err := ValidateFile(data); err != nil {
		return nil, err
	}

	// Check magic bytes early to avoid decoding obviously invalid files
	if len(data) >= 12 {
		ftyp := string(data[4:8])
		if ftyp != "ftyp" {
			return nil, ErrInvalidHEIF
		}
	}

	// Decode using the goheif library
	// Note: We're importing from the package-level import
	// This function would need access to goheif.Decode
	// For now, this is a placeholder for the validation flow
	return nil, nil
}

// GetImageInfo extracts basic image information without full decoding
func GetImageInfo(data []byte) (width, height int, err error) {
	// This would ideally use a fast metadata extractor
	// For now, we need to decode to get dimensions
	// In Phase 2, we can add a fast HEIF metadata parser
	return 0, 0, errors.New("not implemented")
}

// IsHEIFMagic checks if the data has HEIF magic bytes
func IsHEIFMagic(data []byte) bool {
	if len(data) < 12 {
		return false
	}

	// Check for "ftyp" at offset 4 (ISOBMFF format)
	ftyp := string(data[4:8])
	if ftyp != "ftyp" {
		return false
	}

	return true
}
