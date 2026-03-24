package service

import (
	"context"
	"fmt"

	"github.com/ideamans/mautic-form-proxy-api/client"
)

type RecaptchaVerifyResult struct {
	Success bool
	Score   float64
	Errors  []string
}

type FormSubmitResult struct {
	Success bool
	Errors  []string
}

type FormService interface {
	VerifyRecaptcha(ctx context.Context, token string) (*RecaptchaVerifyResult, error)
	SubmitForm(ctx context.Context, formID int, fields map[string]string, recaptchaToken string) (*FormSubmitResult, error)
	RecaptchaEnabled() bool
}

type formService struct {
	recaptcha          client.RecaptchaVerifier
	mautic             client.MauticSubmitter
	recaptchaThreshold float64
}

func NewFormService(recaptcha client.RecaptchaVerifier, mautic client.MauticSubmitter, recaptchaThreshold float64) FormService {
	return &formService{
		recaptcha:          recaptcha,
		mautic:             mautic,
		recaptchaThreshold: recaptchaThreshold,
	}
}

func (s *formService) RecaptchaEnabled() bool {
	return s.recaptcha != nil
}

func (s *formService) VerifyRecaptcha(ctx context.Context, token string) (*RecaptchaVerifyResult, error) {
	if s.recaptcha == nil {
		return nil, fmt.Errorf("reCAPTCHA is not configured")
	}

	if token == "" {
		return &RecaptchaVerifyResult{
			Success: false,
			Errors:  []string{"token is required"},
		}, nil
	}

	result, err := s.recaptcha.Verify(ctx, token)
	if err != nil {
		return nil, err
	}

	if !result.Success {
		return &RecaptchaVerifyResult{
			Success: false,
			Errors:  result.ErrorCodes,
		}, nil
	}

	if result.Score > 0 && result.Score < s.recaptchaThreshold {
		return &RecaptchaVerifyResult{
			Success: false,
			Score:   result.Score,
			Errors:  []string{"score too low"},
		}, nil
	}

	return &RecaptchaVerifyResult{
		Success: true,
		Score:   result.Score,
	}, nil
}

func (s *formService) SubmitForm(ctx context.Context, formID int, fields map[string]string, recaptchaToken string) (*FormSubmitResult, error) {
	if s.recaptcha != nil {
		if recaptchaToken == "" {
			return &FormSubmitResult{
				Success: false,
				Errors:  []string{"recaptcha_token is required"},
			}, nil
		}

		result, err := s.recaptcha.Verify(ctx, recaptchaToken)
		if err != nil {
			return nil, err
		}

		if !result.Success {
			return &FormSubmitResult{
				Success: false,
				Errors:  []string{"reCAPTCHA verification failed"},
			}, nil
		}

		if result.Score > 0 && result.Score < s.recaptchaThreshold {
			return &FormSubmitResult{
				Success: false,
				Errors:  []string{fmt.Sprintf("reCAPTCHA score too low: %.1f", result.Score)},
			}, nil
		}
	}

	mauticResult, err := s.mautic.Submit(ctx, formID, fields)
	if err != nil {
		return nil, err
	}

	return &FormSubmitResult{
		Success: mauticResult.Success,
		Errors:  mauticResult.Errors,
	}, nil
}
