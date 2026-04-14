# mautic-form-proxy-api

A reverse proxy that sits in front of a Mautic instance and exposes a small JSON
form-submission API under `/_form-proxy-api/`. When a reverse-proxy upstream is
configured, every other request is transparently proxied to it, so the container
can be dropped in as:

```
https-endpoint  →  this container  →  Mautic (MAUTIC_UPSTREAM_URL)
```

`MAUTIC_BASE_URL` (the form-submission API target) and `MAUTIC_UPSTREAM_URL` (the
reverse-proxy destination) are intentionally separate. Typically `MAUTIC_UPSTREAM_URL`
points at a **local** Mautic (e.g. `http://mautic:80` in the same Docker network),
while `MAUTIC_BASE_URL` points at its public URL. Pointing both at the same public
URL would loop back into this same proxy.

Optionally integrates Google reCAPTCHA v3 for bot detection on the JSON API.

## How It Works

Mautic form submissions use `multipart/form-data` POST and return results via 302 redirects.
This proxy does two things:

1. Exposes a JSON API under `/_form-proxy-api/` for form submission, reCAPTCHA verification, and health checking.
   - Receives a JSON POST from the client
   - Converts it to `mauticform[...]` multipart/form-data and forwards it to Mautic
   - Inspects the `Location` header for a `mauticError` query parameter (without following the redirect)
   - Returns success or errors as JSON
2. Reverse-proxies **every other path** to `MAUTIC_UPSTREAM_URL` (when set). Tracking pixels, landing pages, assets, `mtracking.gif`, `/form/submit`, everything — the container is transparent for those requests. If `MAUTIC_UPSTREAM_URL` is not set, the reverse proxy is disabled and unknown paths return 404; in that case the container only serves the JSON API under `/_form-proxy-api/`.

With the reverse proxy enabled, a single public hostname can serve both the Mautic site itself and the form-submission JSON API from the same origin, which keeps Mautic tracking cookies (`mautic_device_id` / `mtc_id`) first-party.

### Mautic Contact Identity

To preserve Mautic's anonymous visitor tracking across the proxy, the following headers from the browser request are forwarded to Mautic's `/form/submit`:

- `Cookie` (carries `mautic_device_id` / `mtc_id` so the submission attaches to the existing tracked contact)
- `User-Agent`
- `Accept-Language`
- `Referer`
- `X-Forwarded-For` / `X-Real-IP` (the original client IP is appended)

Without this, all submissions would appear to come from the proxy server itself, orphaning anonymous contacts and breaking attribution. Because the JSON API lives under the same hostname as the Mautic site (the reverse proxy passthrough), Mautic's tracking cookies are automatically first-party and get sent along with API requests without any cross-origin gymnastics.

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
| `MAUTIC_BASE_URL` | Public Mautic server URL used as the target of the form-submission JSON API (`/form/submit`). This URL is what end users would also type into a browser | `https://mautic.ideamans.com` |
| `MAUTIC_UPSTREAM_URL` | Reverse-proxy destination for every non-`/_form-proxy-api/` request. Should point at a **local** Mautic (e.g. `http://mautic:80`, a sibling container) — **not** back at the container's own public URL, which would loop. If unset, the reverse proxy is disabled and non-API paths return 404 | empty (disabled) |
| `LISTEN_ADDR` | Listen address | `:3000` |
| `RECAPTCHA_SECRET_KEY` | Google reCAPTCHA secret key. Enables reCAPTCHA when set | empty (disabled) |
| `RECAPTCHA_THRESHOLD` | reCAPTCHA v3 score threshold (0.0-1.0) | `0.5` |
| `CORS_DOMAINS` | Comma-separated list of allowed origins (e.g. `https://a.com,https://b.com`). Use `*` to allow all | empty (disabled) |
| `CORS_ALLOW_LOCALHOST` | When `true`, allow any `http://localhost:<port>` / `http://127.0.0.1:<port>` origin (for local development) | `false` |

When CORS is enabled, `Access-Control-Allow-Credentials: true` is always sent so that clients using `fetch(..., { credentials: 'include' })` can carry Mautic tracking cookies through the proxy.

## API

All JSON API endpoints live under the `/_form-proxy-api/` path prefix. Any path that does **not** start with this prefix is reverse-proxied verbatim to `MAUTIC_UPSTREAM_URL` (or returns 404 if no upstream is configured).

### POST /_form-proxy-api/form/{formId}

Submits data to a Mautic form. Specify the Mautic form ID in the URL path.

**Example:** `POST /_form-proxy-api/form/15`

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

### POST /_form-proxy-api/recaptcha/verify

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

## Docker

### docker run

```bash
docker run -d -p 3000:3000 \
  -e MAUTIC_BASE_URL=https://mautic.example.com \
  -e MAUTIC_UPSTREAM_URL=http://mautic:80 \
  -e CORS_DOMAINS=https://www.example.com,https://app.example.com \
  ideamans/mautic-form-proxy-api:latest
```

### docker compose

```yaml
services:
  mautic-form-proxy:
    image: ideamans/mautic-form-proxy-api:latest
    ports:
      - "3000:3000"
    environment:
      MAUTIC_BASE_URL: https://mautic.example.com
      MAUTIC_UPSTREAM_URL: http://mautic:80 # sibling Mautic container on the Docker network
      CORS_DOMAINS: "https://www.example.com,https://app.example.com"
      # RECAPTCHA_SECRET_KEY: "6Le..."
      # RECAPTCHA_THRESHOLD: "0.5"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:3000/_form-proxy-api/health"]
      interval: 30s
      timeout: 5s
      retries: 3
```

```bash
docker compose up -d
```

## Health Check

### GET /_form-proxy-api/health

Returns `200 OK` when the server is running.

```json
{"status": "ok"}
```

## Architecture

Three-layer dependency injection architecture. Each layer boundary is defined by interfaces, allowing unit tests to swap in mocks.

```
main.go           Wiring (env vars -> build implementations -> DI -> Listen)
  |
  +-- handler/    Layer 3: HTTP handlers + reverse proxy
  |     +-- handler.go       Request/Response types, path constants, writeJSON
  |     +-- form.go          POST /_form-proxy-api/form/{formId}
  |     +-- recaptcha.go     POST /_form-proxy-api/recaptcha/verify
  |     +-- health.go        GET  /_form-proxy-api/health
  |     +-- proxy.go         Reverse proxy to MAUTIC_UPSTREAM_URL for all other paths
  |     +-- cors.go          CORS middleware (applied only to /_form-proxy-api/*)
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

  const response = await fetch('/_form-proxy-api/form/15', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include', // send Mautic tracking cookies
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

  const response = await fetch('/_form-proxy-api/recaptcha/verify', {
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
