package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/ideamans/mautic-form-proxy-api/client"
	"github.com/ideamans/mautic-form-proxy-api/handler"
	"github.com/ideamans/mautic-form-proxy-api/service"
)

type Config struct {
	MauticBaseURL      string
	MauticUpstreamURL  string
	ListenAddr         string
	RecaptchaSecretKey string
	RecaptchaThreshold float64
	CORSDomains        map[string]bool
	CORSAllowLocalhost bool
}

func loadConfig() Config {
	threshold := 0.5
	if v := os.Getenv("RECAPTCHA_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			threshold = f
		}
	}

	corsDomains := make(map[string]bool)
	if v := os.Getenv("CORS_DOMAINS"); v != "" {
		for _, d := range strings.Split(v, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				corsDomains[d] = true
			}
		}
	}

	allowLocalhost := false
	if v := os.Getenv("CORS_ALLOW_LOCALHOST"); v != "" {
		v = strings.ToLower(v)
		allowLocalhost = v == "true" || v == "1" || v == "yes"
	}

	cfg := Config{
		MauticBaseURL:      os.Getenv("MAUTIC_BASE_URL"),
		MauticUpstreamURL:  os.Getenv("MAUTIC_UPSTREAM_URL"),
		ListenAddr:         os.Getenv("LISTEN_ADDR"),
		RecaptchaSecretKey: os.Getenv("RECAPTCHA_SECRET_KEY"),
		RecaptchaThreshold: threshold,
		CORSDomains:        corsDomains,
		CORSAllowLocalhost: allowLocalhost,
	}
	if cfg.MauticBaseURL == "" {
		cfg.MauticBaseURL = "https://mautic.ideamans.com"
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":3000"
	}
	return cfg
}

func main() {
	cfg := loadConfig()

	// Layer 1: external clients
	var recaptchaVerifier client.RecaptchaVerifier
	if cfg.RecaptchaSecretKey != "" {
		recaptchaVerifier = client.NewGoogleRecaptchaVerifier(cfg.RecaptchaSecretKey)
	}
	mauticSubmitter := client.NewHTTPMauticSubmitter(cfg.MauticBaseURL)

	// Layer 2: service
	svc := service.NewFormService(recaptchaVerifier, mauticSubmitter, cfg.RecaptchaThreshold)

	// Layer 3: HTTP handlers (API endpoints under /_form-proxy-api/)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc(handler.FormSubmitPath, handler.NewFormSubmitHandler(svc))
	apiMux.HandleFunc(handler.RecaptchaVerifyPath, handler.NewRecaptchaVerifyHandler(svc))
	apiMux.HandleFunc(handler.HealthPath, handler.NewHealthHandler())

	var apiHandler http.Handler = apiMux
	if len(cfg.CORSDomains) > 0 || cfg.CORSAllowLocalhost {
		apiHandler = handler.CORSMiddleware(cfg.CORSDomains, cfg.CORSAllowLocalhost, apiMux)
	}

	topMux := http.NewServeMux()
	topMux.Handle(handler.APIPathPrefix+"/", apiHandler)

	// Reverse proxy every other path to the Mautic upstream, if configured.
	// Intentionally separate from MAUTIC_BASE_URL so this container can sit in
	// front of a Mautic instance on a different hostname (e.g. a localhost
	// Mautic container) without creating an infinite loop.
	if cfg.MauticUpstreamURL != "" {
		upstreamProxy, err := handler.NewUpstreamReverseProxy(cfg.MauticUpstreamURL)
		if err != nil {
			log.Fatalf("failed to build upstream reverse proxy: %v", err)
		}
		topMux.Handle("/", upstreamProxy)
	}
	var h http.Handler = topMux

	log.Printf("Starting mautic-form-proxy-api on %s (Mautic API target: %s)", cfg.ListenAddr, cfg.MauticBaseURL)
	if cfg.MauticUpstreamURL != "" {
		log.Printf("Reverse proxy: enabled (upstream: %s)", cfg.MauticUpstreamURL)
	} else {
		log.Printf("Reverse proxy: disabled (set MAUTIC_UPSTREAM_URL to enable)")
	}
	if cfg.RecaptchaSecretKey != "" {
		log.Printf("reCAPTCHA: enabled (threshold: %.1f)", cfg.RecaptchaThreshold)
	} else {
		log.Printf("reCAPTCHA: disabled")
	}
	if len(cfg.CORSDomains) > 0 || cfg.CORSAllowLocalhost {
		parts := []string{}
		if len(cfg.CORSDomains) > 0 {
			parts = append(parts, fmt.Sprintf("%d domain(s)", len(cfg.CORSDomains)))
		}
		if cfg.CORSAllowLocalhost {
			parts = append(parts, "localhost allowed")
		}
		log.Printf("CORS: %s", strings.Join(parts, ", "))
	} else {
		log.Printf("CORS: disabled")
	}

	if err := http.ListenAndServe(cfg.ListenAddr, h); err != nil {
		log.Fatal(err)
	}
}
