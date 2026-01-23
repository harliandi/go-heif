package converter

import (
	"bytes"
	"errors"
	"image"
	"io"
	"log"

	"github.com/adrium/goheif"
	"github.com/harliandi/go-heif/pkg/quality"
	"image/jpeg"

	turbojpeg "github.com/harliandi/go-heif/pkg/jpeg"
)

var (
	ErrInvalidHEIF = errors.New("invalid HEIF file")
	// UseTurboJPEG enables libjpeg-turbo encoding (3.8x faster)
	// Set to false if libjpeg-turbo is not available
	UseTurboJPEG = true
)

// Converter handles HEIF to JPEG conversion
type Converter struct {
	targetSizeKB int
}

// encodeJPEG encodes an image to JPEG using turbo if available, otherwise stdlib
func encodeJPEG(img image.Image, quality int, out *bytes.Buffer) (err error) {
	// Try turbo encoder for YCbCr images (3.8x faster)
	if UseTurboJPEG {
		if ycbcr, ok := img.(*image.YCbCr); ok {
			// Protect against CGO panics (segfaults from libjpeg-turbo)
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("TurboJPEG panic recovered, falling back to stdlib: %v", r)
						err = errors.New("turbojpeg panic")
					}
				}()
				data, turboErr := turbojpeg.EncodeYCbCr(ycbcr, quality)
				if turboErr == nil {
					_, err = out.Write(data)
				} else {
					err = turboErr
				}
			}()
			if err == nil {
				return nil
			}
			// Fall through to stdlib on error or panic
		}
	}
	// Fallback to stdlib
	return jpeg.Encode(out, img, &jpeg.Options{Quality: quality})
}

// New creates a new Converter with the specified target output size in KB
func New(targetSizeKB int) *Converter {
	return &Converter{
		targetSizeKB: targetSizeKB,
	}
}

// ConvertBytes converts HEIF bytes directly, avoiding extra copy
func (c *Converter) ConvertBytes(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidHEIF
	}

	// Decode HEIF
	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidHEIF
	}

	// Validate image dimensions (protect against decompression bombs)
	if err := ValidateImage(img); err != nil {
		return nil, err
	}

	// Find optimal quality for target size (now just math, super fast)
	q, err := quality.FindOptimalQuality(img, c.targetSizeKB)
	if err != nil {
		q = 85 // Fallback
	}

	// Encode as JPEG with optimized settings
	var out bytes.Buffer
	out.Grow(512 * 1024) // Pre-allocate for ~500KB
	if err := encodeJPEG(img, q, &out); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// Convert converts a HEIF image reader to JPEG bytes
func (c *Converter) Convert(source io.Reader) ([]byte, error) {
	if source == nil {
		return nil, ErrInvalidHEIF
	}

	// Read into buffer (this is now done in handler, but kept for compatibility)
	var buf bytes.Buffer
	buf.Grow(10 * 1024 * 1024) // Pre-allocate 10MB for HEIF files
	if _, err := io.Copy(&buf, source); err != nil {
		return nil, err
	}

	return c.ConvertBytes(buf.Bytes())
}

// ConvertWithFixedQuality converts with a specific quality setting
func (c *Converter) ConvertWithFixedQuality(source io.Reader, q int) ([]byte, error) {
	if source == nil {
		return nil, ErrInvalidHEIF
	}
	if q < 1 || q > 100 {
		q = 85
	}

	// Read into buffer
	var buf bytes.Buffer
	buf.Grow(10 * 1024 * 1024)
	if _, err := io.Copy(&buf, source); err != nil {
		return nil, err
	}
	return c.ConvertBytesWithQuality(buf.Bytes(), q)
}

// ConvertBytesWithQuality converts HEIF bytes with a specific quality
func (c *Converter) ConvertBytesWithQuality(data []byte, q int) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidHEIF
	}
	if q < 1 || q > 100 {
		q = 85
	}

	// Decode HEIF
	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidHEIF
	}

	// Validate image dimensions (protect against decompression bombs)
	if err := ValidateImage(img); err != nil {
		return nil, err
	}

	// Encode as JPEG
	var out bytes.Buffer
	out.Grow(512 * 1024)
	if err := encodeJPEG(img, q, &out); err != nil {
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

// ConvertBytesFast converts HEIF bytes with reduced resolution for speed.
// Scales down by the given factor (e.g., 0.5 for half width/height).
// Uses fixed quality 85 for consistent results and good quality/size balance.
func (c *Converter) ConvertBytesFast(data []byte, scale float64) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidHEIF
	}

	// Decode HEIF
	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidHEIF
	}

	// Validate image dimensions (protect against decompression bombs)
	if err := ValidateImage(img); err != nil {
		return nil, err
	}

	// Downsample image for faster encoding
	scaled := scaleImage(img, scale)

	// Use fixed quality 85 for fast mode - good balance of quality and size
	const fastModeQuality = 85

	// Encode as JPEG
	var out bytes.Buffer
	out.Grow(512 * 1024)
	if err := encodeJPEG(scaled, fastModeQuality, &out); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// ConvertBytesFastWithQuality converts HEIF bytes with reduced resolution and fixed quality.
func (c *Converter) ConvertBytesFastWithQuality(data []byte, scale float64, q int) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidHEIF
	}
	if q < 1 || q > 100 {
		q = 85
	}

	// Decode HEIF
	img, err := goheif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidHEIF
	}

	// Validate image dimensions (protect against decompression bombs)
	if err := ValidateImage(img); err != nil {
		return nil, err
	}

	// Downsample image for faster encoding
	scaled := scaleImage(img, scale)

	// Encode as JPEG
	var out bytes.Buffer
	out.Grow(512 * 1024)
	if err := encodeJPEG(scaled, q, &out); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// scaleImage downscales an image by the given factor (e.g., 0.5 for half size).
// Uses fast nearest-neighbor sampling for maximum speed.
func scaleImage(img image.Image, scale float64) image.Image {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("scaleImage panic recovered: %v", r)
		}
	}()

	if scale >= 1.0 {
		return img // No scaling needed
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// Validate image dimensions
	if srcW <= 0 || srcH <= 0 {
		log.Printf("Invalid image dimensions: %dx%d", srcW, srcH)
		return img
	}

	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)

	// Ensure minimum dimensions
	if dstW < 100 {
		dstW = 100
	}
	if dstH < 100 {
		dstH = 100
	}

	// Use YCbCr for JPEG compatibility
	yimg := image.NewYCbCr(image.Rect(0, 0, dstW, dstH), image.YCbCrSubsampleRatio420)

	// Fast nearest-neighbor scaling
	srcYCbCr, ok := img.(*image.YCbCr)
	if ok {
		scaleYCbCrNearest(srcYCbCr, yimg, srcW, srcH, dstW, dstH)
		return yimg
	}

	// Fallback for other image types
	scaleGenericNearest(img, yimg, srcW, srcH, dstW, dstH)
	return yimg
}

// scaleYCbCrNearest scales YCbCr images using nearest-neighbor (fastest)
func scaleYCbCrNearest(src, dst *image.YCbCr, srcW, srcH, dstW, dstH int) {
	xRatio := (srcW << 16) / dstW
	yRatio := (srcH << 16) / dstH

	for y := 0; y < dstH; y++ {
		srcY := (y * yRatio) >> 16
		dstYOffset := y * dst.YStride
		srcYOffset := srcY * src.YStride

		for x := 0; x < dstW; x++ {
			srcX := (x * xRatio) >> 16
			dst.Y[dstYOffset+x] = src.Y[srcYOffset+srcX]
		}
	}

	// Scale chroma (Cb and Cr) at half resolution
	cSrcW := srcW / 2
	cSrcH := srcH / 2
	cDstW := dstW / 2
	cDstH := dstH / 2
	cxRatio := (cSrcW << 16) / cDstW
	cyRatio := (cSrcH << 16) / cDstH

	for y := 0; y < cDstH; y++ {
		srcY := (y * cyRatio) >> 16
		dstCOffset := y * dst.CStride
		srcCOffset := srcY * src.CStride

		for x := 0; x < cDstW; x++ {
			srcX := (x * cxRatio) >> 16
			dst.Cb[dstCOffset+x] = src.Cb[srcCOffset+srcX]
			dst.Cr[dstCOffset+x] = src.Cr[srcCOffset+srcX]
		}
	}
}

// scaleGenericNearest scales any image type using nearest-neighbor
func scaleGenericNearest(src image.Image, dst *image.YCbCr, srcW, srcH, dstW, dstH int) {
	xRatio := (srcW << 16) / dstW
	yRatio := (srcH << 16) / dstH

	for y := 0; y < dstH; y++ {
		srcY := (y * yRatio) >> 16
		dstYOffset := y * dst.YStride

		for x := 0; x < dstW; x++ {
			srcX := (x * xRatio) >> 16
			r, g, b, _ := src.At(srcX, srcY).RGBA()
			// Convert RGB to YCbCr (simplified)
			yVal := (19595*r + 38470*g + 7471*b + 1<<15) >> 24
			dst.Y[dstYOffset+x] = uint8(yVal)
		}
	}

	// Set neutral chroma for simplicity
	for i := range dst.Cb {
		dst.Cb[i] = 128
		dst.Cr[i] = 128
	}
}
