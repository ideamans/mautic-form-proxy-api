# mautic-form-proxy-api

Mauticインスタンスの前段に配置するリバースプロキシです。`/_form-proxy-api/` 配下に小さなJSONフォーム送信APIを公開し、リバースプロキシ先が設定されていれば、それ以外のパスはそのままMauticへ透過転送します。以下のような運用を想定しています。

```
https-endpoint  →  このコンテナ  →  Mautic本体 (MAUTIC_UPSTREAM_URL)
```

`MAUTIC_BASE_URL`（フォーム送信APIの送信先）と `MAUTIC_UPSTREAM_URL`（リバースプロキシ先）は意図的に分離しています。通常 `MAUTIC_UPSTREAM_URL` には **ローカル** の Mautic（例: 同一Dockerネットワーク上の `http://mautic:80`）を指定し、`MAUTIC_BASE_URL` はその公開URLを指定します。両方を同じ公開URLにするとこのプロキシ自身に向かって無限ループするので注意してください。

JSON API側ではオプションでGoogle reCAPTCHA v3によるボット判定を組み込めます。

## 仕組み

Mauticのフォーム送信は `multipart/form-data` でPOSTし、結果は302リダイレクトで返されます。
このコンテナは次の2つを同時に担います。

1. `/_form-proxy-api/` 配下にフォーム送信・reCAPTCHA検証・ヘルスチェック用のJSON APIを公開
   - クライアントからJSON POSTを受け取る
   - `mauticform[...]` 形式のmultipart/form-dataに変換してMauticに転送
   - リダイレクトをフォローせず、`Location` ヘッダーの `mauticError` パラメータを検査
   - 成功/エラーをJSON形式で返却
2. **それ以外のすべてのパス**を `MAUTIC_UPSTREAM_URL` で指定したMauticへ透過的にリバースプロキシ。トラッキングピクセル、ランディングページ、アセット、`mtracking.gif`、`/form/submit` など、Mautic本体宛のリクエストはそのまま通します。`MAUTIC_UPSTREAM_URL` が未設定の場合はリバースプロキシを起動せず、`/_form-proxy-api/` 以外のパスは 404 を返します。

リバースプロキシを有効にすると、Mauticサイト本体とフォーム送信JSON APIを同一の公開ホスト名で提供でき、Mauticのトラッキング Cookie（`mautic_device_id` / `mtc_id`）をファーストパーティのまま扱えます。

### Mauticコンタクトの追跡連携

Mauticの匿名ビジターtrackingをプロキシ越しでも維持するため、ブラウザからのリクエストに含まれる以下のヘッダーをMauticの `/form/submit` にそのまま転送します。

- `Cookie`（`mautic_device_id` / `mtc_id` などのトラッキングCookieを含み、既存の匿名コンタクトに送信内容を紐付け）
- `User-Agent`
- `Accept-Language`
- `Referer`
- `X-Forwarded-For` / `X-Real-IP`（元クライアントのIPを追記）

これが無いとすべての送信がプロキシサーバーからのものに見えてしまい、匿名コンタクトが孤立して属性情報が失われます。本プロキシは Mautic サイト本体と同一ホスト名で提供される運用を想定しているため、JSON API への `fetch` に対しても Mautic のトラッキング Cookie は自動的にファーストパーティ Cookie として送信されます（クロスオリジン設定は不要です）。

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
| `MAUTIC_BASE_URL` | Mauticの公開URL。フォーム送信 JSON API（`/form/submit`）の送信先として使われる。エンドユーザーがブラウザで開くのと同じURL | `https://mautic.ideamans.com` |
| `MAUTIC_UPSTREAM_URL` | `/_form-proxy-api/` 以外の全リクエストのリバースプロキシ先。**ローカル** の Mautic（例: 同一Dockerネットワーク上の隣のコンテナ `http://mautic:80`）を指定する。公開URL（このコンテナ自身のURL）を指定すると無限ループするので不可。未設定の場合はリバースプロキシを起動せず、`/_form-proxy-api/` 以外のパスは 404 | 空（無効） |
| `LISTEN_ADDR` | リッスンアドレス | `:3000` |
| `RECAPTCHA_SECRET_KEY` | Google reCAPTCHAのシークレットキー。設定するとreCAPTCHAが有効になる | 空（無効） |
| `RECAPTCHA_THRESHOLD` | reCAPTCHA v3のスコア閾値（0.0〜1.0） | `0.5` |
| `CORS_DOMAINS` | 許可するオリジンのカンマ区切りリスト（例: `https://a.com,https://b.com`）。`*` で全許可 | 空（無効） |
| `CORS_ALLOW_LOCALHOST` | `true` にするとローカル開発向けに `http://localhost:<port>` / `http://127.0.0.1:<port>` からのオリジンを全て許可 | `false` |

CORSを有効にすると、`Access-Control-Allow-Credentials: true` を常に付与します。これにより `fetch(..., { credentials: 'include' })` で送信した MauticのトラッキングCookieがプロキシを通じて転送されます。

## API

すべてのJSON APIエンドポイントは `/_form-proxy-api/` プレフィックス配下に配置されています。このプレフィックスで始まらないリクエストは、`MAUTIC_UPSTREAM_URL` が設定されていればMautic本体にリバースプロキシされ、未設定であれば 404 が返ります。

### POST /_form-proxy-api/form/{formId}

Mauticフォームにデータを送信します。`{formId}` にはMauticのフォームIDを指定します。

**リクエスト例:** `POST /_form-proxy-api/form/15`

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

### POST /_form-proxy-api/recaptcha/verify

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
      MAUTIC_UPSTREAM_URL: http://mautic:80 # 同一Dockerネットワーク上のMauticコンテナ
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

## ヘルスチェック

### GET /_form-proxy-api/health

サーバーが稼働していれば `200 OK` を返します。

```json
{"status": "ok"}
```

## アーキテクチャ

依存性注入による3層構造です。各層の境界はインターフェースで定義されており、単体テストではモックに差し替え可能です。

```
main.go           Wiring（環境変数 → 実装構築 → DI → Listen）
  │
  ├── handler/    Layer 3: HTTPハンドラー + リバースプロキシ
  │     ├── handler.go       Request/Response型、パス定数、writeJSON
  │     ├── form.go          POST /_form-proxy-api/form/{formId}
  │     ├── recaptcha.go     POST /_form-proxy-api/recaptcha/verify
  │     ├── health.go        GET  /_form-proxy-api/health
  │     ├── proxy.go         MAUTIC_UPSTREAM_URLへのリバースプロキシ（その他すべてのパス）
  │     └── cors.go          CORSミドルウェア（/_form-proxy-api/* のみに適用）
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

  const response = await fetch('/_form-proxy-api/form/15', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include', // MauticのトラッキングCookieを送信
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

  const response = await fetch('/_form-proxy-api/recaptcha/verify', {
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
