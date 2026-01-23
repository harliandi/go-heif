# HEIF to JPEG Conversion API

A lightweight Go HTTP service that converts HEIF/HEIC images to JPEG format in real-time.

## Features

- Converts HEIF/HEIC images to JPEG
- Adaptive quality targeting ~500KB output size
- Strips EXIF metadata for privacy
- RESTful API with multipart upload
- Kubernetes-ready with Skaffold deployment
- Comprehensive test coverage

## API Usage

### Convert Endpoint

```bash
curl -X POST \
  -F "file=@image.heic" \
  http://localhost:8080/convert \
  --output image.jpg
```

### Query Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `max_size` | Target output size in KB | 500 |
| `quality` | Fixed quality (1-100), skips adaptive | - |

```bash
# Use custom target size
curl -X POST -F "file=@image.heic" "http://localhost:8080/convert?max_size=1000"

# Use fixed quality
curl -X POST -F "file=@image.heic" "http://localhost:8080/convert?quality=90"
```

### Health Check

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## Development

### Prerequisites

- Go 1.22+
- Docker
- Kubernetes cluster (for deployment)
- Skaffold (optional, for deployment)

### Run Locally

```bash
# Run tests
go test ./...

# Run server
go run cmd/api/main.go
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Server port |
| `MAX_UPLOAD_MB` | 10 | Max upload size in MB |
| `TARGET_SIZE_KB` | 500 | Default target output size |

## Deployment

### Using Skaffold

```bash
# Development (hot-reload)
skaffold dev

# One-time deployment
skaffold run

# Delete deployment
skaffold delete
```

### Using kubectl

```bash
# Build image
docker build -t go-heif-api .

# Apply manifests
kubectl apply -f k8s/
```

## Kubernetes Resources

| Resource | Description |
|----------|-------------|
| Deployment | 3 replicas, 500m CPU, 512Mi memory limit |
| Service | ClusterIP on port 80 |
| HPA | Auto-scales 2-10 pods based on CPU/memory |
| Ingress | Optional, for external access |

## Project Structure

```
go-heif/
├── cmd/api/           # Main application entry point
├── internal/
│   ├── converter/     # Core conversion logic
│   ├── handler/       # HTTP handlers
│   ├── middleware/    # Logging middleware
│   └── config/        # Configuration
├── pkg/quality/       # Adaptive quality algorithm
├── k8s/               # Kubernetes manifests
├── testdata/          # Test files
├── Dockerfile
├── skaffold.yaml
└── README.md
```

## License

MIT
