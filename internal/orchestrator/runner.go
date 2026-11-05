package orchestrator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/github"
	"github.com/alexandrmotologa/previewforge/internal/model"
	"github.com/alexandrmotologa/previewforge/internal/proxy"
	"github.com/alexandrmotologa/previewforge/internal/storage"
	"github.com/alexandrmotologa/previewforge/internal/webhook"
)

// Orchestrator coordinates the complete preview deployment workflow.
type Orchestrator struct {
	store           *storage.Store
	config          model.SystemConfig
	allocator       *PortAllocator
	gitCloner       GitFetcher
	composeBuilder  *ComposeBuilder
	dockerRunner    DockerRunner
	healthChecker   *HealthChecker
	proxyRouter     proxy.ProxyRouter
	commenter       *github.Commenter
	broadcaster     *LogBroadcaster
	deployLocks     map[string]*sync.Mutex
	locksMu         sync.Mutex
}

// NewOrchestrator constructs an Orchestrator.
func NewOrchestrator(
	store *storage.Store,
	cfg model.SystemConfig,
	allocator *PortAllocator,
	gitCloner GitFetcher,
	composeBuilder *ComposeBuilder,
	dockerRunner DockerRunner,
	healthChecker *HealthChecker,
	proxyRouter proxy.ProxyRouter,
	commenter *github.Commenter,
	broadcaster *LogBroadcaster,
) *Orchestrator {
	return &Orchestrator{
		store:          store,
		config:         cfg,
		allocator:      allocator,
		gitCloner:      gitCloner,
		composeBuilder: composeBuilder,
		dockerRunner:   dockerRunner,
		healthChecker:  healthChecker,
		proxyRouter:    proxyRouter,
		commenter:      commenter,
		broadcaster:    broadcaster,
		deployLocks:    make(map[string]*sync.Mutex),
	}
}

func (o *Orchestrator) getLock(deploymentID string) *sync.Mutex {
	o.locksMu.Lock()
	defer o.locksMu.Unlock()
	if l, ok := o.deployLocks[deploymentID]; ok {
		return l
	}
	l := &sync.Mutex{}
	o.deployLocks[deploymentID] = l
	return l
}

// Broadcaster returns the real-time log event broker.
func (o *Orchestrator) Broadcaster() *LogBroadcaster {
	return o.broadcaster
}

// Deploy creates or updates an isolated Docker Compose preview stack for a pull request.
func (o *Orchestrator) Deploy(ctx context.Context, req webhook.DeployRequest) (*model.Deployment, error) {
	deploymentID := fmt.Sprintf("pr-%d-%s", req.PRNumber, req.RepoName)
	lock := o.getLock(deploymentID)
	lock.Lock()
	defer lock.Unlock()

	startTime := time.Now()

	// 1. Determine Subdomain & Preview URL
	subdomain := fmt.Sprintf("pr-%d.%s", req.PRNumber, o.config.BaseDomain)
	var previewURL string
	if o.config.BaseDomain == "localhost" || strings.HasPrefix(o.config.BaseDomain, "127.0.0.1") {
		previewURL = fmt.Sprintf("http://%s:%d", subdomain, o.config.PublicPort)
	} else {
		previewURL = fmt.Sprintf("https://%s", subdomain)
	}

	// 2. Allocate unique host port
	port, err := o.allocator.Allocate(deploymentID)
	if err != nil {
		return nil, fmt.Errorf("port allocation failed: %w", err)
	}

	// 3. Persist initial deployment state
	dep := &model.Deployment{
		ID:           deploymentID,
		PRNumber:     req.PRNumber,
		PRTitle:      req.PRTitle,
		RepoOwner:    req.RepoOwner,
		RepoName:     req.RepoName,
		Branch:       req.Branch,
		CommitSHA:    req.CommitSHA,
		Author:       req.Author,
		Status:       model.StatusBuilding,
		Port:         port,
		Subdomain:    subdomain,
		PreviewURL:   previewURL,
		ServiceName:  "web",
		HealthStatus: "unknown",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := o.store.SaveDeployment(dep); err != nil {
		log.Printf("[orchestrator] Error saving deployment: %v", err)
	}

	// Helper for log recording
	logLine := func(msg string) {
		formatted := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
		_ = o.store.AppendLog(deploymentID, formatted)
		o.broadcaster.Broadcast(deploymentID, formatted)
	}

	logLine(fmt.Sprintf("PreviewForge initiated for %s/%s PR #%d (branch: %s, commit: %s)",
		req.RepoOwner, req.RepoName, req.PRNumber, req.Branch, req.CommitSHA))

	// 4. Update GitHub status and PR comment
	if o.commenter != nil {
		o.commenter.SetCommitStatus(req.RepoOwner, req.RepoName, req.CommitSHA, "pending", previewURL, "Building preview stack...")
		o.commenter.PostBuilding(dep)
	}

	// 5. Clone repository
	workspaceDir := filepath.Join(o.config.WorkspacesDir, deploymentID)
	if err := o.gitCloner.CloneOrUpdate(ctx, req.CloneURL, req.Branch, req.CommitSHA, workspaceDir, logLine); err != nil {
		return o.failDeployment(dep, "Git checkout failed: "+err.Error(), logLine)
	}

	// 6. Inspect compose definitions and synthesize override
	inspection, err := o.composeBuilder.InspectAndPrepare(workspaceDir, deploymentID, req.PRNumber, port, previewURL)
	if err != nil {
		return o.failDeployment(dep, "Compose preparation failed: "+err.Error(), logLine)
	}

	dep.ServiceName = inspection.ServiceName

	// 7. Execute docker compose up
	projectName := fmt.Sprintf("pf_pr%d_%s", req.PRNumber, strings.ToLower(req.RepoName))
	// Clean non-alphanumeric chars from project name for Docker compatibility
	projectName = strings.ReplaceAll(projectName, "-", "_")

	if err := o.dockerRunner.Up(ctx, projectName, workspaceDir, inspection.BaseComposeFile, inspection.OverrideFile, logLine); err != nil {
		return o.failDeployment(dep, "Docker Compose build/up failed: "+err.Error(), logLine)
	}

	// 8. Health check verification
	logLine("Verifying container readiness...")
	if err := o.healthChecker.WaitForReady(ctx, port, inspection.HealthPath, 60*time.Second, logLine); err != nil {
		return o.failDeployment(dep, "Health check failed: "+err.Error(), logLine)
	}

	// 9. Register reverse proxy route
	if err := o.proxyRouter.RegisterRoute(subdomain, port); err != nil {
		return o.failDeployment(dep, "Proxy route registration failed: "+err.Error(), logLine)
	}

	// 10. Mark Running and finalize
	duration := int64(time.Since(startTime).Seconds())
	dep.Status = model.StatusRunning
	dep.HealthStatus = "healthy"
	dep.BuildDurationSec = duration
	dep.ErrorMessage = ""

	if err := o.store.SaveDeployment(dep); err != nil {
		log.Printf("[orchestrator] Error updating deployment status: %v", err)
	}

	logLine(fmt.Sprintf("Preview environment live at %s (built in %ds).", previewURL, duration))

	if o.commenter != nil {
		o.commenter.SetCommitStatus(req.RepoOwner, req.RepoName, req.CommitSHA, "success", previewURL, "Preview environment ready")
		o.commenter.PostReady(dep)
	}

	return dep, nil
}

// Teardown stops containers and cleans up all resources associated with a pull request.
func (o *Orchestrator) Teardown(ctx context.Context, repoOwner, repoName string, prNumber int) error {
	deploymentID := fmt.Sprintf("pr-%d-%s", prNumber, repoName)
	lock := o.getLock(deploymentID)
	lock.Lock()
	defer lock.Unlock()

	dep, err := o.store.GetDeployment(deploymentID)
	if err != nil || dep == nil {
		return nil
	}

	logLine := func(msg string) {
		formatted := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
		_ = o.store.AppendLog(deploymentID, formatted)
		o.broadcaster.Broadcast(deploymentID, formatted)
	}

	logLine(fmt.Sprintf("Tearing down preview stack for PR #%d...", prNumber))

	// 1. Remove reverse proxy route
	_ = o.proxyRouter.RemoveRoute(dep.Subdomain)

	// 2. Stop docker containers and remove volumes
	projectName := fmt.Sprintf("pf_pr%d_%s", prNumber, strings.ToLower(repoName))
	projectName = strings.ReplaceAll(projectName, "-", "_")
	workspaceDir := filepath.Join(o.config.WorkspacesDir, deploymentID)

	baseFile := "docker-compose.yml"
	overrideFile := "docker-compose.previewforge.override.yml"
	_ = o.dockerRunner.Down(ctx, projectName, workspaceDir, baseFile, overrideFile, logLine)

	// 3. Remove workspace files from host disk
	_ = os.RemoveAll(workspaceDir)

	// 4. Free allocated port
	_ = o.allocator.Release(dep.Port)

	// 5. Update status
	dep.Status = model.StatusDestroyed
	dep.HealthStatus = "stopped"
	_ = o.store.SaveDeployment(dep)

	logLine("Teardown complete. Workspace and containers cleaned up.")

	if o.commenter != nil {
		o.commenter.PostDestroyed(repoOwner, repoName, prNumber)
	}

	return nil
}

func (o *Orchestrator) failDeployment(dep *model.Deployment, errMsg string, logLine func(string)) (*model.Deployment, error) {
	logLine(fmt.Sprintf("Deployment failed: %s", errMsg))

	dep.Status = model.StatusFailed
	dep.HealthStatus = "unhealthy"
	dep.ErrorMessage = errMsg
	_ = o.store.SaveDeployment(dep)

	// Fetch tail of logs for PR comment
	logs, _ := o.store.GetLogs(dep.ID, 20)
	snippet := strings.Join(logs, "\n")

	if o.commenter != nil {
		o.commenter.SetCommitStatus(dep.RepoOwner, dep.RepoName, dep.CommitSHA, "failure", dep.PreviewURL, errMsg)
		o.commenter.PostFailed(dep, snippet)
	}

	return dep, fmt.Errorf("%s", errMsg)
}
