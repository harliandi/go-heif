# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies (libde265 for HEIF, libjpeg-turbo for fast JPEG encoding)
RUN apk add --no-cache \
    git \
    gcc \
    g++ \
    musl-dev \
    libde265-dev \
    libjpeg-turbo-dev \
    pkgconfig

WORKDIR /app

# Copy go mod files
COPY go.* ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled
RUN CGO_ENABLED=1 go build -ldflags="-w -s" -o api ./cmd/api

# Runtime stage
FROM alpine:latest

# Runtime dependencies for CGO-linked binary
RUN apk --no-cache add \
    ca-certificates \
    libde265 \
    libjpeg-turbo

COPY --from=builder /app/api /usr/local/bin/

EXPOSE 8080

CMD ["api"]
