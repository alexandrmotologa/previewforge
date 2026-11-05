package model

import "time"

// DeploymentStatus represents the current lifecycle state of a preview stack.
type DeploymentStatus string

const (
	StatusPending   DeploymentStatus = "pending"
	StatusBuilding  DeploymentStatus = "building"
	StatusRunning   DeploymentStatus = "running"
	StatusFailed    DeploymentStatus = "failed"
	StatusStopped   DeploymentStatus = "stopped"
	StatusDestroyed DeploymentStatus = "destroyed"
)

// Deployment represents an ephemeral pull request preview environment.
type Deployment struct {
	ID               string           `json:"id"`
	PRNumber         int              `json:"pr_number"`
	PRTitle          string           `json:"pr_title"`
	RepoOwner        string           `json:"repo_owner"`
	RepoName         string           `json:"repo_name"`
	Branch           string           `json:"branch"`
	CommitSHA        string           `json:"commit_sha"`
	Author           string           `json:"author"`
	Status           DeploymentStatus `json:"status"`
	Port             int              `json:"port"`
	Subdomain        string           `json:"subdomain"`
	PreviewURL       string           `json:"preview_url"`
	ServiceName      string           `json:"service_name"`
	BuildDurationSec int64            `json:"build_duration_sec"`
	HealthStatus     string           `json:"health_status"`
	ErrorMessage     string           `json:"error_message,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// PortAllocation tracks port reservations in the host port pool.
type PortAllocation struct {
	Port         int       `json:"port"`
	DeploymentID string    `json:"deployment_id"`
	AllocatedAt  time.Time `json:"allocated_at"`
	InUse        bool      `json:"in_use"`
}

// SystemStats contains real-time host and daemon operational metrics.
type SystemStats struct {
	DaemonUptimeSec   int64   `json:"daemon_uptime_sec"`
	ActiveDeployments int     `json:"active_deployments"`
	TotalDeployments  int     `json:"total_deployments"`
	PortPoolUsed      int     `json:"port_pool_used"`
	PortPoolTotal     int     `json:"port_pool_total"`
	HostRAMTotalMB    float64 `json:"host_ram_total_mb"`
	HostRAMUsedMB     float64 `json:"host_ram_used_mb"`
	HostRAMPercent    float64 `json:"host_ram_percent"`
	HostCPUPercent    float64 `json:"host_cpu_percent"`
	HostDiskPercent   float64 `json:"host_disk_percent"`
}

// SystemConfig stores persistent application runtime settings.
type SystemConfig struct {
	BaseDomain          string `json:"base_domain"`
	PublicPort          int    `json:"public_port"`
	ProxyMode           string `json:"proxy_mode"` // "standalone" or "caddy"
	CaddyAdminURL       string `json:"caddy_admin_url"`
	PortRangeStart      int    `json:"port_range_start"`
	PortRangeEnd        int    `json:"port_range_end"`
	GitHubToken         string `json:"github_token"`
	WebhookSecret       string `json:"webhook_secret"`
	WorkspacesDir       string `json:"workspaces_dir"`
	IdleTimeoutMinutes  int    `json:"idle_timeout_minutes"`
	RequireLabel        string `json:"require_label"`
	AllowUntrustedForks bool   `json:"allow_untrusted_forks"`
}

// RepoConfig represents an optional in-repo .previewforge.yml file.
type RepoConfig struct {
	ComposeFile string            `yaml:"compose_file" json:"compose_file"`
	Service     string            `yaml:"service" json:"service"`
	Port        int               `yaml:"port" json:"port"`
	HealthPath  string            `yaml:"health_path" json:"health_path"`
	Env         map[string]string `yaml:"env" json:"env"`
	TimeoutSec  int               `yaml:"timeout_sec" json:"timeout_sec"`
}
