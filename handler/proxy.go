package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewUpstreamReverseProxy returns a handler that transparently reverse-proxies
// every request to the given upstream URL (typically the Mautic instance).
// The Host header is rewritten to the upstream host so Mautic sees requests as
// if they arrived directly. X-Forwarded-For / X-Forwarded-Host / X-Forwarded-Proto
// are set so downstream logs and tracking can see the original client.
func NewUpstreamReverseProxy(upstream string) (http.Handler, error) {
	if upstream == "" {
		return nil, fmt.Errorf("upstream URL is empty")
	}
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL %q: %w", upstream, err)
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, fmt.Errorf("upstream URL %q must include scheme and host", upstream)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	defaultDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origHost := req.Host
		origScheme := "http"
		if req.TLS != nil {
			origScheme = "https"
		}
		if v := req.Header.Get("X-Forwarded-Proto"); v != "" {
			origScheme = v
		}

		defaultDirector(req)
		req.Host = target.Host

		if origHost != "" {
			req.Header.Set("X-Forwarded-Host", origHost)
		}
		req.Header.Set("X-Forwarded-Proto", origScheme)
	}

	return proxy, nil
}
