package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/ideamans/mautic-form-proxy-api/client"
	"github.com/ideamans/mautic-form-proxy-api/service"
)

// clientIPFromRequest returns the best-effort originating client IP, preferring
// the leftmost entry of X-Forwarded-For (populated by an upstream proxy such as
// nginx) and falling back to RemoteAddr.
func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func forwardHeadersFromRequest(r *http.Request) *client.ForwardHeaders {
	return &client.ForwardHeaders{
		Cookie:         r.Header.Get("Cookie"),
		UserAgent:      r.Header.Get("User-Agent"),
		AcceptLanguage: r.Header.Get("Accept-Language"),
		Referer:        r.Header.Get("Referer"),
		ClientIP:       clientIPFromRequest(r),
	}
}

// NewFormSubmitHandler handles POST /api/form/{formId}
func NewFormSubmitHandler(svc service.FormService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, FormSubmitResponse{
				Success: false,
				Errors:  []string{"method not allowed"},
			})
			return
		}

		// Extract formId from URL path: /api/form/{formId}
		path := strings.TrimPrefix(r.URL.Path, "/api/form/")
		formID, err := strconv.Atoi(path)
		if err != nil || formID <= 0 {
			writeJSON(w, http.StatusBadRequest, FormSubmitResponse{
				Success: false,
				Errors:  []string{"invalid form_id in URL path"},
			})
			return
		}

		var req FormSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, FormSubmitResponse{
				Success: false,
				Errors:  []string{"invalid JSON: " + err.Error()},
			})
			return
		}

		result, err := svc.SubmitForm(r.Context(), formID, req.Fields, req.RecaptchaToken, forwardHeadersFromRequest(r))
		if err != nil {
			writeJSON(w, http.StatusBadGateway, FormSubmitResponse{
				Success: false,
				Errors:  []string{err.Error()},
			})
			return
		}

		if !result.Success {
			writeJSON(w, http.StatusUnprocessableEntity, FormSubmitResponse{
				Success: false,
				Errors:  result.Errors,
			})
			return
		}

		writeJSON(w, http.StatusOK, FormSubmitResponse{Success: true})
	}
}
