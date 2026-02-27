package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig holds the configuration for CORS middleware
type CORSConfig struct {
	AllowedOrigins []string // List of allowed origins (use "*" to allow all, but not recommended for production)
	AllowedMethods []string // HTTP methods allowed
	AllowedHeaders []string // HTTP headers allowed
	MaxAge         int      // Preflight cache duration in seconds
}

// DefaultCORSConfig returns a default CORS configuration with commonly allowed methods and headers
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{}, // Empty means no origins allowed by default - must be configured
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-Secret-Token", "X-Requested-With"},
		MaxAge:         300, // 5 minutes
	}
}

// CORS returns a CORS middleware handler with the given configuration
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			for _, allowedOrigin := range config.AllowedOrigins {
				if allowedOrigin == "*" {
					allowed = true
					break
				} else if strings.EqualFold(allowedOrigin, origin) {
					allowed = true
					break
				}
			}

			// For requests without Origin header, allow if no origins are configured (development mode)
			// In production, you should configure specific allowed origins
			if origin == "" && len(config.AllowedOrigins) == 0 {
				allowed = true
			}

			if allowed && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else if allowed && origin == "" {
				// Allow requests without Origin header
			}

			// Handle preflight requests
			if r.Method == http.MethodOptions {
				if allowed {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// Add CORS headers for actual request if origin was allowed
			if allowed && origin != "" {
				w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")
			}

			next.ServeHTTP(w, r)
		})
	}
}
