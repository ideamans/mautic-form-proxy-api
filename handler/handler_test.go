package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ideamans/mautic-form-api-proxy/service"
)

// --- Mock FormService ---

type mockFormService struct {
	recaptchaEnabled    bool
	verifyResult        *service.RecaptchaVerifyResult
	verifyErr           error
	submitResult        *service.FormSubmitResult
	submitErr           error
	lastSubmitFormID    int
	lastSubmitFields    map[string]string
	lastSubmitRecaptcha string
	lastVerifyToken     string
}

func (m *mockFormService) RecaptchaEnabled() bool {
	return m.recaptchaEnabled
}

func (m *mockFormService) VerifyRecaptcha(ctx context.Context, token string) (*service.RecaptchaVerifyResult, error) {
	m.lastVerifyToken = token
	return m.verifyResult, m.verifyErr
}

func (m *mockFormService) SubmitForm(ctx context.Context, formID int, fields map[string]string, recaptchaToken string) (*service.FormSubmitResult, error) {
	m.lastSubmitFormID = formID
	m.lastSubmitFields = fields
	m.lastSubmitRecaptcha = recaptchaToken
	return m.submitResult, m.submitErr
}

// --- FormSubmit handler tests ---

func TestFormSubmitHandler_MethodNotAllowed(t *testing.T) {
	h := NewFormSubmitHandler(&mockFormService{})
	req := httptest.NewRequest(http.MethodGet, "/api/form/15", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestFormSubmitHandler_InvalidFormID(t *testing.T) {
	h := NewFormSubmitHandler(&mockFormService{})

	tests := []struct {
		name string
		path string
	}{
		{"non-numeric", "/api/form/abc"},
		{"zero", "/api/form/0"},
		{"negative", "/api/form/-1"},
		{"empty", "/api/form/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(`{"fields":{}}`))
			w := httptest.NewRecorder()
			h(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestFormSubmitHandler_InvalidJSON(t *testing.T) {
	h := NewFormSubmitHandler(&mockFormService{})
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestFormSubmitHandler_Success(t *testing.T) {
	mock := &mockFormService{
		submitResult: &service.FormSubmitResult{Success: true},
	}
	h := NewFormSubmitHandler(mock)
	body, _ := json.Marshal(FormSubmitRequest{
		Fields:         map[string]string{"email": "a@b.com"},
		RecaptchaToken: "tok",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mock.lastSubmitFormID != 15 {
		t.Errorf("expected formID=15, got %d", mock.lastSubmitFormID)
	}
	if mock.lastSubmitRecaptcha != "tok" {
		t.Errorf("expected recaptcha=tok, got %q", mock.lastSubmitRecaptcha)
	}

	var resp FormSubmitResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success in response")
	}
}

func TestFormSubmitHandler_ServiceValidationError(t *testing.T) {
	mock := &mockFormService{
		submitResult: &service.FormSubmitResult{
			Success: false,
			Errors:  []string{"'Email' is required."},
		},
	}
	h := NewFormSubmitHandler(mock)
	body, _ := json.Marshal(FormSubmitRequest{Fields: map[string]string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestFormSubmitHandler_ServiceTransportError(t *testing.T) {
	mock := &mockFormService{
		submitErr: fmt.Errorf("connection refused"),
	}
	h := NewFormSubmitHandler(mock)
	body, _ := json.Marshal(FormSubmitRequest{Fields: map[string]string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/form/15", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

// --- RecaptchaVerify handler tests ---

func TestRecaptchaVerifyHandler_MethodNotAllowed(t *testing.T) {
	h := NewRecaptchaVerifyHandler(&mockFormService{recaptchaEnabled: true})
	req := httptest.NewRequest(http.MethodGet, "/api/recaptcha/verify", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestRecaptchaVerifyHandler_NotConfigured(t *testing.T) {
	h := NewRecaptchaVerifyHandler(&mockFormService{recaptchaEnabled: false})
	req := httptest.NewRequest(http.MethodPost, "/api/recaptcha/verify", bytes.NewBufferString(`{"token":"x"}`))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestRecaptchaVerifyHandler_InvalidJSON(t *testing.T) {
	h := NewRecaptchaVerifyHandler(&mockFormService{recaptchaEnabled: true})
	req := httptest.NewRequest(http.MethodPost, "/api/recaptcha/verify", bytes.NewBufferString("bad"))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRecaptchaVerifyHandler_Success(t *testing.T) {
	mock := &mockFormService{
		recaptchaEnabled: true,
		verifyResult:     &service.RecaptchaVerifyResult{Success: true, Score: 0.9},
	}
	h := NewRecaptchaVerifyHandler(mock)
	body, _ := json.Marshal(RecaptchaVerifyRequest{Token: "tok"})
	req := httptest.NewRequest(http.MethodPost, "/api/recaptcha/verify", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if mock.lastVerifyToken != "tok" {
		t.Errorf("expected token=tok, got %q", mock.lastVerifyToken)
	}

	var resp RecaptchaVerifyResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success")
	}
	if resp.Score != 0.9 {
		t.Errorf("expected score 0.9, got %f", resp.Score)
	}
}

func TestRecaptchaVerifyHandler_Failure(t *testing.T) {
	mock := &mockFormService{
		recaptchaEnabled: true,
		verifyResult:     &service.RecaptchaVerifyResult{Success: false, Score: 0.2, Errors: []string{"score too low"}},
	}
	h := NewRecaptchaVerifyHandler(mock)
	body, _ := json.Marshal(RecaptchaVerifyRequest{Token: "tok"})
	req := httptest.NewRequest(http.MethodPost, "/api/recaptcha/verify", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestRecaptchaVerifyHandler_TransportError(t *testing.T) {
	mock := &mockFormService{
		recaptchaEnabled: true,
		verifyErr:        fmt.Errorf("network error"),
	}
	h := NewRecaptchaVerifyHandler(mock)
	body, _ := json.Marshal(RecaptchaVerifyRequest{Token: "tok"})
	req := httptest.NewRequest(http.MethodPost, "/api/recaptcha/verify", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}
