package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ideamans/mautic-form-api-proxy/service"
)

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

		result, err := svc.SubmitForm(r.Context(), formID, req.Fields, req.RecaptchaToken)
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
