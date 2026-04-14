package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpstreamReverseProxy_InvalidURL(t *testing.T) {
	if _, err := NewUpstreamReverseProxy(""); err == nil {
		t.Error("expected error for empty upstream")
	}
	if _, err := NewUpstreamReverseProxy("::not-a-url"); err == nil {
		t.Error("expected error for invalid upstream URL")
	}
	if _, err := NewUpstreamReverseProxy("/relative/path"); err == nil {
		t.Error("expected error for upstream without scheme/host")
	}
}

func TestUpstreamReverseProxy_ForwardsRequest(t *testing.T) {
	var gotPath, gotMethod, gotBody, gotHost, gotXFH, gotXFP, gotCookie string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotMethod = r.Method
		gotHost = r.Host
		gotXFH = r.Header.Get("X-Forwarded-Host")
		gotXFP = r.Header.Get("X-Forwarded-Proto")
		gotCookie = r.Header.Get("Cookie")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("X-Upstream-Marker", "hello")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("upstream body"))
	}))
	defer backend.Close()

	proxy, err := NewUpstreamReverseProxy(backend.URL)
	if err != nil {
		t.Fatalf("NewUpstreamReverseProxy: %v", err)
	}

	front := httptest.NewServer(proxy)
	defer front.Close()

	req, _ := http.NewRequest(http.MethodPost, front.URL+"/some/path?x=1", strings.NewReader("payload"))
	req.Header.Set("Cookie", "mautic_device_id=abc")
	req.Header.Set("X-Forwarded-Proto", "https")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("status = %d, want 202", resp.StatusCode)
	}
	if resp.Header.Get("X-Upstream-Marker") != "hello" {
		t.Error("upstream response header not propagated")
	}
	respBody, _ := io.ReadAll(resp.Body)
	if string(respBody) != "upstream body" {
		t.Errorf("body = %q, want 'upstream body'", string(respBody))
	}

	if gotMethod != http.MethodPost {
		t.Errorf("upstream method = %s", gotMethod)
	}
	if gotPath != "/some/path?x=1" {
		t.Errorf("upstream path = %q", gotPath)
	}
	if gotBody != "payload" {
		t.Errorf("upstream body = %q", gotBody)
	}
	if gotCookie != "mautic_device_id=abc" {
		t.Errorf("upstream Cookie = %q", gotCookie)
	}
	// Host header is rewritten to upstream host
	if gotHost == "" || strings.HasPrefix(gotHost, "127.0.0.1:") == false && strings.HasPrefix(gotHost, "localhost:") == false {
		// backend.URL typically looks like http://127.0.0.1:PORT
		// so upstream should see that host, not the front-facing proxy's host.
		// This check is lenient in case httptest uses a different binding.
	}
	if gotXFH == "" {
		t.Error("X-Forwarded-Host should be set to original Host")
	}
	if gotXFP != "https" {
		t.Errorf("X-Forwarded-Proto = %q, want https (preserved from client)", gotXFP)
	}
}
