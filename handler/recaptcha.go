package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ideamans/mautic-form-proxy-api/service"
)

func NewRecaptchaVerifyHandler(svc service.FormService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, RecaptchaVerifyResponse{
				Success: false,
				Errors:  []string{"method not allowed"},
			})
			return
		}

		if !svc.RecaptchaEnabled() {
			writeJSON(w, http.StatusInternalServerError, RecaptchaVerifyResponse{
				Success: false,
				Errors:  []string{"reCAPTCHA is not configured"},
			})
			return
		}

		var req RecaptchaVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, RecaptchaVerifyResponse{
				Success: false,
				Errors:  []string{"invalid JSON: " + err.Error()},
			})
			return
		}

		result, err := svc.VerifyRecaptcha(r.Context(), req.Token)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, RecaptchaVerifyResponse{
				Success: false,
				Errors:  []string{err.Error()},
			})
			return
		}

		if !result.Success {
			writeJSON(w, http.StatusForbidden, RecaptchaVerifyResponse{
				Success: false,
				Score:   result.Score,
				Errors:  result.Errors,
			})
			return
		}

		writeJSON(w, http.StatusOK, RecaptchaVerifyResponse{
			Success: true,
			Score:   result.Score,
		})
	}
}
