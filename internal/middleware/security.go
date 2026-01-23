package middleware

import (
	"net/http"
)

// Security adds security-related headers to all responses
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Enable XSS filter (already default in modern browsers)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Restrict referrer information
		w.Header().Set("Referrer-Policy", "no-referrer")

		// Content Security Policy for API endpoints
		w.Header().Set("Content-Security-Policy", "default-src 'none'")

		// HSTS (only if using HTTPS)
		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
