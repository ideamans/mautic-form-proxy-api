package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/ideamans/mautic-form-proxy-api/client"
)

// --- Mocks ---

type mockRecaptchaVerifier struct {
	result *client.RecaptchaResult
	err    error
}

func (m *mockRecaptchaVerifier) Verify(ctx context.Context, token string) (*client.RecaptchaResult, error) {
	return m.result, m.err
}

type mockMauticSubmitter struct {
	result      *client.MauticSubmitResult
	err         error
	lastHeaders *client.ForwardHeaders
}

func (m *mockMauticSubmitter) Submit(ctx context.Context, formID int, fields map[string]string, headers *client.ForwardHeaders) (*client.MauticSubmitResult, error) {
	m.lastHeaders = headers
	return m.result, m.err
}

// --- VerifyRecaptcha tests ---

func TestVerifyRecaptcha_NotConfigured(t *testing.T) {
	svc := NewFormService(nil, &mockMauticSubmitter{}, 0.5)
	_, err := svc.VerifyRecaptcha(context.Background(), "token")
	if err == nil {
		t.Error("expected error when reCAPTCHA not configured")
	}
}

func TestVerifyRecaptcha_EmptyToken(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{}, &mockMauticSubmitter{}, 0.5)
	result, err := svc.VerifyRecaptcha(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for empty token")
	}
}

func TestVerifyRecaptcha_Success(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{
		result: &client.RecaptchaResult{Success: true, Score: 0.9},
	}, &mockMauticSubmitter{}, 0.5)

	result, err := svc.VerifyRecaptcha(context.Background(), "token")
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

func TestVerifyRecaptcha_ScoreTooLow(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{
		result: &client.RecaptchaResult{Success: true, Score: 0.3},
	}, &mockMauticSubmitter{}, 0.5)

	result, err := svc.VerifyRecaptcha(context.Background(), "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for low score")
	}
}

func TestVerifyRecaptcha_GoogleRejects(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{
		result: &client.RecaptchaResult{Success: false, ErrorCodes: []string{"bad-request"}},
	}, &mockMauticSubmitter{}, 0.5)

	result, err := svc.VerifyRecaptcha(context.Background(), "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if len(result.Errors) != 1 || result.Errors[0] != "bad-request" {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestVerifyRecaptcha_TransportError(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{
		err: fmt.Errorf("network error"),
	}, &mockMauticSubmitter{}, 0.5)

	_, err := svc.VerifyRecaptcha(context.Background(), "token")
	if err == nil {
		t.Error("expected error")
	}
}

// --- SubmitForm tests ---

func TestSubmitForm_NoRecaptcha_Success(t *testing.T) {
	svc := NewFormService(nil, &mockMauticSubmitter{
		result: &client.MauticSubmitResult{Success: true},
	}, 0.5)

	result, err := svc.SubmitForm(context.Background(), 15, map[string]string{"email": "a@b.com"}, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestSubmitForm_RecaptchaEnabled_NoToken(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{}, &mockMauticSubmitter{}, 0.5)

	result, err := svc.SubmitForm(context.Background(), 15, map[string]string{}, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when token missing")
	}
}

func TestSubmitForm_RecaptchaEnabled_ValidToken(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{
		result: &client.RecaptchaResult{Success: true, Score: 0.9},
	}, &mockMauticSubmitter{
		result: &client.MauticSubmitResult{Success: true},
	}, 0.5)

	result, err := svc.SubmitForm(context.Background(), 15, map[string]string{"email": "a@b.com"}, "valid-token", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestSubmitForm_RecaptchaEnabled_LowScore(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{
		result: &client.RecaptchaResult{Success: true, Score: 0.2},
	}, &mockMauticSubmitter{
		result: &client.MauticSubmitResult{Success: true},
	}, 0.5)

	result, err := svc.SubmitForm(context.Background(), 15, map[string]string{}, "token", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for low score")
	}
}

func TestSubmitForm_RecaptchaEnabled_VerifyFails(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{
		result: &client.RecaptchaResult{Success: false},
	}, &mockMauticSubmitter{
		result: &client.MauticSubmitResult{Success: true},
	}, 0.5)

	result, err := svc.SubmitForm(context.Background(), 15, map[string]string{}, "bad-token", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
}

func TestSubmitForm_MauticValidationError(t *testing.T) {
	svc := NewFormService(nil, &mockMauticSubmitter{
		result: &client.MauticSubmitResult{
			Success: false,
			Errors:  []string{"'Email' is required."},
		},
	}, 0.5)

	result, err := svc.SubmitForm(context.Background(), 15, map[string]string{}, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if len(result.Errors) != 1 || result.Errors[0] != "'Email' is required." {
		t.Errorf("unexpected errors: %v", result.Errors)
	}
}

func TestSubmitForm_MauticTransportError(t *testing.T) {
	svc := NewFormService(nil, &mockMauticSubmitter{
		err: fmt.Errorf("connection refused"),
	}, 0.5)

	_, err := svc.SubmitForm(context.Background(), 15, map[string]string{}, "", nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestSubmitForm_RecaptchaTransportError(t *testing.T) {
	svc := NewFormService(&mockRecaptchaVerifier{
		err: fmt.Errorf("network error"),
	}, &mockMauticSubmitter{}, 0.5)

	_, err := svc.SubmitForm(context.Background(), 15, map[string]string{}, "token", nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestRecaptchaEnabled(t *testing.T) {
	svcWith := NewFormService(&mockRecaptchaVerifier{}, &mockMauticSubmitter{}, 0.5)
	svcWithout := NewFormService(nil, &mockMauticSubmitter{}, 0.5)

	if !svcWith.RecaptchaEnabled() {
		t.Error("expected RecaptchaEnabled=true")
	}
	if svcWithout.RecaptchaEnabled() {
		t.Error("expected RecaptchaEnabled=false")
	}
}
