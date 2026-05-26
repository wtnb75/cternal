# cternal — 仕様書

コンテナにブラウザからアタッチしてターミナル操作し、その履歴を録画・再生・エクスポートできる Web アプリケーション。

## 対応コンテナランタイム

| ランタイム | 接続方法 |
|---|---|
| Docker | `/var/run/docker.sock`（または `DOCKER_HOST`） |
| Podman | Docker 互換 API ソケット（`PODMAN_HOST`） |
| Kubernetes | `kubeconfig` / in-cluster config、`kubectl exec` 相当 |

## 機能一覧

### 1. コンテナ一覧

- 実行中のコンテナ/Pod 一覧を表示
- ランタイム種別、コンテナ名、イメージ、ステータスを表示
- コンテナ名・ラベルによるフィルタ・検索対応

### 2. ターミナルアタッチ

- ブラウザからコンテナへ WebSocket 経由でアタッチ
- コンテナ内で動いているシェルをそのまま操作（`exec` モード）
- ターミナルサイズ変更（SIGWINCH）をリアルタイムに反映
- アタッチと同時にセッションの録画を開始する

### 3. セッション録画

- 接続中のすべての入出力をタイムスタンプ付きでメモリに保持
- asciinema v2 フォーマットを内部表現として使用
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

- 録画セッションを asciinema v2（`.cast`）形式でダウンロード
- ファイル名: `<container-name>_<ISO8601>.cast`

### 6. UI カスタマイズ

- ダーク・ライトテーマ切り替え
- ターミナルフォントの変更
- UI 表示言語の切り替え（i18n）

## アーキテクチャ

```
Browser
  │  WebSocket /ws/sessions/{id}
  │  REST        /api/v1/...
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

## WebSocket プロトコル

接続: `ws://<host>/ws/sessions/{session_id}`

### メッセージ形式（JSON）

**クライアント → サーバ**

```jsonc
// 入力
{ "type": "input", "data": "<base64>" }

// ターミナルリサイズ
{ "type": "resize", "cols": 220, "rows": 50 }

// ハートビート
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

## REST API

| メソッド | パス | 説明 |
|---|---|---|
| GET | `/api/v1/containers` | コンテナ一覧 |
| POST | `/api/v1/sessions` | セッション作成（コンテナ指定） |
| GET | `/api/v1/sessions` | セッション一覧 |
| GET | `/api/v1/sessions/{id}` | セッション情報 |
| DELETE | `/api/v1/sessions/{id}` | セッション切断・破棄 |
| GET | `/api/v1/sessions/{id}/cast` | asciinema `.cast` ダウンロード |
| GET | `/api/v1/sessions/{id}/events` | 録画イベント JSON 配列（リプレイ用） |

### POST /api/v1/sessions リクエスト

```jsonc
{
  "runtime": "docker",        // "docker" | "podman" | "k8s"
  "container": "my-container",
  "shell": "/bin/bash",       // 省略時はコンテナデフォルトシェル
  "cols": 220,
  "rows": 50
}
```

## asciinema v2 フォーマット

```
{"version": 2, "width": 220, "height": 50, "timestamp": 1716000000, "title": "my-container"}
[0.0, "o", "[2J"]
[1.23, "o", "root@abc123:/#"]
[2.10, "i", "ls\r"]
...
```

- 1 行目: JSON ヘッダ
- 2 行目以降: `[秒数, タイプ, データ]`
  - タイプ: `"o"` 出力, `"i"` 入力, `"r"` リサイズ（`"COLSxROWS"`）

## 設定（環境変数）

| 変数 | デフォルト | 説明 |
|---|---|---|
| `CTERNAL_ADDR` | `:8080` | リッスンアドレス |
| `CTERNAL_RUNTIME` | `docker` | デフォルトランタイム |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker ソケット |
| `PODMAN_HOST` | — | Podman ソケット（未指定時は Docker と同じ） |
| `KUBECONFIG` | `~/.kube/config` | K8s 設定ファイル |
| `CTERNAL_SESSION_STORE` | `memory` | `memory` / `file` |
| `CTERNAL_SESSION_DIR` | `/tmp/cternal` | `file` 時の保存先 |
| `CTERNAL_SCROLLBACK` | `10000` | スクロールバック行数 |

## 非機能要件

- 単一バイナリで動作（フロントエンドは `go:embed` で同梱）
- コンテナ 1 つあたりの追加メモリ: 録画イベント分のみ（目安 < 50 MB/h）
- WebSocket 切断時にセッションは保持し、再接続で録画の続きを再開できる
- 認証: 本バージョンは対象外（ネットワーク境界で保護する前提）

## リリース

- `goreleaser` でリリースバイナリを生成
- 対象アーキテクチャ: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- コンテナイメージも同時にビルド・公開（`ghcr.io/wtnb75/cternal`）
