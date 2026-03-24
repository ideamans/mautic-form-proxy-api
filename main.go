package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/ideamans/mautic-form-api-proxy/client"
	"github.com/ideamans/mautic-form-api-proxy/handler"
	"github.com/ideamans/mautic-form-api-proxy/service"
)

type Config struct {
	MauticBaseURL      string
	ListenAddr         string
	RecaptchaSecretKey string
	RecaptchaThreshold float64
}

func loadConfig() Config {
	threshold := 0.5
	if v := os.Getenv("RECAPTCHA_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			threshold = f
		}
	}

	cfg := Config{
		MauticBaseURL:      os.Getenv("MAUTIC_BASE_URL"),
		ListenAddr:         os.Getenv("LISTEN_ADDR"),
		RecaptchaSecretKey: os.Getenv("RECAPTCHA_SECRET_KEY"),
		RecaptchaThreshold: threshold,
	}
	if cfg.MauticBaseURL == "" {
		cfg.MauticBaseURL = "https://mautic.ideamans.com"
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
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

	log.Printf("Starting mautic-form-api-proxy on %s (upstream: %s)", cfg.ListenAddr, cfg.MauticBaseURL)
	if cfg.RecaptchaSecretKey != "" {
		log.Printf("reCAPTCHA: enabled (threshold: %.1f)", cfg.RecaptchaThreshold)
	} else {
		log.Printf("reCAPTCHA: disabled")
	}

	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatal(err)
	}
}
