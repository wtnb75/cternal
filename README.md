# cternal

Web-based container terminal with session recording. Attach to Docker, Podman, or Kubernetes containers from a browser, record sessions as [asciicast](https://docs.asciinema.org/manual/asciicast/v2/) files, and replay them later.

## Features

- **Browser terminal** — xterm.js-based terminal for exec / attach / logs modes
- **Session recording** — automatic asciicast v2 recording; download or auto-export via HTTP PUT
- **Replay** — in-browser playback with seek and speed control
- **Multi-pane** — 1 / 2 / 4 split layouts
- **Multi-runtime** — Docker, Podman (Docker-compatible API), Kubernetes
- **Webhook** — HTTP POST on session start / end
- **OpenTelemetry** — traces and metrics via OTLP (no-op when unconfigured)
- **CLI tools** — `play`, `record`, `logscast` subcommands for offline use

## Quick start

```sh
docker compose -f deploy/compose.yml up
```

Open http://localhost:8081 — the Grafana observability stack is at http://localhost:3000 (admin / admin).

## CLI

```sh
# Serve (default when no subcommand is given)
cternal serve --addr :8080 --runtime docker

# Play back a recording
cternal play session.cast
cternal play --speed 2.0 --loop session.cast

# Record a container session
cternal record <container-id> --output session.cast

# Convert container logs to asciicast
cternal logscast <container-id> --output logs.cast --since 1h
cternal logscast <container-id> --follow --idle-time-limit 2.0
```

## Configuration

All flags can also be set via `CTERNAL_*` environment variables.

| Flag | Env var | Default | Description |
|---|---|---|---|
| `--addr` | `CTERNAL_ADDR` | `:3000` | Listen address |
| `--runtime` | `CTERNAL_RUNTIME` | `docker` | Runtime: `docker`, `podman`, `k8s` |
| `--max-sessions` | `CTERNAL_MAX_SESSIONS` | `100` | Max concurrent sessions |
| `--session-ttl` | `CTERNAL_SESSION_TTL` | `3600` | Session TTL (seconds) after disconnect |
| `--scrollback` | `CTERNAL_SCROLLBACK` | `5000` | Terminal scrollback lines |
| `--webhook-url` | `CTERNAL_WEBHOOK_URL` | — | Webhook URL(s) for session events |
| `--export-url` | `CTERNAL_EXPORT_URL` | — | HTTP PUT endpoint for auto `.cast` export |
| `--podman-host` | `CTERNAL_PODMAN_HOST` | — | Podman socket URL |
| `--kubeconfig` | `CTERNAL_KUBECONFIG` | — | Path to kubeconfig |
| `--log-level` | `CTERNAL_LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `--log-format` | `CTERNAL_LOG_FORMAT` | `text` | `text` / `json` |

Set `OTEL_EXPORTER_OTLP_ENDPOINT` (e.g. `host:4317`) to enable OpenTelemetry export.

## Development

```sh
# Install dependencies
task install

# Start Go server + Vite dev server
task dev

# Run all tests
task test

# Lint
task lint

# Local release snapshot (binaries only, no Docker push)
task release:dry
```

Requires Go 1.22+, Node.js 22+, pnpm, and [Task](https://taskfile.dev).

## License

MIT — see [LICENSE](LICENSE).
