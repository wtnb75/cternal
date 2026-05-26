# cternal — 仕様書

コンテナにブラウザからアタッチしてターミナル操作し、その履歴を録画・再生・エクスポートできる Web アプリケーション。

## 対応コンテナランタイム

| ランタイム | 接続方法 |
|---|---|
| Docker | `/var/run/docker.sock`（または `DOCKER_HOST`）。`ssh://user@host` でリモート接続も可 |
| Podman | Docker 互換 API ソケット（`PODMAN_HOST`）。`ssh://` によるリモート接続は Podman REST API の対応状況に依存 |
| Kubernetes | `kubeconfig` / in-cluster config、`kubectl exec` 相当 |

## 機能一覧

### 1. コンテナ一覧

- 実行中のコンテナ/Pod 一覧を表示
- ランタイム種別、コンテナ名、イメージ、ステータスを表示
- コンテナ名・ラベルによるフィルタ・検索対応

### 2. ターミナル接続

ブラウザからコンテナへ WebSocket 経由で接続する。接続モードは3種類。

| モード | 相当コマンド | 入力 | プロセス |
|---|---|---|---|
| `exec` | `docker exec` | あり | 新規シェルを起動（セッションごと独立） |
| `attach` | `docker attach` | あり | メインプロセス（PID 1）に接続 |
| `logs` | `docker logs -f` | なし（読み取り専用） | ログストリームを購読 |

- `exec` / `attach` モード: ターミナルサイズ変更（SIGWINCH）をリアルタイムに反映
- `attach` モード: コンテナごとにセッションは1つ。複数ブラウザが同一セッションに接続して出力を共有し、いずれからも入力可能
- `logs` モード: 過去ログ＋ライブストリームを受信。入力メッセージは無視する。取得開始位置は `since` で指定し、省略時はコンテナ起動時点からの全ログ
- 接続と同時にセッションの録画を開始する

### 3. セッション録画

- 接続中のすべての入出力をタイムスタンプ付きでメモリに保持
- asciicast v3 フォーマットを内部表現として使用
- サーバ側での永続化は任意（設定で有効化）

### 4. 履歴ナビゲーション

#### 4a. スクロールバック

- 接続中でも過去の出力をスクロールして参照できる
- xterm.js のスクロールバックバッファを使用（デフォルト 10,000 行）

#### 4b. リプレイ

- 録画済みセッションを再生する専用ビュー
- タイムライン UI（シークバー）でジャンプ可能
- 再生速度を変更可能（0.5×, 1×, 2×, 5×）
- 一時停止・再開対応

### 5. asciinema エクスポート

- 録画セッションを asciicast v3（`.cast`）形式でダウンロード
- ファイル名: `<container-name>_<ISO8601>.cast`

### 6. UI カスタマイズ

- ダーク・ライトテーマ切り替え
- ターミナルフォントの変更
- UI 表示言語の切り替え（日本語 / 英語）
- 設定はすべてブラウザの `localStorage` に保存し、再訪時に復元する

### 7. ターミナル内テキスト検索

- Ctrl+F（macOS: Cmd+F）で検索バーを表示
- 前・次の一致箇所へジャンプ
- 大文字小文字の区別・正規表現のオプション
- `@xterm/addon-search` で実装

### 8. セッション共有 URL

- セッション ID を URL ハッシュに含め、直接アクセスできる
- `/#/sessions/{id}` — ライブ接続（`exec` / `attach` / `logs`）
- `/#/sessions/{id}/replay` — リプレイビュー
- `attach` モードのセッション URL を共有すると、複数人が同じセッションを参照・操作できる

### 9. 出力パターン通知

- ターミナル出力に対してパターン（文字列または正規表現）を登録できる
- パターンにマッチした行が現れたとき、ブラウザ通知（Web Notifications API）を送出する
- `exec` / `attach` / `logs` の全モードで動作
- パターンは UI から追加・削除し、`localStorage` に保存する

### 10. 複数ペイン表示

- ブラウザ内で複数のセッションを分割表示できる
- 水平・垂直の 2 分割、および 4 分割をサポート
- 各ペインは独立したセッションを持つ

### 11. 録画の自動エクスポート

- セッション終了時に `.cast` ファイルを自動的に外部ストレージへ送信する
- 対応エクスポート先: ローカルファイル（`file` モード）、HTTP PUT（任意の URL）
- `CTERNAL_EXPORT_URL` に HTTP PUT エンドポイントを設定することで有効化

### 12. Webhook 通知

- セッション開始・終了のタイミングで指定 URL に HTTP POST する
- ペイロードは JSON（セッション情報 + イベント種別）
- `CTERNAL_WEBHOOK_URL` で設定。複数 URL をカンマ区切りで指定可能

## アーキテクチャ

```
Browser
  │  WebSocket {BASE_PATH}/ws/sessions/{id}
  │  REST        {BASE_PATH}/api/v1/...
  ▼
Go HTTP Server (cternal)
  ├── api/          ルーティング・ハンドラ
  ├── session/      セッションライフサイクル管理
  ├── recorder/     イベント録画・再生エンジン
  └── runtime/      コンテナ I/O 抽象化
        ├── docker.go
        ├── podman.go   (Docker 互換 API)
        └── k8s.go
```

## CLI サブコマンド

| サブコマンド | 説明 |
|---|---|
| `serve` | HTTP サーバを起動する（デフォルト動作） |
| `play <file.cast>` | `.cast` ファイルをターミナルで再生する |
| `record <container>` | コンテナのターミナルセッションを録画してファイルに保存する |
| `version` | バージョン情報を表示する |
| `completion` | シェル補完スクリプトを出力する |

### cternal serve

現在の設定セクションで定義したフラグをすべて受け付ける。引数なしで `cternal` を実行した場合も `serve` として動作する。

### cternal play

```sh
cternal play [flags] <file.cast>
```

| フラグ | デフォルト | 説明 |
|---|---|---|
| `--speed` | `1.0` | 再生速度（0.5 / 1.0 / 2.0 / 5.0） |
| `--loop` | `false` | 末尾に達したら先頭に戻る |

サーバ不要。`internal/recorder` の再生エンジンを直接使用してターミナルに出力する。

### cternal record

```sh
cternal record [flags] <container>
```

| フラグ | デフォルト | 説明 |
|---|---|---|
| `--runtime` | `docker` | コンテナランタイム |
| `--shell` | （SHELL 環境変数 → `/bin/sh`） | 起動するシェル |
| `--output`, `-o` | `<container>_<ISO8601>.cast` | 出力ファイルパス |

サーバ不要。`internal/runtime` と `internal/recorder` を直接使用する。Ctrl+D または Ctrl+C で録画を終了してファイルに書き出す。

### cternal version

ビルド時に goreleaser の ldflags で埋め込んだバージョン・コミットハッシュ・ビルド日時を表示する。

## WebSocket プロトコル

接続: `ws://<host>{BASE_PATH}/ws/sessions/{session_id}`

`BASE_PATH` は `CTERNAL_BASE_PATH` 環境変数で設定する（デフォルトは空文字列 = ルート）。

### メッセージ形式（JSON）

**クライアント → サーバ**

```jsonc
// 入力（exec / attach のみ。logs モードでは無視）
{ "type": "input", "data": "<base64>" }

// ターミナルリサイズ（exec / attach のみ）
{ "type": "resize", "cols": 220, "rows": 50 }

// ハートビート（全モード共通）
{ "type": "ping" }
```

**サーバ → クライアント**

```jsonc
// 出力
{ "type": "output", "data": "<base64>" }

// リサイズ確認
{ "type": "resize", "cols": 220, "rows": 50 }

// エラー
{ "type": "error", "message": "..." }

// pong
{ "type": "pong" }
```

### 再接続フロー

1. クライアントは切断を検知したら、同じ `session_id` で WebSocket に再接続する
2. サーバは切断中に発生した出力をバッファに保持しており、再接続時に差分を `output` メッセージとして順次送信する
3. 差分送信が完了すると、以降はリアルタイム出力に切り替わる
4. 切断から `CTERNAL_SESSION_TTL`（デフォルト 3600 秒）を超えたセッションはサーバ側で破棄される

## REST API

すべてのパスは `CTERNAL_BASE_PATH` を先頭に付与する（例: `BASE_PATH=/cternal` なら `/cternal/api/v1/config`）。

| メソッド | パス | 説明 |
|---|---|---|
| GET | `/api/v1/config` | フロントエンド向けサーバー設定の取得 |
| GET | `/api/v1/containers` | コンテナ一覧 |
| POST | `/api/v1/sessions` | セッション作成（コンテナ指定） |
| GET | `/api/v1/sessions` | セッション一覧 |
| GET | `/api/v1/sessions/{id}` | セッション情報 |
| DELETE | `/api/v1/sessions/{id}` | セッション切断・破棄（録画データも同時に削除） |
| GET | `/api/v1/sessions/{id}/cast` | asciinema `.cast` ダウンロード |
| GET | `/api/v1/sessions/{id}/events` | 録画イベント JSON 配列（リプレイ用） |

### GET /api/v1/config レスポンス

フロントエンドが起動時に取得するサーバー管理の設定値。

```jsonc
{
  "scrollback": 10000  // xterm.js のスクロールバック行数
}
```

### GET /api/v1/containers クエリパラメータ

| パラメータ | 型 | 説明 |
|---|---|---|
| `name` | string | コンテナ名で絞り込み（部分一致） |
| `label` | string | ラベルで絞り込み（`key=value` 形式、複数指定可） |
| `runtime` | string | ランタイムで絞り込み（`docker` / `podman` / `k8s`） |

例: `/api/v1/containers?name=web&label=env%3Dprod&label=team%3Dbackend`

### GET /api/v1/containers レスポンス

```jsonc
[
  {
    "id": "abc123def456",
    "name": "my-container",
    "image": "nginx:latest",
    "status": "running",
    "runtime": "docker",      // "docker" | "podman" | "k8s"
    "labels": { "env": "prod", "team": "backend" }
  }
]
```

### POST /api/v1/sessions リクエスト

```jsonc
{
  "runtime": "docker",        // "docker" | "podman" | "k8s"
  "container": "my-container",
  "mode": "exec",             // "exec" | "attach" | "logs"（省略時は "exec"）
  "shell": "/bin/bash",       // exec のみ。省略時はコンテナの SHELL 環境変数を参照し、未設定なら /bin/sh
  "cols": 220,                // exec / attach のみ
  "rows": 50,                 // exec / attach のみ
  "since": "2024-05-18T10:00:00Z" // logs のみ。RFC3339 またはデュレーション文字列（"1h", "30m"）。省略時はコンテナ起動時点から
}
```

`attach` モードは、対象コンテナの既存セッションを返す（get-or-create）。
`logs` モードは `shell` / `cols` / `rows` を無視する。`since` は `logs` モード専用で他のモードでは無視する。

### POST /api/v1/sessions レスポンス

```jsonc
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "container": "my-container",
  "runtime": "docker",
  "mode": "exec",             // "exec" | "attach" | "logs"
  "status": "active",
  "created_at": "2024-05-18T10:00:00Z",
  "cols": 220,
  "rows": 50,
  "ws_url": "{BASE_PATH}/ws/sessions/550e8400-e29b-41d4-a716-446655440000"
}
```

### GET /api/v1/sessions レスポンス

```jsonc
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "container": "my-container",
    "runtime": "docker",
    "mode": "exec",           // "exec" | "attach" | "logs"
    "status": "active",       // "active" | "disconnected" | "closed"
    "created_at": "2024-05-18T10:00:00Z",
    "cols": 220,
    "rows": 50
  }
]
```

`GET /api/v1/sessions/{id}` は同じ構造の単一オブジェクトを返す。

## asciicast v3 フォーマット

```
{"version": 3, "width": 220, "height": 50, "timestamp": 1716000000, "title": "my-container"}
[0.0, "o", "[2J"]
[1.23, "o", "root@abc123:/#"]
[2.10, "i", "ls\r"]
...
```

- 1 行目: JSON ヘッダ
- 2 行目以降: `[秒数, タイプ, データ]`
  - `"o"`: 出力
  - `"i"`: 入力
  - `"r"`: リサイズ（データは `"COLSxROWS"` 形式）

## 設定

優先順位: **CLI フラグ > 環境変数 > デフォルト値**

| CLI フラグ | 環境変数 | デフォルト | 説明 |
|---|---|---|---|
| `--addr` | `CTERNAL_ADDR` | `:8080` | リッスンアドレス |
| `--base-path` | `CTERNAL_BASE_PATH` | `` | サブパスプレフィックス（例: `/cternal`）。末尾スラッシュなし |
| `--runtime` | `CTERNAL_RUNTIME` | `docker` | デフォルトランタイム |
| `--docker-host` | `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker ソケット。`ssh://user@host` でリモート接続も可（SSH エージェントが必要。コンテナで動かす場合は `SSH_AUTH_SOCK` のマウントが必要） |
| `--podman-host` | `PODMAN_HOST` | — | Podman ソケット（未指定時は Docker と同じ） |
| `--kubeconfig` | `KUBECONFIG` | `~/.kube/config` | K8s 設定ファイル |
| `--session-store` | `CTERNAL_SESSION_STORE` | `memory` | `memory` / `file` |
| `--session-dir` | `CTERNAL_SESSION_DIR` | `/tmp/cternal` | `file` 時の保存先 |
| `--scrollback` | `CTERNAL_SCROLLBACK` | `10000` | スクロールバック行数（`/api/v1/config` 経由でフロントエンドに配信） |
| `--session-ttl` | `CTERNAL_SESSION_TTL` | `3600` | 切断後にセッションを保持する秒数（経過後に破棄） |
| `--max-sessions` | `CTERNAL_MAX_SESSIONS` | `100` | 同時セッション数の上限。超えた場合は 503 を返す |
| `--export-url` | `CTERNAL_EXPORT_URL` | — | セッション終了時の `.cast` 自動送信先 URL（HTTP PUT） |
| `--webhook-url` | `CTERNAL_WEBHOOK_URL` | — | イベント通知先 Webhook URL（カンマ区切りで複数指定可） |
| `--log-level` | `CTERNAL_LOG_LEVEL` | `info` | ログレベル（`debug` / `info` / `warn` / `error`） |
| `--log-format` | `CTERNAL_LOG_FORMAT` | `text` | ログ形式（`text` / `json`） |
| — | `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP エクスポート先（例: `http://localhost:4318`）。未設定時は OTel 無効 |
| — | `OTEL_SERVICE_NAME` | `cternal` | テレメトリ上のサービス名 |

上記以外の OTel SDK 標準環境変数（`OTEL_TRACES_SAMPLER`、`OTEL_METRICS_EXPORTER`、`OTEL_PROPAGATORS` 等）もそのまま有効。

```sh
# 使用例
cternal --addr :9090 --base-path /cternal --session-store file --session-dir /var/lib/cternal
```

## 非機能要件

- 単一バイナリで動作（フロントエンドは `go:embed` で同梱）
- コンテナ 1 つあたりの追加メモリ: 録画イベント分のみ（目安 < 50 MB/h）
- WebSocket 切断時にセッションは保持し、再接続で録画の続きを再開できる
- 認証: 本バージョンは対象外（ネットワーク境界で保護する前提）
- 同時セッション数が `CTERNAL_MAX_SESSIONS` を超えた場合は `POST /api/v1/sessions` が 503 を返す
- 複数ブラウザからの同時アクセスに対してデータ競合が起きない設計とする（セッションストア・録画バッファは排他制御、WebSocket 書き込みはセッションごとに直列化）
- `OTEL_EXPORTER_OTLP_ENDPOINT` が設定されている場合、OpenTelemetry によるトレース・メトリクスを OTLP で送信する

## コンテナイメージ

ベースイメージには `gcr.io/distroless/static` を使用する。

- CA 証明書（`/etc/ssl/certs`）が含まれており、Webhook・エクスポート先への HTTPS 接続が追加設定なしで動作する
- シェルや不要なバイナリを含まないため攻撃面が最小
- タイムゾーンデータは Go ビルド時に `import _ "time/tzdata"` で埋め込むため、イメージ側の追加ファイルは不要

コンテナイメージに含まれるのはシングルバイナリ（`cternal`）のみ。Docker ソケット・kubeconfig・SSH エージェントソケットはすべてランタイムマウントで提供する。

## クイックスタート

### Docker Compose

`deploy/compose.yml` で Docker ランタイム向けの最小構成を提供する。

```yaml
# deploy/compose.yml
services:
  cternal:
    image: ghcr.io/wtnb75/cternal:latest
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      CTERNAL_RUNTIME: docker
```

```sh
docker compose -f deploy/compose.yml up
```

### Helm チャート

`deploy/helm/` で Kubernetes 向けの Helm チャートを提供する。
K8s ランタイムを使う場合は `docker.sock` マウント不要で、in-cluster 権限で動作する。

```sh
# ローカル試用（port-forward）
helm install cternal deploy/helm/ --set service.type=ClusterIP
kubectl port-forward svc/cternal 8080:8080

# Ingress + サブパスの例
helm install cternal deploy/helm/ \
  --set ingress.enabled=true \
  --set ingress.host=example.com \
  --set config.basePath=/cternal
```

主な values:

| キー | デフォルト | 説明 |
|---|---|---|
| `config.runtime` | `k8s` | コンテナランタイム |
| `config.basePath` | `` | サブパスプレフィックス |
| `config.sessionStore` | `memory` | セッション永続化方式 |
| `config.sessionTTL` | `3600` | セッション保持秒数 |
| `service.type` | `ClusterIP` | Service タイプ |
| `ingress.enabled` | `false` | Ingress の有効化 |
| `ingress.host` | `` | Ingress ホスト名 |
| `rbac.create` | `true` | Pod exec 用 RBAC の自動作成 |

## リリース

- `goreleaser` でリリースバイナリを生成
- バイナリ対象: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- コンテナイメージ対象: `linux/amd64`, `linux/arm64`（Linux カーネルが必要なため darwin は対象外）
- コンテナイメージは `ghcr.io/wtnb75/cternal` に公開
