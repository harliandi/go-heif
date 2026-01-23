# HEIF to JPEG Conversion API Design

**Date**: 2025-01-23
**Author**: Claude
**Status**: Approved

## Overview

A lightweight Go HTTP service that converts HEIF/HEIC images to JPEG format in real-time. The service accepts multipart file uploads, converts using the `adrium/goheif` library, and returns the JPEG with adaptive quality targeting ~500KB max file size. EXIF metadata is stripped for privacy.

## Requirements

- Convert HEIF/HEIC images to JPEG
- Synchronous HTTP API (upload → immediate response)
- Adaptive quality targeting ~500KB output size
- Strip EXIF metadata for privacy
- Deploy to existing Kubernetes cluster using Skaffold
- Comprehensive test coverage

## HTTP Interface

### Endpoint

```
POST /convert
Content-Type: multipart/form-data
```

### Request Parameters (Query String)

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_size` | int | 500 | Target output size in KB |
| `quality` | int | - | Fixed quality 1-100 (skips adaptive) |

### Response

**Success**: `200 OK`
```
Content-Type: image/jpeg
<jpeg binary data>
```

**Errors**:
| Code | Description |
|------|-------------|
| 400 | Invalid request, no file uploaded |
| 413 | Upload exceeds size limit |
| 415 | Not a HEIF/HEIC file |
| 500 | Conversion failed |

## Architecture

### Project Structure

```
go-heif/
├── cmd/api/
│   └── main.go              # Application entry point
├── internal/
│   ├── converter/
│   │   ├── converter.go     # Core conversion logic
│   │   └── converter_test.go
│   ├── handler/
│   │   ├── handler.go       # HTTP handlers
│   │   └── handler_test.go
│   ├── middleware/
│   │   ├── logger.go        # Logging middleware
│   │   └── limits.go        # Request limits
│   └── config/
│       └── config.go        # Configuration
├── pkg/quality/
│   ├── adaptive.go          # Adaptive quality algorithm
│   └── adaptive_test.go
├── k8s/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── hpa.yaml
│   └── ingress.yaml         # Optional
├── testdata/
│   └── *.heif               # Sample HEIF files for testing
├── Dockerfile
├── skaffold.yaml
├── go.mod
└── README.md
```

### Core Components

#### 1. Converter Package (`internal/converter/`)

Wraps `goheif` library with clean interface:

```go
type Converter struct {
    maxSizeKB int
}

func (c *Converter) Convert(source io.Reader) (io.Reader, error)
```

- Handles decoding HEIF and encoding JPEG
- Strips EXIF data during conversion
- Uses adaptive quality algorithm

#### 2. Quality Package (`pkg/quality/`)

Adaptive sizing algorithm:

```go
func FindOptimalQuality(img image.Image, targetSizeKB int) (int, error)
```

**Algorithm**:
- Binary search approach to find optimal quality
- Start at quality 85, measure output size
- Adjust up/down based on distance from target
- Max 3-4 iterations (balance speed vs accuracy)
- Quality bounds: 10-100

#### 3. Handler Package (`internal/handler/`)

HTTP layer:

- Single `/convert` endpoint handler
- Multipart form parsing with memory limits
- Content-Type validation (extension + magic bytes)
- Proper error responses with appropriate status codes

#### 4. Config (`internal/config/`)

Environment-based configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Server port |
| `MAX_UPLOAD_MB` | 10 | Max upload size in MB |
| `TARGET_SIZE_KB` | 500 | Default target output size |

## Testing Strategy

### Unit Tests

**converter/**:
- Valid HEIF → JPEG conversion
- Invalid HEIF error handling
- Quality parameter application

**quality/**:
- Target size within tolerance
- Quality min/max bounds (10-100)
- Small image edge cases
- Large target size handling

**handler/**:
- Valid request → JPEG response
- No file → 400 error
- Invalid format → 415 error
- Size limit → 413 error
- Custom size parameter

### Integration Tests

- End-to-end HTTP request cycle
- Actual HEIF test files from `testdata/`

### Benchmark Tests

- `BenchmarkConversion`: Conversion time
- `BenchmarkAdaptiveQuality`: Algorithm performance

## Deployment

### Dockerfile

Multi-stage build for minimal image:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o api ./cmd/api

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/api /usr/local/bin/
EXPOSE 8080
CMD ["api"]
```

### Skaffold Configuration

```yaml
apiVersion: skaffold/v4beta11
kind: Config
metadata:
  name: go-heif-api
build:
  artifacts:
  - image: go-heif-api
    docker:
      dockerfile: Dockerfile
deploy:
  kubectl:
    manifests:
    - k8s/deployment.yaml
    - k8s/service.yaml
    - k8s/hpa.yaml
```

### Kubernetes Manifests

**Deployment** (`k8s/deployment.yaml`):
- Replicas: 3
- Resource limits: 500Mi memory, 500m CPU
- Resource requests: 250Mi memory, 250m CPU
- Liveness/Readiness probes on `/health`

**Service** (`k8s/service.yaml`):
- ClusterIP or LoadBalancer
- Port 80 → 8080

**HPA** (`k8s/hpa.yaml`):
- Scale on CPU (target 70%)
- Min 2 pods, max 10 pods

### Deployment Commands

```bash
# Development (hot-reload)
skaffold dev

# Production deployment
skaffold run

# Remove deployment
skaffold delete
```

## Dependencies

- `github.com/adrium/goheif` - HEIF/HEIC decoding
- Standard library `net/http` - HTTP server
- Standard library `image/jpeg` - JPEG encoding

## Security Considerations

- Strip EXIF metadata (privacy)
- Upload size limits (DoS prevention)
- Input validation (file type checking)
- No shell command execution

## Performance Targets

- Conversion time: < 1 second for typical images
- Memory usage: < 100MB per request
- Support concurrent conversions
