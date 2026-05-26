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
- 設定: 環境変数 + YAML（`github.com/spf13/viper`）
- コンテキストは必ず引き回す（`context.Context` を第一引数に）
- goroutine リークを防ぐため、起動した goroutine は必ず終了を確認できる設計にする

### ディレクトリ構成

```
cmd/cternal/        メインエントリポイント
internal/
  api/              HTTP/WebSocket ハンドラ
  runtime/          コンテナランタイム抽象レイヤ（Docker/Podman/K8s）
  recorder/         セッション録画・再生
  session/          セッション管理
frontend/           Vue フロントエンド（後述）
```

## フロントエンド（Vue 3 + TypeScript）

- Vue 3 Composition API（`<script setup>`）
- Vite でビルド
- 型チェック: `vue-tsc`
- フォーマッタ/リンタ: ESLint + Prettier
- ターミナルエミュレータ: `xterm.js` + `@xterm/addon-fit`
- 状態管理: Pinia
- スタイル: CSS Modules または scoped `<style>`、外部 UI ライブラリは使わない
- テーマ: CSS カスタムプロパティでダーク・ライト切り替えを実装（`prefers-color-scheme` + 手動切り替え）
- i18n: `vue-i18n` で日本語・英語の切り替えをサポート

### ディレクトリ構成

```
frontend/
  src/
    components/     再利用可能なコンポーネント
    views/          ページ単位のビュー
    stores/         Pinia ストア
    composables/    共通ロジック（useTerminal, useSession など）
    types/          TypeScript 型定義
```

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
- asciinema v2 フォーマットを録画の内部表現にも使用する
