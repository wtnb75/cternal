# コーディング規約

## ツール

- `find` → `fdfind`
- `grep` → `rg`
- `sed` → `sd`
- タスク実行: `task`（Taskfile.yml）—ビルド・テスト・リント等のコマンドはすべてここに定義する
- リリース: `goreleaser`（多アーキテクチャバイナリ + コンテナイメージ）

## バックエンド（Go）

- Go 1.22+、モジュール名は `github.com/wtnb75/cternal`
- フォーマッタ: `gofmt` / `goimports`
- リンタ: `golangci-lint`
- テスト: `go test ./...`、テーブルドリブンテスト推奨
- エラーは `fmt.Errorf("...: %w", err)` でラップ
- ロガー: `log/slog`（構造化ログ）
- HTTP フレームワーク: `net/http`（標準ライブラリ）+ `gorilla/websocket`
- コンテナ API クライアント: Docker SDK (`github.com/docker/docker/client`)、Podman は Docker 互換 API 使用、K8s は `k8s.io/client-go`
- CLI: `github.com/spf13/cobra`（サブコマンドなし、`serve` 相当の単一コマンド）
- 設定: CLI フラグ > 環境変数 > デフォルト値の順で解決。`github.com/spf13/viper` と cobra の pflag を統合して使用
- コンテキストは必ず引き回す（`context.Context` を第一引数に）
- goroutine リークを防ぐため、起動した goroutine は必ず終了を確認できる設計にする。複数 goroutine の管理には `golang.org/x/sync/errgroup` を使い、エラー伝播と context キャンセルを一元化する

### 接続モードの実装方針

- **exec**: `docker exec`（または `kubectl exec`）で新規プロセスを起動。セッションと WebSocket 接続は 1:1
- **attach**: `docker attach`（または `kubectl attach`）でメインプロセスに接続。コンテナ 1 つに対してセッションは 1 つとし、複数の WebSocket 接続が同一セッションを購読する fan-out 構造にする。`POST /api/v1/sessions` は get-or-create として動作させる
- **logs**: `docker logs --follow`（または `kubectl logs --follow`）で読み取り専用ストリーム。PTY 不要。WebSocket からの `input` / `resize` は受信しても捨てる

attach / logs モードの fan-out は、セッション内部に `[]chan []byte` の購読者リストを持ち、コンテナからの出力を全購読者にブロードキャストする。購読者の追加・削除もミューテックスで保護する。

### 並行処理規約

複数ブラウザが同時接続する前提で、以下を必ず守ること。

**セッションストア**
- セッションの map は `sync.RWMutex` で保護する。読み取りは `RLock`、追加・削除は `Lock`

**WebSocket 書き込み**
- gorilla/websocket の `WriteMessage` はスレッドセーフでない
- 各セッションに書き込み専用 goroutine を1つ立て、`chan []byte` 経由でメッセージを渡す。直接 `WriteMessage` を複数 goroutine から呼ばない

**録画バッファ**
- イベントスライスへの追記と、再接続時の差分読み出しは同じミューテックスで保護する

**TTL タイマーと再接続の競合**
- 切断後に起動する TTL タイマーは `time.AfterFunc` で管理し、再接続時に `timer.Stop()` でキャンセルする
- Stop の戻り値が `false`（タイマー発火済み）のときはセッション再生成として扱う

### ディレクトリ構成

```
cmd/cternal/        メインエントリポイント
internal/
  api/              HTTP/WebSocket ハンドラ
  runtime/          コンテナランタイム抽象レイヤ（Docker/Podman/K8s）
  recorder/         セッション録画・再生
  session/          セッション管理
frontend/           Vue フロントエンド（後述）
deploy/
  compose.yml       Docker Compose（Docker ランタイム向け最小構成）
  helm/             Helm チャート（K8s ランタイム向け）
    Chart.yaml
    values.yaml
    templates/
```

### デプロイファイル規約

- `compose.yml`: Docker Compose v2 形式。環境変数はすべて `environment:` キーで列挙し、`.env` ファイルへの依存は持たない
- Helm チャート: `values.yaml` のキーは `config.*`（cternal 設定）・`service.*`・`ingress.*`・`rbac.*` の4グループに整理する
- Helm テンプレートの環境変数は `values.yaml` の `config.*` から自動生成し、CLI フラグではなく環境変数経由で渡す
- K8s ランタイム使用時は `rbac.create=true` で Pod exec 権限の ServiceAccount・ClusterRole・ClusterRoleBinding を自動作成する

## フロントエンド（Vue 3 + TypeScript）

- Vue 3 Composition API（`<script setup>`）
- Vite でビルド
- 型チェック: `vue-tsc`
- フォーマッタ/リンタ: ESLint + Prettier
- ターミナルエミュレータ: `xterm.js` + `@xterm/addon-fit` + `@xterm/addon-search`
- ルーティング: `vue-router`（ハッシュモード）— セッション共有 URL に使用
- 開発時プロキシ: `vite.config.ts` の `server.proxy` で `/api` と `/ws` を Go サーバ（`http://localhost:8080`）に転送する。本番ビルドでは Go サーバが静的ファイルと API を同一オリジンで提供するためプロキシ不要
- 状態管理: Pinia
- スタイル: CSS Modules または scoped `<style>`、外部 UI コンポーネントライブラリは使わない（Vuetify, Element Plus 等）
- テーマ: CSS カスタムプロパティでダーク・ライト切り替えを実装（`prefers-color-scheme` + 手動切り替え）
- フォント: xterm.js の `fontFamily`・`fontSize` オプションで制御する。候補はモノスペースフォントに限定し、選択肢をコード内に列挙する
- i18n: `vue-i18n` で日本語・英語の切り替えをサポート
- UI 設定（テーマ・フォント・言語）は `localStorage` に保存し、Pinia ストア経由で参照する

### ディレクトリ構成

```
frontend/
  src/
    components/     再利用可能なコンポーネント
    views/          ページ単位のビュー
    stores/         Pinia ストア
    composables/    共通ロジック（useTerminal, useSession など）
    types/          TypeScript 型定義
    locales/        vue-i18n 翻訳ファイル（ja.json, en.json）
```

### 各機能の実装方針

**コピー / ペースト**
- コピー: マウス選択後に Cmd+C / Ctrl+Shift+C。`copyOnSelect` オプションは好みに応じて `localStorage` に保存する
- ペースト: Cmd+V / Ctrl+V はブラウザが処理して xterm.js に渡すため追加実装不要
- 右クリックコンテキストメニュー等でプログラマティックにクリップボードを操作する場合は `navigator.clipboard` を使う。secure context（HTTPS / localhost）以外では `document.execCommand('copy')` にフォールバックする
- Ctrl+C はターミナルへの SIGINT として扱われるため、コピーショートカットには使わない

**ターミナル内テキスト検索**
- `@xterm/addon-search` の `SearchAddon` を xterm.js に attach する
- Ctrl+F / Cmd+F のキーバインドで検索バーコンポーネントを表示し、`searchNext` / `searchPrevious` を呼ぶ

**セッション共有 URL**
- vue-router をハッシュモード（`createWebHashHistory`）で使用する
- ルート定義: `/#/sessions/:id`（ライブ）、`/#/sessions/:id/replay`（リプレイ）
- ページロード時にルートパラメータからセッション ID を取得し、自動接続する

**出力パターン通知**
- xterm.js の `onData` または録画イベントストリームを購読し、登録パターンと照合する
- マッチ時は `Notification` API（許可が必要）でブラウザ通知を送出する
- パターン設定は `localStorage` に保存し、設定画面から管理する

**複数ペイン表示**
- CSS Grid で 1/2/4 分割レイアウトを実装する。各セルに `TerminalPane` コンポーネントを配置する
- ペイン数の変更時は xterm.js の `fit()` を再実行してサイズを再計算する

**録画の自動エクスポート（サーバサイド）**
- セッション終了（DELETE または TTL 切れ）のタイミングで `CTERNAL_EXPORT_URL` に `.cast` を HTTP PUT する
- 失敗時はログに記録するが、セッション破棄自体はブロックしない

**Webhook 通知（サーバサイド）**
- セッション開始・終了・パターン検知のタイミングで `CTERNAL_WEBHOOK_URL` に HTTP POST する
- ペイロード例: `{"event": "session.start", "session_id": "...", "container": "...", "mode": "exec"}`
- 複数 URL への送信は並列で行い、個別の失敗が他に影響しないようにする

## テスト方針

### 共通

- カバレッジ目標: **90%**（`go test -coverprofile` / `vitest --coverage` で計測）
- テストはコーナーケース重視。正常系 1 件に対してエラー系・境界値を複数書く
- テストファイルに「何の挙動を保証しているか」がわかるテスト名をつける

### Go

- テーブルドリブンテスト（`[]struct{ name, input, want, wantErr }`）で網羅
- コーナーケース例: 空入力、最大長、nil コンテキスト、キャンセル済みコンテキスト、EOF、
  コンテナ未起動、WebSocket 切断中の書き込み、同時接続数上限、不正な JSON
- 外部依存（Docker API, K8s API）はインターフェース経由でモックする
- `testify/assert` と `testify/mock` を使用
- カバレッジ計測: `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`

### フロントエンド（Vue / TypeScript）

- テストフレームワーク: Vitest + Vue Test Utils
- コンポーネントテスト: `@testing-library/vue` でユーザー操作起点のテストを書く
- コーナーケース例: WebSocket 切断・再接続、空のコンテナ一覧、極端に長い出力行、
  リプレイ中のシーク（先頭・末尾・中間）、速度変更のタイミング競合
- カバレッジ計測: `vitest run --coverage`（c8 プロバイダ）

## 共通

- コメントは WHY が自明でない場合のみ記述
- テストなしでのマージ禁止
- WebSocket メッセージは `internal/api/types.go` の型定義と一致させる
- 録画の内部表現には asciicast v3 フォーマットを使用する
