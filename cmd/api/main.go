package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/harliandi/go-heif/internal/config"
	"github.com/harliandi/go-heif/internal/handler"
	"github.com/harliandi/go-heif/internal/middleware"
)

func main() {
	cfg := config.Load()

	h := handler.New(cfg.TargetSizeKB, cfg.MaxUploadMB)

	mux := http.NewServeMux()
	mux.HandleFunc("/convert", h.Convert)
	mux.HandleFunc("/health", h.Health)

	// Wrap with logging middleware
	handler := middleware.Logger(mux)

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("Starting HEIF to JPEG conversion API on %s", addr)
	log.Printf("Target size: %dKB, Max upload: %dMB", cfg.TargetSizeKB, cfg.MaxUploadMB)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
