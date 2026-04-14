package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMauticSubmitter_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Location", "https://www.example.com/success")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	submitter := NewHTTPMauticSubmitter(server.URL)
	result, err := submitter.Submit(context.Background(), 15, map[string]string{
		"email":  "test@example.com",
		"f_name": "Test",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got errors: %v", result.Errors)
	}
}

func TestHTTPMauticSubmitter_ValidationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		location := "https://example.com/callback?mauticError=Errors%3A%3Cbr%20%2F%3E%3Col%3E%3Cli%3E%27Email%27%20is%20required.%3C%2Fli%3E%3Cli%3E%27Name%27%20is%20required.%3C%2Fli%3E%3C%2Fol%3E"
		w.Header().Set("Location", location)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	submitter := NewHTTPMauticSubmitter(server.URL)
	result, err := submitter.Submit(context.Background(), 15, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure")
	}
	if len(result.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(result.Errors), result.Errors)
	}
	if result.Errors[0] != "'Email' is required." {
		t.Errorf("unexpected error[0]: %s", result.Errors[0])
	}
	if result.Errors[1] != "'Name' is required." {
		t.Errorf("unexpected error[1]: %s", result.Errors[1])
	}
}

func TestHTTPMauticSubmitter_NoRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	submitter := NewHTTPMauticSubmitter(server.URL)
	_, err := submitter.Submit(context.Background(), 15, map[string]string{}, nil)
	if err == nil {
		t.Error("expected error for missing redirect")
	}
}

func TestHTTPMauticSubmitter_NetworkError(t *testing.T) {
	submitter := NewHTTPMauticSubmitter("http://127.0.0.1:1")
	_, err := submitter.Submit(context.Background(), 15, map[string]string{}, nil)
	if err == nil {
		t.Error("expected error for network failure")
	}
}

func TestHTTPMauticSubmitter_FieldsAreSent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("failed to parse multipart: %v", err)
		}
		if v := r.FormValue("mauticform[email]"); v != "test@example.com" {
			t.Errorf("expected email field, got %q", v)
		}
		if v := r.FormValue("mauticform[formId]"); v != "15" {
			t.Errorf("expected formId=15, got %q", v)
		}
		if v := r.FormValue("mauticform[submit]"); v != "1" {
			t.Errorf("expected submit=1, got %q", v)
		}
		w.Header().Set("Location", "https://example.com/ok")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	submitter := NewHTTPMauticSubmitter(server.URL)
	result, err := submitter.Submit(context.Background(), 15, map[string]string{
		"email": "test@example.com",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success")
	}
}

func TestHTTPMauticSubmitter_ForwardHeaders(t *testing.T) {
	var gotCookie, gotUA, gotLang, gotReferer, gotXFF, gotXRI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotUA = r.Header.Get("User-Agent")
		gotLang = r.Header.Get("Accept-Language")
		gotReferer = r.Header.Get("Referer")
		gotXFF = r.Header.Get("X-Forwarded-For")
		gotXRI = r.Header.Get("X-Real-IP")
		w.Header().Set("Location", "https://example.com/ok")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	submitter := NewHTTPMauticSubmitter(server.URL)
	_, err := submitter.Submit(context.Background(), 15, map[string]string{"email": "a@b.com"}, &ForwardHeaders{
		Cookie:         "mtc_id=12345; mautic_device_id=abcdef",
		UserAgent:      "Mozilla/5.0 (test)",
		AcceptLanguage: "ja,en-US;q=0.8",
		Referer:        "https://blog.ideamans.com/foo",
		ClientIP:       "203.0.113.42",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCookie != "mtc_id=12345; mautic_device_id=abcdef" {
		t.Errorf("Cookie header: got %q", gotCookie)
	}
	if gotUA != "Mozilla/5.0 (test)" {
		t.Errorf("User-Agent: got %q", gotUA)
	}
	if gotLang != "ja,en-US;q=0.8" {
		t.Errorf("Accept-Language: got %q", gotLang)
	}
	if gotReferer != "https://blog.ideamans.com/foo" {
		t.Errorf("Referer: got %q", gotReferer)
	}
	if gotXFF != "203.0.113.42" {
		t.Errorf("X-Forwarded-For: got %q", gotXFF)
	}
	if gotXRI != "203.0.113.42" {
		t.Errorf("X-Real-IP: got %q", gotXRI)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "standard mautic errors",
			input:    "Errors%3A%3Cbr%20%2F%3E%3Col%3E%3Cli%3E%27Email%27%20is%20required.%3C%2Fli%3E%3C%2Fol%3E",
			expected: []string{"'Email' is required."},
		},
		{
			name:     "multiple errors",
			input:    "Errors%3A%3Col%3E%3Cli%3EError1%3C%2Fli%3E%3Cli%3EError2%3C%2Fli%3E%3C%2Fol%3E",
			expected: []string{"Error1", "Error2"},
		},
		{
			name:     "plain text error",
			input:    "Something+went+wrong",
			expected: []string{"Something went wrong"},
		},
		{
			name:     "html without li",
			input:    "%3Cp%3ESome+error%3C%2Fp%3E",
			expected: []string{"Some error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseErrors(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d errors, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, want := range tt.expected {
				if result[i] != want {
					t.Errorf("error[%d]: expected %q, got %q", i, want, result[i])
				}
			}
		})
	}
}

func TestHTTPMauticSubmitter_FormIDInURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := "/form/submit?formId=42"
		if r.URL.RequestURI() != expected {
			t.Errorf("expected URL %q, got %q", expected, r.URL.RequestURI())
		}
		w.Header().Set("Location", "https://example.com/ok")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	submitter := NewHTTPMauticSubmitter(server.URL)
	_, err := submitter.Submit(context.Background(), 42, map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPMauticSubmitter_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://example.com/ok")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	submitter := NewHTTPMauticSubmitter(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := submitter.Submit(ctx, 15, map[string]string{}, nil)
	if err == nil {
		t.Error("expected error for canceled context")
	}
}
