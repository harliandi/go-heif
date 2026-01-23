# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a lightweight Go HTTP service that converts HEIF/HEIC images to JPEG format. The service features adaptive quality targeting to achieve optimal output sizes (~500KB by default), strips EXIF metadata for privacy, and provides a RESTful API with multipart upload support.

## Development Commands

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./internal/converter
go test ./pkg/quality

# Run the server locally
go run cmd/api/main.go

# Build the binary
go build -o api ./cmd/api

# Build Docker image
docker build -t go-heif-api .

# Development deployment with hot-reload (requires Kubernetes cluster)
skaffold dev

# Production deployment
skaffold run

# Delete deployment
skaffold delete
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Server port |
| `MAX_UPLOAD_MB` | 10 | Max upload size in MB |
| `TARGET_SIZE_KB` | 500 | Default target output size in KB |

## Architecture

### Request Flow

1. HTTP request → `internal/handler/handler.go` (validates multipart form, file extension, magic bytes)
2. Handler → `internal/converter/converter.go` (HEIF decoding using `github.com/adrium/goheif`)
3. Converter → `pkg/quality/adaptive.go` (calculates optimal JPEG quality)
4. Converter → `pkg/jpeg/turbo.go` (JPEG encoding via libjpeg-turbo CGO or stdlib fallback)
5. Response: base64-encoded JPEG in JSON format

### Conversion Modes

The converter supports multiple modes via query parameters:

- **Adaptive quality** (default): Uses mathematical estimation to target `max_size` KB
- **Fixed quality**: Pass `quality=1-100` to skip adaptive calculation
- **Fast mode**: Pass `scale=0.5` (or similar) to downscale during conversion

### Key Packages

- `internal/converter/`: Core conversion engine with `ConvertBytes()`, `ConvertBytesWithQuality()`, `ConvertBytesFast()`, `ConvertBytesFastWithQuality()`
- `internal/handler/`: HTTP handlers for `/convert` (POST) and `/health` (GET)
- `pkg/quality/`: Zero-iteration quality estimation using inverse square law; adjusts for pixel density (>20MP gets -5 quality, <3MP gets +5)
- `pkg/jpeg/`: Optional libjpeg-turbo CGO binding (3.8x faster than stdlib), with graceful fallback

## Testing Notes

- Test HEIF/HEIC files are located in `testdata/`
- The project has comprehensive test coverage for converter, quality algorithm, and handlers
- Use `go test ./pkg/quality` to verify the adaptive quality algorithm specifically

## Performance Characteristics

- Zero-iteration quality estimation (no iterative encoding loops)
- libjpeg-turbo provides 3.8x speedup over Go's stdlib when available
- In-memory processing with pre-allocated buffers
- EXIF stripping is done during conversion, not as a separate pass

## Deployment

Kubernetes manifests are in `k8s/`:
- `deployment.yaml`: 3 replicas, 500m CPU request, 512Mi memory limit
- `service.yaml`: ClusterIP on port 80
- `hpa.yaml`: Auto-scales 2-10 pods based on CPU/memory
- `ingress.yaml`: External access (optional)

Skaffold configuration supports dev (hot-reload) and prod profiles.
