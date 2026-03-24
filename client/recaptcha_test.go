package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGoogleRecaptchaVerifier_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.FormValue("secret") != "test-secret" {
			t.Errorf("expected secret=test-secret, got %q", r.FormValue("secret"))
		}
		if r.FormValue("response") != "test-token" {
			t.Errorf("expected response=test-token, got %q", r.FormValue("response"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"success":      true,
			"score":        0.9,
			"action":       "submit",
			"challenge_ts": "2026-03-25T00:00:00Z",
			"hostname":     "example.com",
		})
	}))
	defer server.Close()

	verifier := NewGoogleRecaptchaVerifier("test-secret")
	verifier.verifyURL = server.URL

	result, err := verifier.Verify(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if result.Score != 0.9 {
		t.Errorf("expected score 0.9, got %f", result.Score)
	}
}

func TestGoogleRecaptchaVerifier_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"success":     false,
			"error-codes": []string{"invalid-input-response"},
		})
	}))
	defer server.Close()

	verifier := NewGoogleRecaptchaVerifier("test-secret")
	verifier.verifyURL = server.URL

	result, err := verifier.Verify(context.Background(), "bad-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if len(result.ErrorCodes) != 1 || result.ErrorCodes[0] != "invalid-input-response" {
		t.Errorf("unexpected error codes: %v", result.ErrorCodes)
	}
}

func TestGoogleRecaptchaVerifier_NetworkError(t *testing.T) {
	verifier := NewGoogleRecaptchaVerifier("test-secret")
	verifier.verifyURL = "http://127.0.0.1:1"

	_, err := verifier.Verify(context.Background(), "token")
	if err == nil {
		t.Error("expected error for network failure")
	}
}

func TestGoogleRecaptchaVerifier_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	verifier := NewGoogleRecaptchaVerifier("test-secret")
	verifier.verifyURL = server.URL

	_, err := verifier.Verify(context.Background(), "token")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
