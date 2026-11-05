package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

// DefaultConfig returns default configuration values suitable for single-node deployments.
func DefaultConfig() model.SystemConfig {
	defaultDataDir := os.Getenv("PREVIEWFORGE_DATA_DIR")
	if defaultDataDir == "" {
		defaultDataDir = "./data"
	}

	workspacesDir := os.Getenv("PREVIEWFORGE_WORKSPACES_DIR")
	if workspacesDir == "" {
		workspacesDir = filepath.Join(defaultDataDir, "workspaces")
	}

	port := 9850
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			port = p
		}
	}
	if envPort := os.Getenv("PREVIEWFORGE_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			port = p
		}
	}

	baseDomain := os.Getenv("PREVIEWFORGE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "localhost"
	}

	proxyMode := os.Getenv("PREVIEWFORGE_PROXY_MODE")
	if proxyMode == "" {
		proxyMode = "standalone"
	}

	caddyURL := os.Getenv("PREVIEWFORGE_CADDY_URL")
	if caddyURL == "" {
		caddyURL = "http://127.0.0.1:2019"
	}

	portStart := 31000
	if val := os.Getenv("PREVIEWFORGE_PORT_START"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			portStart = p
		}
	}

	portEnd := 35000
	if val := os.Getenv("PREVIEWFORGE_PORT_END"); val != "" {
		if p, err := strconv.Atoi(val); err == nil && p > 0 {
			portEnd = p
		}
	}

	idleTimeout := 1440
	if val := os.Getenv("PREVIEWFORGE_IDLE_TIMEOUT"); val != "" {
		if t, err := strconv.Atoi(val); err == nil && t > 0 {
			idleTimeout = t
		}
	}

	return model.SystemConfig{
		BaseDomain:          baseDomain,
		PublicPort:          port,
		ProxyMode:           proxyMode,
		CaddyAdminURL:       caddyURL,
		PortRangeStart:      portStart,
		PortRangeEnd:        portEnd,
		GitHubToken:         os.Getenv("GITHUB_TOKEN"),
		WebhookSecret:       os.Getenv("WEBHOOK_SECRET"),
		WorkspacesDir:       workspacesDir,
		IdleTimeoutMinutes:  idleTimeout,
		RequireLabel:        os.Getenv("PREVIEWFORGE_REQUIRE_LABEL"),
		AllowUntrustedForks: os.Getenv("PREVIEWFORGE_ALLOW_FORKS") == "true",
	}
}
