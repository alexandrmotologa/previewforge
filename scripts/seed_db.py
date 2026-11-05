import sqlite3
import datetime

db_path = 'B:/workgit/previewforge/data/previewforge.db'
conn = sqlite3.connect(db_path)
cur = conn.cursor()

# Run migrations matching Store.migrate()
cur.execute("""
CREATE TABLE IF NOT EXISTS deployments (
    id TEXT PRIMARY KEY,
    pr_number INTEGER NOT NULL,
    pr_title TEXT NOT NULL,
    repo_owner TEXT NOT NULL,
    repo_name TEXT NOT NULL,
    branch TEXT NOT NULL,
    commit_sha TEXT NOT NULL,
    author TEXT NOT NULL,
    status TEXT NOT NULL,
    port INTEGER NOT NULL,
    subdomain TEXT NOT NULL,
    preview_url TEXT NOT NULL,
    service_name TEXT NOT NULL,
    build_duration_sec INTEGER NOT NULL DEFAULT 0,
    health_status TEXT NOT NULL DEFAULT 'unknown',
    error_message TEXT NOT NULL DEFAULT '',
    last_activity_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    auto_sleep_minutes INTEGER NOT NULL DEFAULT 60,
    cpu_percent REAL NOT NULL DEFAULT 0.0,
    memory_mb REAL NOT NULL DEFAULT 0.0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
""")

cur.execute("CREATE INDEX IF NOT EXISTS idx_deployments_repo_pr ON deployments (repo_owner, repo_name, pr_number);")
cur.execute("CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments (status);")

cur.execute("""
CREATE TABLE IF NOT EXISTS port_allocations (
    port INTEGER PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    allocated_at DATETIME NOT NULL,
    in_use INTEGER NOT NULL DEFAULT 1
);
""")

cur.execute("""
CREATE TABLE IF NOT EXISTS deployment_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    line TEXT NOT NULL
);
""")
cur.execute("CREATE INDEX IF NOT EXISTS idx_logs_deployment_id ON deployment_logs (deployment_id);")

cur.execute("""
CREATE TABLE IF NOT EXISTS system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
""")

now = datetime.datetime.now(datetime.timezone.utc).isoformat()
hour_ago = (datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(hours=1)).isoformat()

deployments = [
    (
        "pr-42-storefront", 42, "Add Stripe Checkout & Webhook Handlers", "alexandrmotologa", "storefront",
        "feat/stripe-checkout", "7f8a912b3c4d", "alexandrmotologa", "running", 31042,
        "pr-42.preview.previewforge.dev", "https://pr-42.preview.previewforge.dev", "web", 19, "healthy",
        "", now, 60, 1.8, 142.5, now, now
    ),
    (
        "pr-104-saas-portal", 104, "Migrate to React 19 and Server Actions", "alexandrmotologa", "saas-portal",
        "feat/react-19", "4e2c908f1a3b", "alexandrmotologa", "sleeping", 31043,
        "pr-104.preview.previewforge.dev", "https://pr-104.preview.previewforge.dev", "web", 24, "sleeping",
        "", hour_ago, 60, 0.0, 0.0, hour_ago, hour_ago
    ),
    (
        "pr-58-api-gateway", 58, "Implement Redis Session Cache & Rate Limiter", "alexandrmotologa", "api-gateway",
        "perf/redis-cache", "9a1b8c7d6e5f", "alexandrmotologa", "running", 31044,
        "pr-58.preview.previewforge.dev", "https://pr-58.preview.previewforge.dev", "gateway", 14, "healthy",
        "", now, 60, 0.6, 88.0, now, now
    ),
    (
        "pr-23-backend-core", 23, "Upgrade Postgres connection pooling with PgBouncer", "alexandrmotologa", "backend-core",
        "fix/db-pool", "3d4e5f6a7b8c", "alexandrmotologa", "building", 31045,
        "pr-23.preview.previewforge.dev", "https://pr-23.preview.previewforge.dev", "api", 7, "unknown",
        "", now, 60, 0.0, 0.0, now, now
    ),
]

cur.executemany("""
    INSERT INTO deployments (
        id, pr_number, pr_title, repo_owner, repo_name, branch, commit_sha, author,
        status, port, subdomain, preview_url, service_name, build_duration_sec, health_status,
        error_message, last_activity_at, auto_sleep_minutes, cpu_percent, memory_mb, created_at, updated_at
    ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
""", deployments)

port_allocs = [
    (31042, "pr-42-storefront", now, 1),
    (31043, "pr-104-saas-portal", hour_ago, 1),
    (31044, "pr-58-api-gateway", now, 1),
    (31045, "pr-23-backend-core", now, 1),
]
cur.executemany("INSERT INTO port_allocations (port, deployment_id, allocated_at, in_use) VALUES (?, ?, ?, ?)", port_allocs)

logs = [
    ("pr-42-storefront", now, "[16:20:10] PreviewForge initiated for alexandrmotologa/storefront PR #42 (branch: feat/stripe-checkout, commit: 7f8a912)"),
    ("pr-42-storefront", now, "[16:20:11] Allocated host port 31042 from dynamic pool (31000-35000)"),
    ("pr-42-storefront", now, "[16:20:12] Shallow cloning git ref feat/stripe-checkout into workspaces/pr-42-storefront..."),
    ("pr-42-storefront", now, "[16:20:14] Detected docker-compose.yml and repository configuration .previewforge.yml"),
    ("pr-42-storefront", now, "[16:20:14] Synthesizing docker-compose.previewforge.override.yml with port mapping 31042:3000"),
    ("pr-42-storefront", now, "[16:20:15] Executing: docker compose -p pf_pr42_storefront -f docker-compose.yml -f docker-compose.previewforge.override.yml up -d --build"),
    ("pr-42-storefront", now, "[16:20:21] Service web [build]: Step 1/8 FROM node:20-alpine AS builder"),
    ("pr-42-storefront", now, "[16:20:23] Service web [build]: Step 8/8 COPY --from=builder /app/dist ./dist"),
    ("pr-42-storefront", now, "[16:20:24] Network pf_pr42_storefront_default Created"),
    ("pr-42-storefront", now, "[16:20:24] Container pf_pr42_storefront-db-1 Started"),
    ("pr-42-storefront", now, "[16:20:25] Container pf_pr42_storefront-web-1 Started"),
    ("pr-42-storefront", now, "[16:20:26] Executing post-deploy hook on web: npx prisma migrate deploy"),
    ("pr-42-storefront", now, "[16:20:28] Post-deploy hook output: 2 migrations applied: 20261105_add_stripe_customers, 20261105_webhook_events"),
    ("pr-42-storefront", now, "[16:20:29] Verifying container readiness on http://127.0.0.1:31042/api/health..."),
    ("pr-42-storefront", now, "[16:20:30] Health check passed (HTTP 200 OK)"),
    ("pr-42-storefront", now, "[16:20:30] Registered reverse proxy route pr-42.preview.previewforge.dev -> 127.0.0.1:31042"),
    ("pr-42-storefront", now, "[16:20:31] Injected X-Robots-Tag: noindex, nofollow, noarchive on reverse proxy"),
    ("pr-42-storefront", now, "[16:20:31] Posted sticky PR comment and updated GitHub Deployment to active"),
    ("pr-42-storefront", now, "[16:20:32] Preview environment live at https://pr-42.preview.previewforge.dev (built in 19s)."),
]
cur.executemany("INSERT INTO deployment_logs (deployment_id, timestamp, line) VALUES (?, ?, ?)", logs)

conn.commit()
conn.close()
print("Successfully initialized and seeded previewforge.db!")
