package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORS_NoOriginHeader(t *testing.T) {
	h := CORSMiddleware(map[string]bool{"https://example.com": true}, dummyHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no CORS header when Origin is absent")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCORS_AllowedOrigin(t *testing.T) {
	h := CORSMiddleware(map[string]bool{"https://example.com": true}, dummyHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin=https://example.com, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	h := CORSMiddleware(map[string]bool{"https://example.com": true}, dummyHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", nil)
	req.Header.Set("Origin", "https://evil.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no CORS header for disallowed origin")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCORS_Wildcard(t *testing.T) {
	h := CORSMiddleware(map[string]bool{"*": true}, dummyHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", nil)
	req.Header.Set("Origin", "https://anything.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://anything.com" {
		t.Errorf("expected wildcard to allow any origin, got %q", w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORS_Preflight(t *testing.T) {
	h := CORSMiddleware(map[string]bool{"https://example.com": true}, dummyHandler())
	req := httptest.NewRequest(http.MethodOptions, "/api/form/15", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for preflight, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") != "POST, OPTIONS" {
		t.Errorf("expected Allow-Methods, got %q", w.Header().Get("Access-Control-Allow-Methods"))
	}
	if w.Header().Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Errorf("expected Allow-Headers, got %q", w.Header().Get("Access-Control-Allow-Headers"))
	}
}

func TestCORS_EmptyAllowedOrigins(t *testing.T) {
	h := CORSMiddleware(map[string]bool{}, dummyHandler())
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no CORS header when allowedOrigins is empty")
	}
}

func TestCORS_MultipleOrigins(t *testing.T) {
	origins := map[string]bool{
		"https://a.com": true,
		"https://b.com": true,
	}
	h := CORSMiddleware(origins, dummyHandler())

	for _, origin := range []string{"https://a.com", "https://b.com"} {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Errorf("expected origin %s to be allowed", origin)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Origin", "https://c.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected https://c.com to be rejected")
	}
}
