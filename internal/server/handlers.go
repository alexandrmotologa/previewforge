package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
	"github.com/alexandrmotologa/previewforge/internal/orchestrator"
	"github.com/alexandrmotologa/previewforge/internal/storage"
	"github.com/alexandrmotologa/previewforge/internal/webhook"
)

// APIHandler coordinates REST API operations for the Web Studio frontend.
type APIHandler struct {
	store        *storage.Store
	orchestrator *orchestrator.Orchestrator
	config       model.SystemConfig
	startTime    time.Time
}

// NewAPIHandler constructs an APIHandler.
func NewAPIHandler(store *storage.Store, orch *orchestrator.Orchestrator, cfg model.SystemConfig) *APIHandler {
	return &APIHandler{
		store:        store,
		orchestrator: orch,
		config:       cfg,
		startTime:    time.Now(),
	}
}

func (h *APIHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	deployments, err := h.store.ListDeployments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if deployments == nil {
		deployments = []model.Deployment{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(deployments)
}

func (h *APIHandler) GetDeployment(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/deployments/")
	if id == "" {
		http.Error(w, "Deployment ID required", http.StatusBadRequest)
		return
	}

	dep, err := h.store.GetDeployment(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if dep == nil {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	logs, _ := h.store.GetLogs(id, 200)

	resp := map[string]any{
		"deployment": dep,
		"logs":       logs,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *APIHandler) RebuildDeployment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractID(r.URL.Path, "/api/deployments/")
	id = strings.TrimSuffix(id, "/rebuild")
	if id == "" {
		http.Error(w, "Deployment ID required", http.StatusBadRequest)
		return
	}

	dep, err := h.store.GetDeployment(id)
	if err != nil || dep == nil {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	req := webhook.DeployRequest{
		PRNumber:  dep.PRNumber,
		PRTitle:   dep.PRTitle,
		RepoOwner: dep.RepoOwner,
		RepoName:  dep.RepoName,
		CloneURL:  fmtCloneURL(dep.RepoOwner, dep.RepoName),
		Branch:    dep.Branch,
		CommitSHA: dep.CommitSHA,
		Author:    dep.Author,
	}

	go func() {
		if h.orchestrator == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		_, _ = h.orchestrator.Deploy(ctx, req)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "rebuild_queued", "id": id})
}

func (h *APIHandler) SleepDeployment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractID(r.URL.Path, "/api/deployments/")
	id = strings.TrimSuffix(id, "/sleep")

	if h.orchestrator == nil {
		http.Error(w, "Orchestrator unavailable", http.StatusServiceUnavailable)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = h.orchestrator.Sleep(ctx, id)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "sleep_queued", "id": id})
}

func (h *APIHandler) WakeDeployment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractID(r.URL.Path, "/api/deployments/")
	id = strings.TrimSuffix(id, "/wake")

	if h.orchestrator == nil {
		http.Error(w, "Orchestrator unavailable", http.StatusServiceUnavailable)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = h.orchestrator.Wake(ctx, id)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "wake_queued", "id": id})
}

// ExecRequest payload for container diagnostic command execution.
type ExecRequest struct {
	Command []string `json:"command"`
	Service string   `json:"service"`
}

func (h *APIHandler) ExecContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractID(r.URL.Path, "/api/deployments/")
	id = strings.TrimSuffix(id, "/exec")

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Command) == 0 {
		http.Error(w, "Command cannot be empty", http.StatusBadRequest)
		return
	}

	if h.orchestrator == nil {
		http.Error(w, "Orchestrator unavailable", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	output, err := h.orchestrator.Exec(ctx, id, req.Service, req.Command)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error(), "output": output})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"output": output})
}

func (h *APIHandler) DestroyDeployment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := extractID(r.URL.Path, "/api/deployments/")
	if id == "" {
		http.Error(w, "Deployment ID required", http.StatusBadRequest)
		return
	}

	dep, err := h.store.GetDeployment(id)
	if err != nil || dep == nil {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	go func() {
		if h.orchestrator == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		_ = h.orchestrator.Teardown(ctx, dep.RepoOwner, dep.RepoName, dep.PRNumber)
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "teardown_queued", "id": id})
}

func (h *APIHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/deployments/")
	id = strings.TrimSuffix(id, "/logs")
	if id == "" {
		http.Error(w, "Deployment ID required", http.StatusBadRequest)
		return
	}

	logs, err := h.store.GetLogs(id, 1000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if logs == nil {
		logs = []string{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "logs": logs})
}

func (h *APIHandler) GetSystemStats(w http.ResponseWriter, r *http.Request) {
	active, sleeping, total, _ := h.store.CountDeployments()
	allocs, _ := h.store.ListPortAllocations()

	totalPorts := h.config.PortRangeEnd - h.config.PortRangeStart + 1
	if totalPorts <= 0 {
		totalPorts = 1000
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	allocMB := float64(m.Alloc) / 1024 / 1024

	disk := orchestrator.GetDiskUsage(h.config.WorkspacesDir)

	stats := model.SystemStats{
		DaemonUptimeSec:     int64(time.Since(h.startTime).Seconds()),
		ActiveDeployments:   active,
		SleepingDeployments: sleeping,
		TotalDeployments:    total,
		PortPoolUsed:        len(allocs),
		PortPoolTotal:       totalPorts,
		HostRAMTotalMB:      4096.0,
		HostRAMUsedMB:       allocMB,
		HostRAMPercent:      (allocMB / 4096.0) * 100,
		HostCPUPercent:      0.8,
		HostDiskPercent:     disk.Percent,
		HostDiskFreeGB:      disk.FreeGB,
		HostDiskTotalGB:     disk.TotalGB,
		DiskLowAlert:        disk.Percent > 85.0 || (disk.TotalGB > 0 && disk.FreeGB < 3.0),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (h *APIHandler) GetSystemConfig(w http.ResponseWriter, r *http.Request) {
	safeConfig := map[string]any{
		"base_domain":           h.config.BaseDomain,
		"public_port":           h.config.PublicPort,
		"proxy_mode":            h.config.ProxyMode,
		"port_range_start":      h.config.PortRangeStart,
		"port_range_end":        h.config.PortRangeEnd,
		"workspaces_dir":        h.config.WorkspacesDir,
		"idle_timeout_minutes":  h.config.IdleTimeoutMinutes,
		"auto_sleep_minutes":    h.config.AutoSleepMinutes,
		"require_label":         h.config.RequireLabel,
		"has_github_token":      h.config.GitHubToken != "",
		"has_webhook_secret":    h.config.WebhookSecret != "",
		"allow_untrusted_forks": h.config.AllowUntrustedForks,
		"has_basic_auth":        h.config.BasicAuthUser != "",
		"has_discord_webhook":   h.config.DiscordWebhookURL != "",
		"has_slack_webhook":     h.config.SlackWebhookURL != "",
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(safeConfig)
}

// SimulatorDeployRequest holds parameters for triggering a test preview deployment.
type SimulatorDeployRequest struct {
	PRNumber  int    `json:"pr_number"`
	PRTitle   string `json:"pr_title"`
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"`
	CloneURL  string `json:"clone_url"`
	Branch    string `json:"branch"`
	CommitSHA string `json:"commit_sha"`
	Author    string `json:"author"`
}

func (h *APIHandler) SimulatorDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var simReq SimulatorDeployRequest
	if err := json.NewDecoder(r.Body).Decode(&simReq); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if simReq.PRNumber <= 0 {
		simReq.PRNumber = 1
	}
	if simReq.RepoOwner == "" {
		simReq.RepoOwner = "alexandrmotologa"
	}
	if simReq.RepoName == "" {
		simReq.RepoName = "demo-app"
	}
	if simReq.Branch == "" {
		simReq.Branch = "main"
	}
	if simReq.CommitSHA == "" {
		simReq.CommitSHA = "sim" + time.Now().Format("150405")
	}
	if simReq.Author == "" {
		simReq.Author = "developer"
	}
	if simReq.PRTitle == "" {
		simReq.PRTitle = "Simulator Test Deployment"
	}

	req := webhook.DeployRequest{
		PRNumber:  simReq.PRNumber,
		PRTitle:   simReq.PRTitle,
		RepoOwner: simReq.RepoOwner,
		RepoName:  simReq.RepoName,
		CloneURL:  simReq.CloneURL,
		Branch:    simReq.Branch,
		CommitSHA: simReq.CommitSHA,
		Author:    simReq.Author,
	}

	go func() {
		if h.orchestrator == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		_, _ = h.orchestrator.Deploy(ctx, req)
	}()

	deploymentID := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(simReq.RepoName), " ", "-"))
	expectedSubdomain := strings.ToLower(h.config.BaseDomain)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":    "queued",
		"pr_number": simReq.PRNumber,
		"repo":      simReq.RepoOwner + "/" + simReq.RepoName,
		"subdomain": fmt.Sprintf("pr-%d.%s", simReq.PRNumber, expectedSubdomain),
		"id":        fmt.Sprintf("pr-%d-%s", simReq.PRNumber, deploymentID),
	})
}

func extractID(path, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func fmtCloneURL(owner, repo string) string {
	return "https://github.com/" + owner + "/" + repo + ".git"
}
