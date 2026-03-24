package handler

import (
	"net/http"
)

// CORSMiddleware wraps an http.Handler with CORS support.
// allowedOrigins is a set of permitted origins. If empty, CORS headers are not added.
// A wildcard "*" entry allows all origins.
func CORSMiddleware(allowedOrigins map[string]bool, next http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		return next
	}

	allowAll := allowedOrigins["*"]

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if allowAll || allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
