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
| `--github-token`| `GITHUB_TOKEN` | `""` | GitHub Personal Access Token or App token used for native GitHub Deployments, commit statuses, and sticky comments. |
| `--webhook-secret`| `WEBHOOK_SECRET` | `""` | Secret key used to verify incoming GitHub webhook HMAC-SHA256 signatures (`X-Hub-Signature-256`). |
| `--port-start` | `PREVIEWFORGE_PORT_START` | `31000` | Starting port number for ephemeral container allocations. |
| `--port-end` | `PREVIEWFORGE_PORT_END` | `35000` | Ending port number for ephemeral container allocations. |
| `--idle-timeout` | `PREVIEWFORGE_IDLE_TIMEOUT` | `1440` | Idle timeout in minutes before an unclosed preview stack is automatically reaped (default 24 hours). |
| `--auto-sleep` | `PREVIEWFORGE_AUTO_SLEEP` | `60` | Inactivity duration in minutes before containers are hibernated (scale-to-zero) to reclaim host RAM. Set to 0 to disable. |
| `--require-label`| `PREVIEWFORGE_REQUIRE_LABEL`| `""` | If set, only PRs containing this label (e.g. `preview`) will be deployed. |
| `--allow-forks` | `PREVIEWFORGE_ALLOW_FORKS` | `false` | When false, PRs from external forks are rejected unless opened by the repository owner. |
| `--auth-user` | `PREVIEWFORGE_AUTH_USER` | `""` | Global Basic Auth username protecting preview subdomains. |
| `--auth-pass` | `PREVIEWFORGE_AUTH_PASS` | `""` | Global Basic Auth password protecting preview subdomains. Also usable as query param `?preview_key=...`. |
| `--discord-webhook` | `DISCORD_WEBHOOK_URL` | `""` | Discord Webhook URL for real-time build and teardown notification cards. |
| `--slack-webhook` | `SLACK_WEBHOOK_URL` | `""` | Slack Webhook URL for build and teardown status messages. |

## 2. In-Repository Configuration (`.previewforge.yml`)

Add an optional `.previewforge.yml` file to the root of your target repository to customize build behavior:

```yaml
# Compose file to execute (default: docker-compose.yml or compose.yaml)
compose_file: "docker-compose.yml"

# Primary service to expose via the preview subdomain (default: auto-detected)
service: "web"

# Container internal port (default: 3000)
port: 3000

# Path polled during readiness verification (default: /)
health_path: "/api/health"

# Post-deploy migration command executed inside the container service before marking ready
post_deploy:
  command: "npx prisma migrate deploy"
  service: "web"
  timeout_sec: 120

# Optional password protection for this specific repository preview
basic_auth:
  user: "tester"
  password: "stagingpassword123"

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
