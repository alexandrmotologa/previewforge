<p align="center">
  <img src="docs/images/logo.png?raw=true" alt="PreviewForge Mascot Logo" width="130" style="border-radius: 24px;" />
</p>

<h1 align="center">PreviewForge</h1>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.23+-00ADD8?style=flat-square&logo=go" alt="Go 1.23+" />
  <img src="https://img.shields.io/badge/react-19-61DAFB?style=flat-square&logo=react" alt="React 19" />
  <img src="https://img.shields.io/badge/docker-compose-2496ED?style=flat-square&logo=docker" alt="Docker Compose" />
  <img src="https://img.shields.io/badge/caddy-v2-1F88C0?style=flat-square&logo=caddy" alt="Caddy" />
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License MIT" />
</p>

<p align="center">
  Self-hosted ephemeral pull request preview supervisor and reverse proxy daemon. Runs on a single Linux or VPS server ($5 to $10/month), receives GitHub webhooks, builds Docker Compose stacks in isolated networks with dynamic port multiplexing, configures HTTPS subdomains via Caddy or the built-in proxy, and publishes sticky status comments on pull requests. Features auto-sleep container hibernation, wake-on-request proxying, post-deploy migration hooks, disk space guard, and an interactive web studio. On merge or close, it halts containers and reclaims disk space.
</p>

<p align="center">
  <img src="docs/images/previewforge_demo.gif?raw=true" alt="PreviewForge Live Demonstration" width="100%" style="border-radius: 12px; border: 1px solid #1e293b;" />
</p>

---

## Architecture and Data Flow

```
+-----------------------------------------------------------------------------------+
|                                   PREVIEWFORGE                                    |
+-----------------------------------------+-----------------------------------------+
                                          |
        +---------------------------------+---------------------------------+
        |                                                                   |
+-------v-------------------------+                 +-----------------------v-------+
|    GitHub Webhook Ingestion     |                 |   Docker Compose Orchestrator |
|  * pull_request.opened / sync   |                 |  * Isolated Docker Networks   |
|  * pull_request.closed (clean)  |                 |  * Dynamic Port Multiplexing  |
|  * Webhook Secret Verification  |                 |  * Post-Deploy Migration Hook |
|  * GitHub Deployments API       |                 |  * Disk Guard Auto-Pruner     |
+-------+-------------------------+                 +-----------------------+-------+
        |                                                                   |
        +---------------------------------+---------------------------------+
                                          |
                        +-----------------v-----------------+
                        |   Dynamic Reverse Proxy & SSL     |
                        |  * Embedded Caddy / ACME Router   |
                        |  * Auto-Sleep & Wake-on-Request   |
                        |  * Basic Auth & Anti-Index Robots |
                        |  * pr-{id}.preview.domain.com     |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |    Status & Team Notifications    |
                        |  * Sticky GitHub PR Comments      |
                        |  * Native GitHub Deployments API  |
                        |  * Discord & Slack Webhooks       |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |   Interactive Web Studio (:9850)  |
                        |  * Active & Hibernated Matrix     |
                        |  * Per-Container RAM / CPU Gauges |
                        |  * Live SSE Streaming Logs        |
                        |  * Browser Container Exec Console |
                        |  * 1-Click Manual Teardown & Wake |
                        +-----------------------------------+
```

---

## Key Capabilities

- **Single-Binary Deployment**: Shipped as a self-contained Go binary with an embedded React 19 web dashboard and pure-Go SQLite storage engine. No external database or runtime dependencies required.
- **Auto-Sleep & Wake-on-Request (Scale-to-Zero)**:
  - Automatically hibernates preview stacks that receive no HTTP traffic for a configurable duration (default 60 minutes), freeing VPS memory.
  - When traffic hits a hibernated preview (`pr-42.preview.domain.com`), the reverse proxy serves an auto-reloading wake splash page, resumes containers in 1 to 2 seconds, and connects the user.
- **Dynamic Port Multiplexing**: Allocates host ports across a designated range (default `31000` to `35000`) with persistent tracking in SQLite to prevent port collisions across daemon restarts.
- **Dual Reverse Proxy Support**:
  - Standalone mode: Built-in Go HTTP reverse proxy with host-based dynamic routing, WebSocket upgrade support, and custom error handling.
  - Caddy mode: Directly manages Caddy reverse proxy routes through the Caddy Admin JSON API (`http://127.0.0.1:2019`).
- **Security & Privacy Controls**:
  - Injects `X-Robots-Tag: noindex, nofollow, noarchive` headers on all preview responses to prevent search engine indexing.
  - Supports Basic Auth password protection globally or per-repository via `.previewforge.yml` or `?preview_key=...` bypass.
- **Post-Deploy Migration Hooks**: Executes database migrations (e.g. `npm run db:migrate` or `prisma migrate deploy`) inside the target container service before marking the preview healthy.
- **Native GitHub Deployments API**: Registers official GitHub Deployments with active environment links and updates sticky markdown PR comments with commit SHA, duration, and preview URL.
- **Team Webhook Notifications**: Sends real-time build and teardown cards to Discord or Slack channels.
- **Disk Space Guard**: Monitors host storage in real-time and proactively triggers Docker build cache and image prunes when free space falls below safety thresholds.
- **Interactive Web Studio**:
  - Real-time browser dashboard displaying active and hibernated preview environments.
  - CPU and memory consumption per container stack.
  - Live Server-Sent Events (SSE) build logs.
  - In-browser Container Exec console for running diagnostic commands (`env`, `ls`, `npm list`, `df -h`) without SSH access.
  - Manual sleep, wake, rebuild, and teardown buttons.

---

## Interactive Web Studio & Mobile Experience

### Desktop Matrix Overview
Full visibility into active preview containers, dynamic port allocations, host RAM consumption, and VPS storage guards.

<p align="center">
  <img src="docs/images/previewforge_dashboard.png?raw=true" alt="PreviewForge Desktop Dashboard" width="100%" style="border-radius: 8px; border: 1px solid #1e293b;" />
</p>

### Real-Time Build Logs & Diagnostic Container Console
Inspect shallow git clone progress, docker compose build output, and run interactive shell commands inside live containers directly from the web studio.

<p align="center">
  <img src="docs/images/previewforge_logs_exec.png?raw=true" alt="PreviewForge Build Logs" width="49%" style="border-radius: 8px; border: 1px solid #1e293b;" />
  <img src="docs/images/previewforge_exec_console.png?raw=true" alt="PreviewForge Container Exec Console" width="49%" style="border-radius: 8px; border: 1px solid #1e293b;" />
</p>

### Responsive Mobile Interface
Optimized for smartphones and tablets. Review preview environments, monitor build statuses, and trigger manual wake or teardown actions from mobile devices.

<p align="center">
  <img src="docs/images/previewforge_mobile.png?raw=true" alt="PreviewForge Mobile Responsive View" width="340" style="border-radius: 16px; border: 1px solid #1e293b;" />
</p>

---

## Quick Start

### 1. Run with Docker Compose

Create a `docker-compose.yml` on your server:

```yaml
services:
  previewforge:
    image: ghcr.io/alexandrmotologa/previewforge:latest
    container_name: previewforge
    restart: unless-stopped
    ports:
      - "9850:9850"
      - "31000-32000:31000-32000"
    environment:
      - PREVIEWFORGE_DOMAIN=preview.yourdomain.com
      - GITHUB_TOKEN=ghp_yourpersonalaccesstoken
      - WEBHOOK_SECRET=your_webhook_hmac_secret
      - PREVIEWFORGE_AUTO_SLEEP=60
      - DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/data
```

Start the daemon:

```bash
docker compose up -d
```

### 2. Run Directly from Binary

Download the compiled binary and start the supervisor:

```bash
./previewforge serve \
  --domain preview.yourdomain.com \
  --port 9850 \
  --github-token ghp_yourpersonalaccesstoken \
  --webhook-secret your_webhook_hmac_secret \
  --auto-sleep 60
```

Access the Web Studio dashboard at `http://localhost:9850`.

---

## CLI Reference

PreviewForge provides management subcommands:

```bash
# Start the supervisor daemon, web studio, and proxy
previewforge serve --domain preview.yourdomain.com --port 9850

# List active preview environments
previewforge list

# Destroy a specific preview environment and clean up containers
previewforge destroy pr-42-storefront

# Prune orphaned workspaces on disk
previewforge prune

# Print application version
previewforge version
```

---

## Pull Request Lifecycle

```
Developer pushes to PR branch
  │
  ▼
GitHub sends webhook to /webhook
  │
  ▼
PreviewForge verifies HMAC signature
  │
  ▼
Allocates free host port (e.g. 31042)
  │
  ▼
Shallow clones PR commit into ephemeral workspace
  │
  ▼
Synthesizes docker-compose.previewforge.override.yml
  │
  ▼
Executes docker compose up -d --build
  │
  ▼
Executes post-deploy migration hook (e.g. npm run db:migrate)
  │
  ▼
Polls http://127.0.0.1:31042/ for readiness
  │
  ▼
Registers route pr-42.preview.yourdomain.com -> 127.0.0.1:31042
  │
  ▼
Posts sticky comment, native GitHub deployment, and team webhook
  │
  ▼
Inactivity timer triggers Auto-Sleep (docker compose stop)
  │
  ▼
Next HTTP request wakes stack in 1-2 seconds (docker compose start)
  │
  ▼
PR merge/close triggers docker compose down -v and frees port
```

---

## In-Repository Configuration (`.previewforge.yml`)

Add an optional `.previewforge.yml` file to the root of your repository to override defaults:

```yaml
# Compose file to execute (default: docker-compose.yml)
compose_file: "docker-compose.yml"

# Primary service to expose via the preview subdomain
service: "web"

# Container internal port (default: 3000)
port: 3000

# Path polled during healthcheck verification (default: /)
health_path: "/api/health"

# Post-deploy command executed inside the container before marking ready
post_deploy:
  command: "npx prisma migrate deploy"
  service: "web"
  timeout_sec: 120

# Optional password protection for preview subdomains
basic_auth:
  user: "tester"
  password: "stagingpassword123"

# Custom environment variables injected into preview containers
env:
  NODE_ENV: "production"
  DATABASE_URL: "postgres://postgres:secret@db:5432/app"
```

---

## Architecture and Documentation

- [Architecture Deep Dive](docs/ARCHITECTURE.md): Container isolation, hibernation state machine, port ledger, and proxy design.
- [Configuration Guide](docs/CONFIGURATION.md): Complete list of environment variables, CLI flags, notifications, and webhook setup.
- [Caddy and Wildcard TLS](docs/CADDY_INTEGRATION.md): Wildcard DNS, Cloudflare DNS challenge, and Caddy Admin API.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
