package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/harliandi/go-heif/internal/config"
	"github.com/harliandi/go-heif/internal/converter"
	"github.com/harliandi/go-heif/internal/handler"
	"github.com/harliandi/go-heif/internal/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()

	// Initialize global worker pool for conversion jobs
	converter.InitGlobalWorkerPool(cfg.WorkerCount, cfg.TargetSizeKB)

	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)
	mux.HandleFunc("/health", h.Health)
	mux.Handle("/metrics", promhttp.Handler())

	// Apply middlewares in order (outermost first):
	// 1. Security headers (always applied)
	// 2. Rate limiting (per IP)
	// 3. Concurrency limit (global)
	// 4. Recovery (catches panics)
	// 5. Logger (logs requests)
	handler := middleware.Security(
		middleware.RateLimit(cfg.RateLimitPerSec, cfg.RateLimitBurst)(
			middleware.ConcurrencyLimit(cfg.MaxConcurrent)(
				middleware.Recovery(
					middleware.Logger(mux),
				),
			),
		),
	)

	// Configure server with timeouts to prevent slowloris and hanging connections
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting HEIF to JPEG conversion API on %s", server.Addr)
	log.Printf("Target size: %dKB, Max upload: %dMB, Max concurrent: %d, Rate limit: %d/sec, Workers: %d",
		cfg.TargetSizeKB, cfg.MaxUploadMB, cfg.MaxConcurrent, cfg.RateLimitPerSec, cfg.WorkerCount)

	if err := server.ListenAndServe(); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
