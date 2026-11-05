# Configuration Guide

PreviewForge can be configured through environment variables, CLI flags, or an optional repository-level configuration file.

## 1. Environment Variables and CLI Flags

| Flag | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- |
| `--domain` | `PREVIEWFORGE_DOMAIN` | `localhost` | Base wildcard domain (e.g. `preview.example.com`). Subdomains take the form `pr-{number}.preview.example.com`. |
| `--port` | `PREVIEWFORGE_PORT`, `PORT` | `9850` | HTTP port where PreviewForge listens for webhooks, web studio traffic, and standalone proxy requests. |
| `--proxy-mode` | `PREVIEWFORGE_PROXY_MODE` | `standalone` | Reverse proxy routing mechanism: `standalone` (built-in Go proxy) or `caddy` (Caddy Admin API). |
| `--caddy-url` | `PREVIEWFORGE_CADDY_URL` | `http://127.0.0.1:2019` | Endpoint for Caddy Admin API when running in `caddy` mode. |
| `--data-dir` | `PREVIEWFORGE_DATA_DIR` | `./data` | Directory where SQLite database and cloned git workspaces reside. |
| `--github-token`| `GITHUB_TOKEN` | `""` | GitHub Personal Access Token or App token used to set commit statuses and publish sticky comments. |
| `--webhook-secret`| `WEBHOOK_SECRET` | `""` | Secret key used to verify incoming GitHub webhook HMAC-SHA256 signatures (`X-Hub-Signature-256`). |
| `--port-start` | `PREVIEWFORGE_PORT_START` | `31000` | Starting port number for ephemeral container allocations. |
| `--port-end` | `PREVIEWFORGE_PORT_END` | `35000` | Ending port number for ephemeral container allocations. |
| `--idle-timeout` | `PREVIEWFORGE_IDLE_TIMEOUT` | `1440` | Idle timeout in minutes before an unclosed preview stack is automatically reaped (default 24 hours). |
| `--require-label`| `PREVIEWFORGE_REQUIRE_LABEL`| `""` | If set, only PRs containing this label (e.g. `preview`) will be deployed. |
| `--allow-forks` | `PREVIEWFORGE_ALLOW_FORKS` | `false` | When false, PRs from external forks are rejected unless opened by the repository owner. |

## 2. In-Repository Configuration (`.previewforge.yml`)

Add an optional `.previewforge.yml` file to the root of your target repository to customize build behavior:

```yaml
# Path to compose file relative to repo root (default: docker-compose.yml or compose.yaml)
compose_file: "docker-compose.yml"

# Primary service to expose via the preview subdomain (default: auto-detected)
service: "web"

# Container internal port (default: 3000)
port: 3000

# Path polled during readiness verification (default: /)
health_path: "/api/health"

# Environment variables injected into preview containers
env:
  NODE_ENV: "production"
  DATABASE_URL: "postgres://user:pass@db:5432/app"
  ENABLE_BETA_FEATURES: "true"

# Maximum build duration before aborting (in seconds)
timeout_sec: 600
```

## 3. GitHub Webhook Configuration

In your GitHub repository:
1. Navigate to **Settings > Webhooks > Add webhook**.
2. **Payload URL**: `http://your-server-ip:9850/webhook` (or `https://previewforge.yourdomain.com/webhook`).
3. **Content type**: `application/json`.
4. **Secret**: Enter your chosen `WEBHOOK_SECRET`.
5. **Which events would you like to trigger this webhook?**: Select **Let me select individual events**, and check **Pull requests**.
6. Click **Add webhook**.
