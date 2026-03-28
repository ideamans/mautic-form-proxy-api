package handler

import (
	"net/http"
	"net/url"
)

// isLocalhostOrigin returns true if the origin is http://localhost or http://127.0.0.1 (any port).
func isLocalhostOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1"
}

// CORSMiddleware wraps an http.Handler with CORS support.
// allowedOrigins is a set of permitted origins. If empty and allowLocalhost is false, CORS headers are not added.
// A wildcard "*" entry allows all origins.
// If allowLocalhost is true, any http://localhost:<port> or http://127.0.0.1:<port> origin is permitted.
func CORSMiddleware(allowedOrigins map[string]bool, allowLocalhost bool, next http.Handler) http.Handler {
	if len(allowedOrigins) == 0 && !allowLocalhost {
		return next
	}

	allowAll := allowedOrigins["*"]

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if allowAll || allowedOrigins[origin] || (allowLocalhost && isLocalhostOrigin(origin)) {
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
