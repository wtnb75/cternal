# ヘッダベースのログインユーザー名表示 設計

## 背景・目的

cternal はリバースプロキシ(oauth2-proxy, Authelia, nginx auth_request など)の
背後で動かすことを想定している。これらのプロキシは認証済みユーザー名を
HTTPヘッダ(例: `X-Remote-User`)で渡してくる。cternal 自身は認証を行わないが、
指定されたヘッダの値を「ログインユーザー名」として画面表示・ログ記録できる
オプションを追加する。

合わせて、プロキシのログアウトURLを設定し、フロントエンドにログアウトリンクを
表示できるオプションも追加する。

## CLIフラグ

| フラグ | 環境変数 | デフォルト | 説明 |
|---|---|---|---|
| `--user-header` | `CTERNAL_USER_HEADER` | `""` | ログインユーザー名として扱うHTTPヘッダ名。未指定(空文字)の場合は機能無効 |
| `--logout-url` | `CTERNAL_LOGOUT_URL` | `""` | ログアウトリンクのURL。未指定の場合はリンクを表示しない |

既存フラグと同様に `cmd/cternal/main.go` の `serveCmd.Flags()` で定義し、
`viper.BindPFlag` で `CTERNAL_*` 環境変数とのフォールバックを設定する
(既存の `f.String("addr", ...)` 等と同じパターン)。

## バックエンド (Go)

### `internal/api/server.go` — `Config` 構造体

```go
type Config struct {
	Runtime     string `json:"runtime"`
	MaxSessions int    `json:"maxSessions"`
	BasePath    string `json:"basePath"`
	Version     string `json:"version"`
	Scrollback  int    `json:"scrollback,omitempty"`
	LogoutURL   string `json:"logoutUrl,omitempty"` // 追加

	// Not exposed via /api/v1/config (JSONには出さない).
	WebhookURLs []string `json:"-"`
	ExportURL   string   `json:"-"`
	UserHeader  string   `json:"-"` // 追加
}
```

- `LogoutURL` は静的設定値なので `Config` の通常フィールドとして
  `omitempty` で公開する。
- `UserHeader` は「どのヘッダ名を見るか」という設定値であり、ヘッダ名自体は
  公開しない(`json:"-"`)。

### `GET /api/v1/config` — `handleConfig`

`username` はリクエストごとに変わる動的な値のため、`Config` をそのまま
返さず、埋め込み構造体でレスポンスを構築する。

```go
type configResponse struct {
	Config
	Username string `json:"username,omitempty"`
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	resp := configResponse{Config: s.config}
	if s.config.UserHeader != "" {
		resp.Username = r.Header.Get(s.config.UserHeader)
	}
	writeJSON(w, http.StatusOK, resp)
}
```

- `UserHeader` が空文字(未設定)の場合、`Username` は常に空文字 →
  `omitempty` によりJSONに含まれない。
- `UserHeader` が設定されているがリクエストにヘッダが存在しない場合、
  `r.Header.Get()` は空文字を返す → 同様に `username` はJSONに含まれない。
  (フロントエンドからは「未設定」「ヘッダ設定済みだが値なし」を区別しない)

### `accessLog` ミドルウェア

ヘッダ名を引数で受け取れるようにシグネチャを変更する。

```go
func accessLog(userHeader string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		args := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if userHeader != "" {
			args = append(args, "user", r.Header.Get(userHeader))
		}
		slog.Info("http", args...)
	})
}
```

- `--user-header` 未設定時はログ出力に変更なし(`user` フィールドは出ない)。
- 設定時は常に `user` フィールドを出力する。ヘッダが付与されていない場合は
  `user=""` となり、プロキシ側の設定漏れを検出できる。

呼び出し側 (`Handler()` または `NewServer` 内のミドルウェア適用箇所) を
`accessLog(s.config.UserHeader, mux)` のように更新する。

### `internal/session.Session` — `User` フィールド

```go
type Session struct {
	// 既存フィールド ...
	User string // ログインユーザー名(UserHeader未設定時は空文字)
}
```

### `createSession` (POST /api/v1/sessions)

- `UserHeader` が設定されている場合、`r.Header.Get(s.config.UserHeader)` を
  `sess.User` に保存する。
- 「session created」ログに `user` フィールドを追加(`UserHeader` 設定時のみ)。

```go
args := []any{"id", id, "container", req.ContainerID, "mode", req.Mode}
if s.config.UserHeader != "" {
	args = append(args, "user", sess.User)
}
slog.Info("session created", args...)
```

### `deleteSession` / `EvictSession`

同様に、`UserHeader` 設定時のみ `sess.User` を `user` フィールドとして
「session deleted」「session evicted by TTL」ログに追加する。

### Webhook

今回はスコープ外。`webhook.Payload` への `user` フィールド追加は行わない。

## フロントエンド (Vue)

### `frontend/src/stores/config.ts`

```ts
const username = ref('')
const logoutUrl = ref('')

async function load() {
  // ... 既存処理
  if (typeof cfg.username === 'string') {
    username.value = cfg.username
  }
  if (typeof cfg.logoutUrl === 'string') {
    logoutUrl.value = cfg.logoutUrl
  }
}
```

- `username` が空文字の場合、ユーザー名表示・ログアウトリンクは
  どちらも表示しない。
- `logoutUrl` は `username` が空でも、設定されていれば表示する
  (ユーザー名ヘッダ未設定でもログアウトリンクだけ使いたいケースに対応)。

### 表示箇所

1. **`ContainerSidebar.vue`** — サイドバー上部に、`username` が
   設定されている場合のみユーザー名を表示。`logoutUrl` が設定されている場合は
   隣にログアウトリンク(`<a :href="logoutUrl">`)を表示。
2. **`SettingsModal.vue`** — 設定モーダル内にも同様に現在のログインユーザーと
   ログアウトリンクを表示(読み取り専用の情報セクションとして追加)。

### i18n

`frontend/src/locales/ja.json` / `en.json` に以下のキーを追加:

- `user.label`: "ユーザー" / "User"
- `user.logout`: "ログアウト" / "Log out"

## テスト方針

### Go (`internal/api`)

- `TestHandleConfig` をテーブルドリブンで拡張:
  - `UserHeader` 未設定 → `username`/`logoutUrl` がレスポンスに含まれない
  - `UserHeader` 設定済み・リクエストにヘッダあり → `username` に値が入る
  - `UserHeader` 設定済み・リクエストにヘッダなし → `username` が空(省略)
  - `LogoutURL` 設定済み → レスポンスに `logoutUrl` が含まれる
- `TestAccessLog`: `userHeader` の有無で `user` フィールドの有無/値を検証
  (ログ出力先をバッファにして検証)
- `TestCreateSession`: `UserHeader` 設定時に `sess.User` が正しく保存され、
  「session created」ログに `user` フィールドが出力されることを検証
- `TestDeleteSession` / `TestEvictSession`: 同様に `user` フィールドの検証

### フロントエンド (Vitest)

- `config.ts` store: `/api/v1/config` のレスポンスに `username`/`logoutUrl`
  が含まれる場合・含まれない場合の store 状態を検証
- `ContainerSidebar.vue` / `SettingsModal.vue`: `username` 空文字時に
  ユーザー名表示・ログアウトリンクが出ないこと、設定時に表示されることを
  `@testing-library/vue` で検証

## スコープ外

- 認証・認可機能そのものは実装しない(プロキシ側が担当)
- Webhookペイロードへの `user` フィールド追加
- 複数ヘッダの優先順位指定などの高度な設定
