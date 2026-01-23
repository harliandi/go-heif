package handler

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/harliandi/go-heif/internal/converter"
)

const (
	maxMemory = 32 << 20 // 32MB max in-memory for multipart parsing
	// HEIF magic bytes validation
	// HEIC files start with: 00 00 00 1[8,c] 66 74 79 70 68 65 69 63
	// ISO Base Media File Format (ISOBMFF) starts with ftyp at offset 4
	ftypMagic = "ftyp"
)

// Handler handles HTTP requests for image conversion
type Handler struct {
	converter    *converter.Converter
	maxUploadMB  int
	targetSizeKB int
}

// New creates a new Handler
func New(targetSizeKB, maxUploadMB int) *Handler {
	return &Handler{
		converter:    converter.New(targetSizeKB),
		maxUploadMB:  maxUploadMB,
		targetSizeKB: targetSizeKB,
	}
}

// Convert handles the /convert endpoint
func (h *Handler) Convert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form with size limit
	if err := r.ParseMultipartForm(int64(h.maxUploadMB) << 20); err != nil {
		if errors.Is(err, http.ErrNotMultipart) {
			http.Error(w, "Content-Type must be multipart/form-data", http.StatusBadRequest)
		} else {
			http.Error(w, "Request too large", http.StatusRequestEntityTooLarge)
		}
		return
	}

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension
	if !isHEIFExtension(header.Filename) {
		http.Error(w, "Not a HEIF/HEIC file (wrong extension)", http.StatusUnsupportedMediaType)
		return
	}

	// Validate file magic bytes (actual format check)
	if !isValidHEIF(file) {
		http.Error(w, "Invalid HEIF/HEIC file format", http.StatusUnsupportedMediaType)
		return
	}

	// Reset file reader for conversion
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// Check query parameters
	query := r.URL.Query()

	// Check for fixed quality parameter
	if qualityStr := query.Get("quality"); qualityStr != "" {
		q, err := strconv.Atoi(qualityStr)
		if err == nil && q >= 1 && q <= 100 {
			h.convertWithQuality(w, file, q)
			return
		}
	}

	// Check for custom max_size parameter
	if maxSizeStr := query.Get("max_size"); maxSizeStr != "" {
		sizeKB, err := strconv.Atoi(maxSizeStr)
		if err == nil && sizeKB > 0 {
			h.converter = converter.New(sizeKB)
		}
	}

	// Convert with adaptive quality
	jpegData, err := h.converter.Convert(file)
	if err != nil {
		log.Printf("Conversion error: %v", err)
		http.Error(w, "Conversion failed", http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(jpegData)))
	w.WriteHeader(http.StatusOK)
	w.Write(jpegData)
}

func (h *Handler) convertWithQuality(w http.ResponseWriter, source io.Reader, quality int) {
	// Seek back to start if possible
	if seeker, ok := source.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	jpegData, err := h.converter.ConvertWithFixedQuality(source, quality)
	if err != nil {
		log.Printf("Conversion error: %v", err)
		http.Error(w, "Conversion failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(jpegData)))
	w.WriteHeader(http.StatusOK)
	w.Write(jpegData)
}

// Health handles the /health endpoint for readiness/liveness probes
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// isHEIFExtension checks if the filename has a HEIF/HEIC extension
func isHEIFExtension(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".heif") || strings.HasSuffix(lower, ".heic")
}

// isValidHEIF validates the file magic bytes to confirm it's a HEIF file
// HEIF files follow ISO Base Media File Format (ISOBMFF)
// They start with: [4 bytes size] + "ftyp" + [brand]
func isValidHEIF(r io.Reader) bool {
	// Read first 12 bytes to check for ftyp magic
	header := make([]byte, 12)
	n, err := io.ReadFull(r, header)
	if err != nil || n < 12 {
		return false
	}

	// Check for "ftyp" at offset 4 (ISOBMFF format)
	if len(header) < 8 {
		return false
	}

	ftyp := string(header[4:8])
	if ftyp != ftypMagic {
		return false
	}

	// Check for known HEIF brands (heic, heim, heis, heix, etc.)
	brand := strings.ToLower(string(header[8:12]))
	validBrands := []string{"heic", "heim", "heis", "heix", "mif1"}
	for _, vb := range validBrands {
		if brand == vb {
			return true
		}
	}

	// Also check compatible brand at offset 16 if available
	// For now, ftyp + any brand starting with "he" is good enough
	return strings.HasPrefix(brand, "he") || brand == "mif1"
}
