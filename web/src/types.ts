export type DeploymentStatus = 'pending' | 'building' | 'running' | 'sleeping' | 'failed' | 'stopped' | 'destroyed';

export interface Deployment {
  id: string;
  pr_number: number;
  pr_title: string;
  repo_owner: string;
  repo_name: string;
  branch: string;
  commit_sha: string;
  author: string;
  status: DeploymentStatus;
  port: number;
  subdomain: string;
  preview_url: string;
  service_name: string;
  build_duration_sec: number;
  health_status: string;
  error_message?: string;
  last_activity_at: string;
  auto_sleep_minutes: number;
  cpu_percent: number;
  memory_mb: number;
  created_at: string;
  updated_at: string;
}

export interface SystemStats {
  daemon_uptime_sec: number;
  active_deployments: number;
  sleeping_deployments: number;
  total_deployments: number;
  port_pool_used: number;
  port_pool_total: number;
  host_ram_total_mb: number;
  host_ram_used_mb: number;
  host_ram_percent: number;
  host_cpu_percent: number;
  host_disk_percent: number;
  host_disk_free_gb: number;
  host_disk_total_gb: number;
  disk_low_alert: boolean;
}

export interface SystemConfig {
  base_domain: string;
  public_port: number;
  proxy_mode: string;
  port_range_start: number;
  port_range_end: number;
  workspaces_dir: string;
  idle_timeout_minutes: number;
  auto_sleep_minutes: number;
  require_label: string;
  has_github_token: boolean;
  has_webhook_secret: boolean;
  allow_untrusted_forks: boolean;
  has_basic_auth: boolean;
  has_discord_webhook: boolean;
  has_slack_webhook: boolean;
}
