package converter

import (
	"bytes"
	"errors"
	"io"

	"github.com/adrium/goheif"
	"github.com/harliandi/go-heif/pkg/quality"
	"image/jpeg"
)

var (
	ErrInvalidHEIF = errors.New("invalid HEIF file")
)

// Converter handles HEIF to JPEG conversion
type Converter struct {
	targetSizeKB int
}

// New creates a new Converter with the specified target output size in KB
func New(targetSizeKB int) *Converter {
	return &Converter{
		targetSizeKB: targetSizeKB,
	}
}

// Convert converts a HEIF image reader to a JPEG writer.
// Returns the JPEG data as a byte slice.
func (c *Converter) Convert(source io.Reader) ([]byte, error) {
	if source == nil {
		return nil, ErrInvalidHEIF
	}
	// Read source into buffer for decoding
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, source); err != nil {
		return nil, err
	}
	data := buf.Bytes()

	// Decode HEIF
	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidHEIF
	}

	// Find optimal quality for target size
	q, err := quality.FindOptimalQuality(img, c.targetSizeKB)
	if err != nil {
		// Fallback to default quality if adaptive fails
		q = 85
	}

	// Encode as JPEG (EXIF is stripped by not passing it)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: q}); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// ConvertWithFixedQuality converts with a specific quality setting (1-100)
// bypassing the adaptive quality algorithm
func (c *Converter) ConvertWithFixedQuality(source io.Reader, quality int) ([]byte, error) {
	if source == nil {
		return nil, ErrInvalidHEIF
	}
	if quality < 1 || quality > 100 {
		quality = 85
	}

	// Read source into buffer for decoding
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, source); err != nil {
		return nil, err
	}
	data := buf.Bytes()

	// Decode HEIF
	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidHEIF
	}

	// Encode as JPEG
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// Validate checks if the reader contains a valid HEIF file
func Validate(source io.Reader) error {
	if source == nil {
		return ErrInvalidHEIF
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, source); err != nil {
		return err
	}
	_, err := goheif.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return ErrInvalidHEIF
	}
	return nil
}
