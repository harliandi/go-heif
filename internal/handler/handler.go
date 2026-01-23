package handler

import (
	"encoding/base64"
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
	converter   *converter.Converter
	maxUploadMB int
	useWorkerPool bool
}

// New creates a new Handler
func New(targetSizeKB, maxUploadMB int) *Handler {
	return &Handler{
		converter:   converter.New(targetSizeKB),
		maxUploadMB: maxUploadMB,
		useWorkerPool: true, // Enable worker pool by default for better performance
	}
}

// Convert handles the /convert endpoint
func (h *Handler) Convert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for client cancellation early
	select {
	case <-r.Context().Done():
		log.Printf("Request cancelled by client")
		return
	default:
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

	// Read entire file into memory to avoid seek/reader exhaustion issues
	// This also allows us to validate magic bytes before conversion
	fileData, err := io.ReadAll(file)
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Strict file size validation before processing
	if err := converter.ValidateFile(fileData); err != nil {
		log.Printf("File validation failed: %v", err)
		if errors.Is(err, converter.ErrFileTooLarge) {
			http.Error(w, "File too large (max 20MB)", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Invalid file", http.StatusBadRequest)
		}
		return
	}

	// Validate file magic bytes (actual format check)
	if !isValidHEIF(fileData) {
		http.Error(w, "Invalid HEIF/HEIC file format", http.StatusUnsupportedMediaType)
		return
	}

	// Check query parameters
	query := r.URL.Query()

	// Default scale is 0.5 (50% resolution) for speed
	// Use scale=1 to get full resolution
	var scale float64 = 0.5
	if scaleStr := query.Get("scale"); scaleStr != "" {
		s, err := strconv.ParseFloat(scaleStr, 64)
		if err == nil && s > 0 {
			scale = s
		}
	}

	// Get quality parameter (use adaptive quality by default)
	var quality int = -1
	if qualityStr := query.Get("quality"); qualityStr != "" {
		q, err := strconv.Atoi(qualityStr)
		if err == nil && q >= 1 && q <= 100 {
			quality = q
		}
	}

	// Convert with scale and/or quality
	if scale > 0 && scale < 1.0 {
		if quality > 0 {
			h.convertFastWithQuality(w, r, fileData, scale, quality)
		} else {
			h.convertFast(w, r, fileData, scale)
		}
		return
	}

	// scale >= 1.0 means full resolution
	if quality > 0 {
		h.convertWithQuality(w, r, fileData, quality)
		return
	}

	// Check for custom max_size parameter - worker pool doesn't support dynamic target size
	// For custom max_size, fall back to direct conversion
	var jpegData []byte
	if maxSizeStr := query.Get("max_size"); maxSizeStr != "" {
		// Custom target size - use direct conversion (worker pool uses default target size)
		sizeKB, parseErr := strconv.Atoi(maxSizeStr)
		if parseErr == nil && sizeKB > 0 {
			conv := converter.New(sizeKB)
			jpegData, err = conv.ConvertBytes(fileData)
		} else {
			jpegData, err = h.converter.ConvertBytes(fileData)
		}
	} else if h.useWorkerPool {
		// Use worker pool with context for cancellation support
		jpegData, err = converter.SubmitToGlobalPool(r.Context(), fileData, 1.0, -1)
	} else {
		// Direct conversion fallback
		jpegData, err = h.converter.ConvertBytes(fileData)
	}

	if err != nil {
		log.Printf("Conversion error: %v", err)
		if errors.Is(err, converter.ErrPoolBusy) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"Service busy, please retry"}`))
			return
		}
		http.Error(w, "Conversion failed", http.StatusInternalServerError)
		return
	}

	// Check response format - default to binary for better performance
	format := query.Get("format")
	if format == "json" {
		// Legacy base64 JSON response
		h.sendJSONResponse(w, jpegData)
	} else {
		// Default: stream raw JPEG binary (more efficient)
		h.sendBinaryJPEGResponse(w, jpegData)
	}
}

func (h *Handler) convertWithQuality(w http.ResponseWriter, r *http.Request, fileData []byte, quality int) {
	var jpegData []byte
	var err error
	if h.useWorkerPool {
		jpegData, err = converter.SubmitToGlobalPool(r.Context(), fileData, 1.0, quality)
	} else {
		jpegData, err = h.converter.ConvertBytesWithQuality(fileData, quality)
	}
	if err != nil {
		log.Printf("Conversion error: %v", err)
		if errors.Is(err, converter.ErrPoolBusy) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"Service busy, please retry"}`))
			return
		}
		http.Error(w, "Conversion failed", http.StatusInternalServerError)
		return
	}
	h.sendJPEGResponse(w, r, jpegData)
}

func (h *Handler) convertFast(w http.ResponseWriter, r *http.Request, fileData []byte, scale float64) {
	var jpegData []byte
	var err error
	if h.useWorkerPool {
		jpegData, err = converter.SubmitToGlobalPool(r.Context(), fileData, scale, -1)
	} else {
		jpegData, err = h.converter.ConvertBytesFast(fileData, scale)
	}
	if err != nil {
		log.Printf("Conversion error: %v", err)
		if errors.Is(err, converter.ErrPoolBusy) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"Service busy, please retry"}`))
			return
		}
		http.Error(w, "Conversion failed", http.StatusInternalServerError)
		return
	}
	h.sendJPEGResponse(w, r, jpegData)
}

func (h *Handler) convertFastWithQuality(w http.ResponseWriter, r *http.Request, fileData []byte, scale float64, quality int) {
	var jpegData []byte
	var err error
	if h.useWorkerPool {
		jpegData, err = converter.SubmitToGlobalPool(r.Context(), fileData, scale, quality)
	} else {
		jpegData, err = h.converter.ConvertBytesFastWithQuality(fileData, scale, quality)
	}
	if err != nil {
		log.Printf("Conversion error: %v", err)
		if errors.Is(err, converter.ErrPoolBusy) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"Service busy, please retry"}`))
			return
		}
		http.Error(w, "Conversion failed", http.StatusInternalServerError)
		return
	}
	h.sendJPEGResponse(w, r, jpegData)
}

// sendJPEGResponse sends the JPEG data using format from query parameter
func (h *Handler) sendJPEGResponse(w http.ResponseWriter, r *http.Request, data []byte) {
	// Check format parameter - default to binary for better performance
	format := r.URL.Query().Get("format")
	if format == "json" {
		h.sendJSONResponse(w, data)
	} else {
		h.sendBinaryJPEGResponse(w, data)
	}
}

// sendBinaryJPEGResponse streams raw JPEG data (more efficient)
func (h *Handler) sendBinaryJPEGResponse(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year cache
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		// Client may have disconnected, log but don't panic
		log.Printf("Failed to write response: %v", err)
	}
}

// sendJSONResponse sends the JPEG data as base64 in JSON response (legacy format)
func (h *Handler) sendJSONResponse(w http.ResponseWriter, data []byte) {
	base64Data := base64.StdEncoding.EncodeToString(data)
	response := `{"data":"data:image/jpeg;base64,` + base64Data + `"}`

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(response)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(response)); err != nil {
		// Client may have disconnected, log but don't panic
		log.Printf("Failed to write response: %v", err)
	}
}

// Health handles the /health endpoint for readiness/liveness probes
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
		log.Printf("Failed to write health response: %v", err)
	}
}

// isHEIFExtension checks if the filename has a HEIF/HEIC extension
func isHEIFExtension(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".heif") || strings.HasSuffix(lower, ".heic")
}

// isValidHEIF validates the file magic bytes to confirm it's a HEIF file
// HEIF files follow ISO Base Media File Format (ISOBMFF)
// They start with: [4 bytes size] + "ftyp" + [brand]
func isValidHEIF(data []byte) bool {
	if len(data) < 12 {
		return false
	}

	// Check for "ftyp" at offset 4 (ISOBMFF format)
	ftyp := string(data[4:8])
	if ftyp != ftypMagic {
		return false
	}

	// Check for known HEIF brands (heic, heim, heis, heix, mif1)
	brand := strings.ToLower(string(data[8:12]))
	validBrands := []string{"heic", "heim", "heis", "heix", "mif1"}
	for _, vb := range validBrands {
		if brand == vb {
			return true
		}
	}

	// Accept any brand starting with "he" for broader compatibility
	return strings.HasPrefix(brand, "he")
}
