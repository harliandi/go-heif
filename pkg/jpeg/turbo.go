// Package jpeg provides fast JPEG encoding using libjpeg-turbo via CGO.
// This can be 2-4x faster than the standard library's pure Go encoder.
package jpeg

/*
#cgo pkg-config: libjpeg
#include <stdio.h>
#include <jpeglib.h>
#include <jerror.h>
#include <stdlib.h>
#include <string.h>
#include <setjmp.h>

typedef struct {
    struct jpeg_error_mgr pub;
    jmp_buf setjmp_buffer;
    char msg[JMSG_LENGTH_MAX];
} my_error_mgr;

static void my_error_exit(j_common_ptr cinfo) {
    my_error_mgr *err = (my_error_mgr *)cinfo->err;
    (*cinfo->err->format_message)(cinfo, err->msg);
    longjmp(err->setjmp_buffer, 1);
}

static my_error_mgr* setup_error_mgr(struct jpeg_compress_struct *cinfo) {
    my_error_mgr *err = (my_error_mgr *)malloc(sizeof(my_error_mgr));
    cinfo->err = jpeg_std_error(&err->pub);
    err->pub.error_exit = my_error_exit;
    return err;
}

// Initialize memory destination
static void init_mem_dest(struct jpeg_compress_struct *cinfo,
                          unsigned char **out_buffer,
                          unsigned long *out_size) {
    jpeg_mem_dest(cinfo, out_buffer, out_size);
}

// Encode YCbCr to JPEG
static int encode_ycc(
    const unsigned char *y, int y_stride,
    const unsigned char *cb, int cb_stride,
    const unsigned char *cr, int cr_stride,
    int width, int height, int quality,
    unsigned char **out_buffer, unsigned long *out_size,
    char **error_msg) {

    struct jpeg_compress_struct cinfo;
    my_error_mgr *jerr = NULL;
    JSAMPROW row[1];
    unsigned char *row_buffer = NULL;
    int result = 0;

    *out_buffer = NULL;
    *out_size = 0;
    *error_msg = NULL;

    jerr = setup_error_mgr(&cinfo);
    if (setjmp(jerr->setjmp_buffer)) {
        *error_msg = strdup(jerr->msg);
        result = -1;
        goto cleanup;
    }

    jpeg_create_compress(&cinfo);
    init_mem_dest(&cinfo, out_buffer, out_size);

    cinfo.image_width = width;
    cinfo.image_height = height;
    cinfo.input_components = 3;
    cinfo.in_color_space = JCS_YCbCr;

    jpeg_set_defaults(&cinfo);
    jpeg_set_quality(&cinfo, quality, TRUE);
    cinfo.dct_method = JDCT_FASTEST;

    // 4:2:0 subsampling
    cinfo.comp_info[0].h_samp_factor = 2;
    cinfo.comp_info[0].v_samp_factor = 2;
    cinfo.comp_info[1].h_samp_factor = 1;
    cinfo.comp_info[1].v_samp_factor = 1;
    cinfo.comp_info[2].h_samp_factor = 1;
    cinfo.comp_info[2].v_samp_factor = 1;

    jpeg_start_compress(&cinfo, TRUE);

    // Allocate interleaved row buffer: Y + Cb/2 + Cr/2
    int row_size = width + width;  // Y + Cb/2 + Cr/2
    row_buffer = (unsigned char *)malloc(row_size);

    while (cinfo.next_scanline < cinfo.image_height) {
        int y_row = cinfo.next_scanline;
        int c_row = y_row / 2;
        int c_width = width / 2;

        // Copy Y
        memcpy(row_buffer, y + y_row * y_stride, width);

        // Copy Cb and Cr (interleaved for libjpeg)
        unsigned char *dst = row_buffer + width;
        const unsigned char *cb_src = cb + c_row * cb_stride;
        const unsigned char *cr_src = cr + c_row * cr_stride;

        for (int i = 0; i < c_width; i++) {
            *dst++ = cb_src[i];
            *dst++ = cr_src[i];
        }

        row[0] = row_buffer;
        jpeg_write_scanlines(&cinfo, row, 1);
    }

    jpeg_finish_compress(&cinfo);

cleanup:
    if (row_buffer) free(row_buffer);
    jpeg_destroy_compress(&cinfo);
    if (jerr) free(jerr);
    return result;
}
*/
import "C"
import (
	"fmt"
	"image"
	"image/jpeg"
	"unsafe"
)

const (
	// DefaultQuality is the default JPEG quality
	DefaultQuality = 85
	// MinQuality is the minimum quality
	MinQuality = 1
	// MaxQuality is the maximum quality
	MaxQuality = 100
)

// EncodeYCbCr encodes an image.YCbCr to JPEG using libjpeg-turbo.
func EncodeYCbCr(img *image.YCbCr, quality int) ([]byte, error) {
	if quality < MinQuality {
		quality = MinQuality
	}
	if quality > MaxQuality {
		quality = MaxQuality
	}

	var (
		outBuffer  *C.uchar
		outSize    C.ulong
		errorMsg   *C.char
	)

	result := C.encode_ycc(
		(*C.uchar)(&img.Y[0]),
		C.int(img.YStride),
		(*C.uchar)(&img.Cb[0]),
		C.int(img.CStride),
		(*C.uchar)(&img.Cr[0]),
		C.int(img.CStride),
		C.int(img.Rect.Dx()),
		C.int(img.Rect.Dy()),
		C.int(quality),
		&outBuffer,
		&outSize,
		&errorMsg,
	)

	if result != 0 || outBuffer == nil {
		err := fmt.Errorf("jpeg encode failed")
		if errorMsg != nil {
			err = fmt.Errorf("jpeg encode failed: %s", C.GoString(errorMsg))
			C.free(unsafe.Pointer(errorMsg))
		}
		return nil, err
	}

	// Copy to Go slice
	data := C.GoBytes(unsafe.Pointer(outBuffer), C.int(outSize))
	C.free(unsafe.Pointer(outBuffer))

	return data, nil
}

// Encode encodes any image.Image to JPEG.
// For non-YCbCr images, it falls back to standard library encoding.
func Encode(img image.Image, quality int) ([]byte, error) {
	if ycbcr, ok := img.(*image.YCbCr); ok {
		return EncodeYCbCr(ycbcr, quality)
	}

	// Fallback for other image types
	buf := make([]byte, 0, 512*1024)
	w := &bufferWriter{buf: &buf}
	err := jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	return buf, err
}

type bufferWriter struct {
	buf *[]byte
}

func (w *bufferWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}
