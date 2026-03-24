package client

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type MauticSubmitResult struct {
	Success bool
	Errors  []string
}

type MauticSubmitter interface {
	Submit(ctx context.Context, formID int, fields map[string]string) (*MauticSubmitResult, error)
}

type HTTPMauticSubmitter struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPMauticSubmitter(baseURL string) *HTTPMauticSubmitter {
	return &HTTPMauticSubmitter{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *HTTPMauticSubmitter) Submit(ctx context.Context, formID int, fields map[string]string) (*MauticSubmitResult, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for key, val := range fields {
		_ = writer.WriteField(fmt.Sprintf("mauticform[%s]", key), val)
	}
	_ = writer.WriteField("mauticform[formId]", fmt.Sprintf("%d", formID))
	_ = writer.WriteField("mauticform[submit]", "1")
	_ = writer.WriteField("mauticform[return]", "https://mautic-form-proxy.invalid/callback")
	writer.Close()

	submitURL := fmt.Sprintf("%s/form/submit?formId=%d", s.baseURL, formID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, submitURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("mautic: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mautic: failed to submit form: %w", err)
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("mautic: unexpected response status %d with no redirect", resp.StatusCode)
	}

	parsed, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("mautic: failed to parse redirect URL: %w", err)
	}

	mauticError := parsed.Query().Get("mauticError")
	if mauticError != "" {
		return &MauticSubmitResult{
			Success: false,
			Errors:  parseErrors(mauticError),
		}, nil
	}

	return &MauticSubmitResult{Success: true}, nil
}

var liRegexp = regexp.MustCompile(`<li>(.*?)</li>`)
var htmlTagRegexp = regexp.MustCompile(`<[^>]*>`)

func parseErrors(mauticError string) []string {
	decoded, err := url.QueryUnescape(mauticError)
	if err != nil {
		return []string{mauticError}
	}
	decoded = html.UnescapeString(decoded)

	matches := liRegexp.FindAllStringSubmatch(decoded, -1)
	if len(matches) == 0 {
		stripped := htmlTagRegexp.ReplaceAllString(decoded, "")
		stripped = strings.TrimSpace(stripped)
		if stripped != "" {
			return []string{stripped}
		}
		return []string{decoded}
	}

	var errors []string
	for _, m := range matches {
		text := strings.TrimSpace(m[1])
		if text != "" {
			errors = append(errors, text)
		}
	}
	return errors
}
