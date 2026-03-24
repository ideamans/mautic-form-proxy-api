package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type RecaptchaResult struct {
	Success    bool
	Score      float64
	ErrorCodes []string
}

type RecaptchaVerifier interface {
	Verify(ctx context.Context, token string) (*RecaptchaResult, error)
}

type GoogleRecaptchaVerifier struct {
	secretKey  string
	verifyURL  string
	httpClient *http.Client
}

func NewGoogleRecaptchaVerifier(secretKey string) *GoogleRecaptchaVerifier {
	return &GoogleRecaptchaVerifier{
		secretKey:  secretKey,
		verifyURL:  "https://www.google.com/recaptcha/api/siteverify",
		httpClient: http.DefaultClient,
	}
}

type googleRecaptchaResponse struct {
	Success     bool     `json:"success"`
	Score       float64  `json:"score"`
	Action      string   `json:"action"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

func (v *GoogleRecaptchaVerifier) Verify(ctx context.Context, token string) (*RecaptchaResult, error) {
	resp, err := v.httpClient.PostForm(v.verifyURL, url.Values{
		"secret":   {v.secretKey},
		"response": {token},
	})
	if err != nil {
		return nil, fmt.Errorf("recaptcha: failed to contact Google: %w", err)
	}
	defer resp.Body.Close()

	var gr googleRecaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("recaptcha: failed to decode response: %w", err)
	}

	return &RecaptchaResult{
		Success:    gr.Success,
		Score:      gr.Score,
		ErrorCodes: gr.ErrorCodes,
	}, nil
}
