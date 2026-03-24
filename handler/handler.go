package handler

import (
	"encoding/json"
	"net/http"
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
