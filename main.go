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

	// Layer 3: HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/api/form/", handler.NewFormSubmitHandler(svc))
	mux.HandleFunc("/api/recaptcha/verify", handler.NewRecaptchaVerifyHandler(svc))
	mux.HandleFunc("/.well-known/health", handler.NewHealthHandler())

	// CORS middleware
	var h http.Handler = mux
	if len(cfg.CORSDomains) > 0 || cfg.CORSAllowLocalhost {
		h = handler.CORSMiddleware(cfg.CORSDomains, cfg.CORSAllowLocalhost, mux)
	}

	log.Printf("Starting mautic-form-proxy-api on %s (upstream: %s)", cfg.ListenAddr, cfg.MauticBaseURL)
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
