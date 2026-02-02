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

	webp "github.com/chai2010/webp"
)

var (
	ErrInvalidHEIF = errors.New("invalid HEIF file")
	// UseTurboJPEG is kept for backward compatibility but currently unused
	UseTurboJPEG = false
)

// Converter handles HEIF to JPEG/WebP conversion
type Converter struct {
	targetSizeKB int
	outputFormat string // "jpeg" or "webp"
}

// SetOutputFormat sets the output format ("jpeg" or "webp")
func (c *Converter) SetOutputFormat(format string) {
	c.outputFormat = format
}

// encodeImage encodes an image to JPEG or WebP based on outputFormat
func encodeImage(img image.Image, quality int, format string, out *bytes.Buffer) error {
	if format == "webp" {
		// Convert to RGBA first for WebP encoding
		var rgba *image.RGBA
		if src, ok := img.(*image.RGBA); ok {
			rgba = src
		} else {
			rgba = image.NewRGBA(img.Bounds())
			for y := rgba.Rect.Min.Y; y < rgba.Rect.Max.Y; y++ {
				for x := rgba.Rect.Min.X; x < rgba.Rect.Max.X; x++ {
					rgba.Set(x, y, img.At(x, y))
				}
			}
		}
		return webp.Encode(out, rgba, &webp.Options{Quality: float32(quality)})
	}
	// Default to JPEG
	return jpeg.Encode(out, img, &jpeg.Options{Quality: quality})
}

// New creates a new Converter with the specified target output size in KB
func New(targetSizeKB int) *Converter {
	return &Converter{
		targetSizeKB: targetSizeKB,
		outputFormat: "jpeg", // default to JPEG
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
	if err := encodeImage(img, q, c.outputFormat, &out); err != nil {
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
	if err := encodeImage(img, q, c.outputFormat, &out); err != nil {
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
	if err := encodeImage(scaled, fastModeQuality, c.outputFormat, &out); err != nil {
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
	if err := encodeImage(scaled, q, c.outputFormat, &out); err != nil {
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

	// IMPORTANT: YCbCr images with 4:2:0 subsampling already have Cb/Cr planes at half resolution!
	// We must use the actual chroma dimensions from the image structures, not recalculate them.
	// Scale chroma (Cb and Cr) at half resolution
	cSrcW := src.Rect.Dx() / 2
	cSrcH := src.Rect.Dy() / 2
	cDstW := dst.Rect.Dx() / 2
	cDstH := dst.Rect.Dy() / 2
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
// Properly converts RGB to YCbCr to preserve colors
func scaleGenericNearest(src image.Image, dst *image.YCbCr, srcW, srcH, dstW, dstH int) {
	xRatio := (srcW << 16) / dstW
	yRatio := (srcH << 16) / dstH

	// First, collect all RGB samples for chroma calculation
	type rgbSample struct {
		r, g, b uint32
	}
	// We'll compute chroma at 2x2 blocks (420 subsampling)
	cDstW := dstW / 2
	cDstH := dstH / 2

	for y := 0; y < dstH; y++ {
		srcY := (y * yRatio) >> 16
		dstYOffset := y * dst.YStride

		for x := 0; x < dstW; x++ {
			srcX := (x * xRatio) >> 16
			r, g, b, _ := src.At(srcX, srcY).RGBA()
			// RGBA returns pre-multiplied values in range [0, 65535]
			// Convert RGB to YCbCr using standard JPEG conversion formulas
			// Y = 0.299*R + 0.587*G + 0.114*B
			// Cb = -0.1687*R - 0.3313*G + 0.5*B + 128
			// Cr = 0.5*R - 0.4187*G - 0.0813*B + 128

			// Use fixed-point arithmetic for performance
			// Y: (19595*R + 38470*G + 7471*B + 1<<15) >> 24 (but RGBA is 16-bit, not 8-bit)
			// For 16-bit input: (19595*R + 38470*G + 7471*B + 1<<23) >> 24
			r8 := r >> 8
			g8 := g >> 8
			b8 := b >> 8

			yVal := (19595*r8 + 38470*g8 + 7471*b8 + 1<<15) >> 16
			dst.Y[dstYOffset+x] = uint8(yVal)
		}
	}

	// Compute chroma (Cb, Cr) for each 2x2 block
	for cy := 0; cy < cDstH; cy++ {
		for cx := 0; cx < cDstW; cx++ {
			// Sample the center of the 2x2 block in the source
			srcX := ((cx * 2) * xRatio) >> 16
			srcY := ((cy * 2) * yRatio) >> 16
			r, g, b, _ := src.At(srcX, srcY).RGBA()
			r8 := r >> 8
			g8 := g >> 8
			b8 := b >> 8

			// Cb = -0.1687*R - 0.3313*G + 0.5*B + 128
			cbVal := (int32(32768*b8) - int32(11056*r8) - int32(21712*g8))>>16 + 128
			// Cr = 0.5*R - 0.4187*G - 0.0813*B + 128
			crVal := (int32(32768*r8) - int32(27440*g8) - int32(5328*b8))>>16 + 128

			dst.Cb[cy*dst.CStride+cx] = uint8(cbVal)
			dst.Cr[cy*dst.CStride+cx] = uint8(crVal)
		}
	}
}
