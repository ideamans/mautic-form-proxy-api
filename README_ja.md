# mautic-form-proxy-api

MauticのフォームPOST（multipart/form-data + リダイレクト方式）をJSON APIに変換するプロキシサーバーです。
オプションでGoogle reCAPTCHA v3によるボット判定を組み込めます。

## 仕組み

Mauticのフォーム送信は `multipart/form-data` でPOSTし、結果は302リダイレクトで返されます。
このプロキシは以下の変換を行います。

1. クライアントからJSON POSTを受け取る
2. `mauticform[...]` 形式のmultipart/form-dataに変換してMauticに転送
3. リダイレクトをフォローせず、`Location` ヘッダーの `mauticError` パラメータを検査
4. 成功/エラーをJSON形式で返却

## セットアップ

### 必要要件

- Go 1.24+

### ビルド

```bash
go build -o mautic-form-proxy-api .
```

### テスト

```bash
go test ./...
```

### 起動

```bash
# 最小構成（reCAPTCHAなし）
MAUTIC_BASE_URL=https://mautic.example.com ./mautic-form-proxy-api

# reCAPTCHA有効
MAUTIC_BASE_URL=https://mautic.example.com \
RECAPTCHA_SECRET_KEY=6Le... \
RECAPTCHA_THRESHOLD=0.5 \
./mautic-form-proxy-api
```

## 環境変数

| 変数 | 説明 | デフォルト |
|------|------|-----------|
| `MAUTIC_BASE_URL` | MauticサーバーのURL | `https://mautic.ideamans.com` |
| `LISTEN_ADDR` | リッスンアドレス | `:3000` |
| `RECAPTCHA_SECRET_KEY` | Google reCAPTCHAのシークレットキー。設定するとreCAPTCHAが有効になる | 空（無効） |
| `RECAPTCHA_THRESHOLD` | reCAPTCHA v3のスコア閾値（0.0〜1.0） | `0.5` |
| `CORS_DOMAINS` | 許可するオリジンのカンマ区切りリスト（例: `https://a.com,https://b.com`）。`*` で全許可 | 空（無効） |

## API

### POST /api/form/{formId}

Mauticフォームにデータを送信します。`{formId}` にはMauticのフォームIDを指定します。

**リクエスト例:** `POST /api/form/15`

```json
{
  "fields": {
    "email": "user@example.com",
    "f_name": "山田太郎",
    "zhi_wen": "質問内容"
  },
  "recaptcha_token": "reCAPTCHAトークン（reCAPTCHA有効時は必須）"
}
```

| フィールド | 必須 | 説明 |
|-----------|------|------|
| `fields` | はい | フォームフィールドのキーバリュー |
| `recaptcha_token` | reCAPTCHA有効時 | フロントエンドで取得したreCAPTCHAトークン |

**成功レスポンス (200):**

```json
{
  "success": true
}
```

**バリデーションエラー (422):**

```json
{
  "success": false,
  "errors": [
    "'Email' is required.",
    "'Name' is required."
  ]
}
```

**reCAPTCHA拒否 (422):**

```json
{
  "success": false,
  "errors": ["reCAPTCHA verification failed"]
}
```

**上流エラー (502):**

```json
{
  "success": false,
  "errors": ["mautic: failed to submit form: ..."]
}
```

### POST /api/recaptcha/verify

reCAPTCHAトークンを単独で検証します。フォーム表示前のボット判定などに使用します。

**リクエスト:**

```json
{
  "token": "reCAPTCHAトークン"
}
```

| フィールド | 必須 | 説明 |
|-----------|------|------|
| `token` | はい | フロントエンドで取得したreCAPTCHAトークン |

**成功レスポンス (200):**

```json
{
  "success": true,
  "score": 0.9
}
```

**拒否レスポンス (403):**

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
      CORS_DOMAINS: "https://www.example.com,https://app.example.com"
      # RECAPTCHA_SECRET_KEY: "6Le..."
      # RECAPTCHA_THRESHOLD: "0.5"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:3000/.well-known/health"]
      interval: 30s
      timeout: 5s
      retries: 3
```

```bash
docker compose up -d
```

## ヘルスチェック

### GET /.well-known/health

サーバーが稼働していれば `200 OK` を返します。

```json
{"status": "ok"}
```

## アーキテクチャ

依存性注入による3層構造です。各層の境界はインターフェースで定義されており、単体テストではモックに差し替え可能です。

```
main.go           Wiring（環境変数 → 実装構築 → DI → Listen）
  │
  ├── handler/    Layer 3: HTTPハンドラー（JSON ↔ Service呼び出し）
  │     ├── handler.go       Request/Response型、writeJSON
  │     ├── form.go          POST /api/form/{formId}
  │     └── recaptcha.go     POST /api/recaptcha/verify
  │
  ├── service/    Layer 2: ビジネスロジック（reCAPTCHA判定 + Mautic送信の統合）
  │     └── service.go       FormService interface + 実装
  │
  └── client/     Layer 1: 外部システムクライアント
        ├── recaptcha.go     RecaptchaVerifier interface + Google実装
        └── mautic.go        MauticSubmitter interface + HTTP実装
```

**依存の方向:** `handler` → `service` → `client`

### インターフェース

```go
// client.RecaptchaVerifier — Google reCAPTCHA APIの抽象化
type RecaptchaVerifier interface {
    Verify(ctx context.Context, token string) (*RecaptchaResult, error)
}

// client.MauticSubmitter — Mauticフォーム送信の抽象化
type MauticSubmitter interface {
    Submit(ctx context.Context, formID int, fields map[string]string) (*MauticSubmitResult, error)
}

// service.FormService — ビジネスロジックの抽象化
type FormService interface {
    VerifyRecaptcha(ctx context.Context, token string) (*RecaptchaVerifyResult, error)
    SubmitForm(ctx context.Context, formID int, fields map[string]string, recaptchaToken string) (*FormSubmitResult, error)
    RecaptchaEnabled() bool
}
```

## フロントエンド連携例

### reCAPTCHA v3 + フォーム送信

```javascript
// 1. reCAPTCHAトークンを取得してフォーム送信
grecaptcha.ready(async () => {
  const token = await grecaptcha.execute('YOUR_SITE_KEY', { action: 'submit' })

  const response = await fetch('/api/form/15', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      fields: {
        email: 'user@example.com',
        f_name: '山田太郎'
      },
      recaptcha_token: token
    })
  })

  const result = await response.json()
  if (result.success) {
    // 送信成功
  } else {
    // エラー表示: result.errors
  }
})
```

### ボタンクリック時の事前判定

```javascript
// 2. フォーム表示ボタンにreCAPTCHA判定を仕込む
document.getElementById('show-form-btn').addEventListener('click', async () => {
  const token = await grecaptcha.execute('YOUR_SITE_KEY', { action: 'show_form' })

  const response = await fetch('/api/recaptcha/verify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token })
  })

  const result = await response.json()
  if (result.success) {
    // フォームを表示
  } else {
    // ボットと判定、フォームを表示しない
  }
})
```

## ライセンス

MIT
