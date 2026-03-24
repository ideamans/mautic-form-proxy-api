# mautic-form-proxy-api

A proxy server that converts Mautic form submissions (multipart/form-data + redirect) into a JSON API.
Optionally integrates Google reCAPTCHA v3 for bot detection.

## How It Works

Mautic form submissions use `multipart/form-data` POST and return results via 302 redirects.
This proxy handles the translation:

1. Receives a JSON POST from the client
2. Converts it to `mauticform[...]` multipart/form-data and forwards it to Mautic
3. Inspects the `Location` header for a `mauticError` query parameter (without following the redirect)
4. Returns success or errors as JSON

## Setup

### Requirements

- Go 1.24+

### Build

```bash
go build -o mautic-form-proxy-api .
```

### Test

```bash
go test ./...
```

### Run

```bash
# Minimal (without reCAPTCHA)
MAUTIC_BASE_URL=https://mautic.example.com ./mautic-form-proxy-api

# With reCAPTCHA enabled
MAUTIC_BASE_URL=https://mautic.example.com \
RECAPTCHA_SECRET_KEY=6Le... \
RECAPTCHA_THRESHOLD=0.5 \
./mautic-form-proxy-api
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `MAUTIC_BASE_URL` | Mautic server URL | `https://mautic.ideamans.com` |
| `LISTEN_ADDR` | Listen address | `:3000` |
| `RECAPTCHA_SECRET_KEY` | Google reCAPTCHA secret key. Enables reCAPTCHA when set | empty (disabled) |
| `RECAPTCHA_THRESHOLD` | reCAPTCHA v3 score threshold (0.0-1.0) | `0.5` |
| `CORS_DOMAINS` | Comma-separated list of allowed origins (e.g. `https://a.com,https://b.com`). Use `*` to allow all | empty (disabled) |

## API

### POST /api/form/{formId}

Submits data to a Mautic form. Specify the Mautic form ID in the URL path.

**Example:** `POST /api/form/15`

```json
{
  "fields": {
    "email": "user@example.com",
    "f_name": "John Doe",
    "zhi_wen": "Question"
  },
  "recaptcha_token": "token (required when reCAPTCHA is enabled)"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `fields` | Yes | Key-value pairs of form fields |
| `recaptcha_token` | When reCAPTCHA is enabled | reCAPTCHA token obtained on the frontend |

**Success (200):**

```json
{
  "success": true
}
```

**Validation Error (422):**

```json
{
  "success": false,
  "errors": [
    "'Email' is required.",
    "'Name' is required."
  ]
}
```

**reCAPTCHA Rejected (422):**

```json
{
  "success": false,
  "errors": ["reCAPTCHA verification failed"]
}
```

**Upstream Error (502):**

```json
{
  "success": false,
  "errors": ["mautic: failed to submit form: ..."]
}
```

### POST /api/recaptcha/verify

Verifies a reCAPTCHA token independently. Useful for bot detection before showing a form.

**Request:**

```json
{
  "token": "reCAPTCHA token"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `token` | Yes | reCAPTCHA token obtained on the frontend |

**Success (200):**

```json
{
  "success": true,
  "score": 0.9
}
```

**Rejected (403):**

```json
{
  "success": false,
  "score": 0.3,
  "errors": ["score too low"]
}
```

## Architecture

Three-layer dependency injection architecture. Each layer boundary is defined by interfaces, allowing unit tests to swap in mocks.

```
main.go           Wiring (env vars -> build implementations -> DI -> Listen)
  |
  +-- handler/    Layer 3: HTTP handlers (JSON <-> Service calls)
  |     +-- handler.go       Request/Response types, writeJSON
  |     +-- form.go          POST /api/form/{formId}
  |     +-- recaptcha.go     POST /api/recaptcha/verify
  |
  +-- service/    Layer 2: Business logic (reCAPTCHA + Mautic orchestration)
  |     +-- service.go       FormService interface + implementation
  |
  +-- client/     Layer 1: External system clients
        +-- recaptcha.go     RecaptchaVerifier interface + Google implementation
        +-- mautic.go        MauticSubmitter interface + HTTP implementation
```

**Dependency direction:** `handler` -> `service` -> `client`

### Interfaces

```go
// client.RecaptchaVerifier - abstraction for Google reCAPTCHA API
type RecaptchaVerifier interface {
    Verify(ctx context.Context, token string) (*RecaptchaResult, error)
}

// client.MauticSubmitter - abstraction for Mautic form submission
type MauticSubmitter interface {
    Submit(ctx context.Context, formID int, fields map[string]string) (*MauticSubmitResult, error)
}

// service.FormService - abstraction for business logic
type FormService interface {
    VerifyRecaptcha(ctx context.Context, token string) (*RecaptchaVerifyResult, error)
    SubmitForm(ctx context.Context, formID int, fields map[string]string, recaptchaToken string) (*FormSubmitResult, error)
    RecaptchaEnabled() bool
}
```

## Frontend Examples

### reCAPTCHA v3 + Form Submission

```javascript
grecaptcha.ready(async () => {
  const token = await grecaptcha.execute('YOUR_SITE_KEY', { action: 'submit' })

  const response = await fetch('/api/form/15', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      fields: {
        email: 'user@example.com',
        f_name: 'John Doe'
      },
      recaptcha_token: token
    })
  })

  const result = await response.json()
  if (result.success) {
    // Success
  } else {
    // Show errors: result.errors
  }
})
```

### Pre-check on Button Click

```javascript
document.getElementById('show-form-btn').addEventListener('click', async () => {
  const token = await grecaptcha.execute('YOUR_SITE_KEY', { action: 'show_form' })

  const response = await fetch('/api/recaptcha/verify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token })
  })

  const result = await response.json()
  if (result.success) {
    // Show the form
  } else {
    // Detected as bot, do not show the form
  }
})
```

## License

MIT
