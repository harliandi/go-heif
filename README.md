# HEIF to JPEG Conversion API

A high-performance Go HTTP service that converts HEIF/HEIC images to JPEG format in real-time.

## What It Does

- **Converts HEIF/HEIC images to JPEG** with adaptive quality targeting (~500KB default)
- **Multiple output modes**: raw JPEG streaming (default) or base64 JSON (`?format=json`)
- **Fast scaling**: Optional downsampling for speed (`?scale=0.5`)
- **Privacy-focused**: Strips EXIF metadata by default
- **RESTful API** with multipart upload support

## Performance & Security Features

| Feature | Description |
|---------|-------------|
| **Worker Pool** | Bounded goroutine pool for controlled CPU usage |
| **Rate Limiting** | Token-bucket per IP (configurable) |
| **Concurrency Limit** | Max simultaneous requests (prevents OOM) |
| **Panic Recovery** | Server survives crashes, returns HTTP 500 |
| **Security Headers** | CSP, X-Content-Type-Options, HSTS |
| **Request Validation** | File size limit (20MB), dimension checks |
| **Prometheus Metrics** | `/metrics` endpoint for monitoring |

## API Usage

### Convert (default: raw JPEG streaming)

```bash
curl -X POST \
  -F "file=@image.heic" \
  http://localhost:8080/convert \
  --output image.jpg
```

### Query Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `scale` | Downsample factor (0.1-1.0) | 0.5 |
| `quality` | Fixed quality 1-100 | adaptive |
| `max_size` | Target size in KB | 500 |
| `format` | `json` or binary | binary |

```bash
# Full resolution, adaptive quality
curl -X POST -F "file=@image.heic" "http://localhost:8080/convert?scale=1"

# Fixed quality 90
curl -X POST -F "file=@image.heic" "http://localhost:8080/convert?quality=90"

# Base64 JSON response (legacy)
curl -X POST -F "file=@image.heic" "http://localhost:8080/convert?format=json"
```

### Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/convert` | POST | Convert HEIF to JPEG |
| `/health` | GET | Health check |
| `/metrics` | GET | Prometheus metrics |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Server port |
| `MAX_UPLOAD_MB` | 10 | Max upload size (MB) |
| `TARGET_SIZE_KB` | 500 | Default output target (KB) |
| `MAX_CONCURRENT` | 50 | Max concurrent requests |
| `RATE_LIMIT` | 10 | Requests/sec per IP |
| `RATE_LIMIT_BURST` | 20 | Rate limit burst |
| `WORKER_COUNT` | 10 | Conversion worker pool size |

## Development

```bash
# Run tests
go test ./...

# Run server
go run cmd/api/main.go

# Build
go build -o api ./cmd/api
```

## Deployment

```bash
# Using Skaffold
skaffold dev    # Development
skaffold run   # Production

# Using Docker
docker build -t go-heif-api .
kubectl apply -f k8s/
```

## Future Improvements

**Phase 3 (Enhancements):**
- Replace nearest-neighbor scaling with bicubic/Lanczos
- Add API key system for tiered quotas
- Distributed rate limiting (Redis) for multi-pod deployments

**Considered for Later:**
- WebP/AVIF output format support
- Thumbnail generation presets
- S3/integrated storage backends
- Authentication/OAuth integration
- Batch conversion endpoint

## Project Structure

```
go-heif/
├── cmd/api/              # Main entry point
├── internal/
│   ├── converter/        # Core conversion, worker pool, validation
│   ├── handler/          # HTTP handlers
│   ├── middleware/       # Security, rate limit, concurrency, logging
│   └── config/           # Configuration
├── pkg/
│   ├── metrics/          # Prometheus metrics
│   ├── quality/          # Adaptive quality algorithm
│   └── jpeg/             # libjpeg-turbo CGO binding
├── k8s/                  # Kubernetes manifests
└── testdata/             # Test files
```

## License

MIT
