package handler

import (
	"encoding/json"
	"net/http"
)

// APIPathPrefix is the URL prefix under which this proxy's own API endpoints live.
// Every request whose path does not begin with this prefix is reverse-proxied to
// the configured upstream (Mautic).
const APIPathPrefix = "/_form-proxy-api"

// API endpoint paths.
const (
	FormSubmitPath     = APIPathPrefix + "/form/"
	RecaptchaVerifyPath = APIPathPrefix + "/recaptcha/verify"
	HealthPath          = APIPathPrefix + "/health"
)

type FormSubmitRequest struct {
	Fields         map[string]string `json:"fields"`
	RecaptchaToken string            `json:"recaptcha_token,omitempty"`
}

type FormSubmitResponse struct {
	Success bool     `json:"success"`
	Errors  []string `json:"errors,omitempty"`
}

type RecaptchaVerifyRequest struct {
	Token string `json:"token"`
}

type RecaptchaVerifyResponse struct {
	Success bool     `json:"success"`
	Score   float64  `json:"score,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
