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
	"github.com/alexandrmotologa/previewforge/internal/notification"
	"github.com/alexandrmotologa/previewforge/internal/proxy"
	"github.com/alexandrmotologa/previewforge/internal/storage"
	"github.com/alexandrmotologa/previewforge/internal/webhook"
)

// Orchestrator coordinates the complete preview deployment workflow.
type Orchestrator struct {
	store          *storage.Store
	config         model.SystemConfig
	allocator      *PortAllocator
	gitCloner      GitFetcher
	composeBuilder *ComposeBuilder
	dockerRunner   DockerRunner
	healthChecker  *HealthChecker
	proxyRouter    proxy.ProxyRouter
	commenter      *github.Commenter
	notifier       *notification.Notifier
	broadcaster    *LogBroadcaster
	deployLocks    map[string]*sync.Mutex
	locksMu        sync.Mutex
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
	notifier *notification.Notifier,
	broadcaster *LogBroadcaster,
) *Orchestrator {
	orch := &Orchestrator{
		store:          store,
		config:         cfg,
		allocator:      allocator,
		gitCloner:      gitCloner,
		composeBuilder: composeBuilder,
		dockerRunner:   dockerRunner,
		healthChecker:  healthChecker,
		proxyRouter:    proxyRouter,
		commenter:      commenter,
		notifier:       notifier,
		broadcaster:    broadcaster,
		deployLocks:    make(map[string]*sync.Mutex),
	}

	// Connect dynamic reverse proxy wake-on-request and activity tracking hooks
	if proxyRouter != nil {
		proxyRouter.SetWakeHandler(func(deploymentID string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			return orch.Wake(ctx, deploymentID)
		})
		proxyRouter.SetActivityHandler(func(deploymentID string) {
			_ = store.TouchActivity(deploymentID)
		})
	}

	return orch
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
	autoSleep := o.config.AutoSleepMinutes
	if autoSleep <= 0 {
		autoSleep = 60
	}

	dep := &model.Deployment{
		ID:               deploymentID,
		PRNumber:         req.PRNumber,
		PRTitle:          req.PRTitle,
		RepoOwner:        req.RepoOwner,
		RepoName:         req.RepoName,
		Branch:           req.Branch,
		CommitSHA:        req.CommitSHA,
		Author:           req.Author,
		Status:           model.StatusBuilding,
		Port:             port,
		Subdomain:        subdomain,
		PreviewURL:       previewURL,
		ServiceName:      "web",
		HealthStatus:     "unknown",
		LastActivityAt:   time.Now().UTC(),
		AutoSleepMinutes: autoSleep,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := o.store.SaveDeployment(dep); err != nil {
		log.Printf("[orchestrator] Error saving deployment: %v", err)
	}

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
	projectName = strings.ReplaceAll(projectName, "-", "_")

	if err := o.dockerRunner.Up(ctx, projectName, workspaceDir, inspection.BaseComposeFile, inspection.OverrideFile, logLine); err != nil {
		return o.failDeployment(dep, "Docker Compose build/up failed: "+err.Error(), logLine)
	}

	// 8. Run optional post-deploy hook (e.g. database migrations)
	if inspection.PostDeploy != nil && inspection.PostDeploy.Command != "" {
		targetService := inspection.PostDeploy.Service
		if targetService == "" {
			targetService = inspection.ServiceName
		}
		logLine(fmt.Sprintf("Executing post-deploy hook on %s: %s", targetService, inspection.PostDeploy.Command))

		hookCtx := ctx
		if inspection.PostDeploy.TimeoutSec > 0 {
			var hookCancel context.CancelFunc
			hookCtx, hookCancel = context.WithTimeout(ctx, time.Duration(inspection.PostDeploy.TimeoutSec)*time.Second)
			defer hookCancel()
		}

		cmdParts := strings.Fields(inspection.PostDeploy.Command)
		hookOut, hookErr := o.dockerRunner.Exec(hookCtx, projectName, targetService, cmdParts)
		if hookErr != nil {
			logLine(fmt.Sprintf("Post-deploy hook warning: %v", hookErr))
		} else {
			logLine(fmt.Sprintf("Post-deploy hook output: %s", strings.TrimSpace(hookOut)))
		}
	}

	// 9. Health check verification
	logLine("Verifying container readiness...")
	if err := o.healthChecker.WaitForReady(ctx, port, inspection.HealthPath, 60*time.Second, logLine); err != nil {
		return o.failDeployment(dep, "Health check failed: "+err.Error(), logLine)
	}

	// 10. Register reverse proxy route with Basic Auth if configured
	authUser := o.config.BasicAuthUser
	authPass := o.config.BasicAuthPassword
	if inspection.AuthUser != "" {
		authUser = inspection.AuthUser
		authPass = inspection.AuthPassword
	}

	routeCfg := proxy.RouteConfig{
		TargetPort:   port,
		DeploymentID: deploymentID,
		AuthUser:     authUser,
		AuthPassword: authPass,
		IsSleeping:   false,
	}

	if err := o.proxyRouter.RegisterRouteWithConfig(subdomain, routeCfg); err != nil {
		return o.failDeployment(dep, "Proxy route registration failed: "+err.Error(), logLine)
	}

	// 11. Mark Running and finalize
	duration := int64(time.Since(startTime).Seconds())
	dep.Status = model.StatusRunning
	dep.HealthStatus = "healthy"
	dep.BuildDurationSec = duration
	dep.ErrorMessage = ""
	dep.LastActivityAt = time.Now().UTC()

	// Capture initial stats
	if stats, statErr := o.dockerRunner.Stats(ctx, projectName); statErr == nil && stats != nil {
		dep.CPUPercent = stats.CPUPercent
		dep.MemoryMB = stats.MemoryMB
	}

	if err := o.store.SaveDeployment(dep); err != nil {
		log.Printf("[orchestrator] Error updating deployment status: %v", err)
	}

	logLine(fmt.Sprintf("Preview environment live at %s (built in %ds).", previewURL, duration))

	if o.commenter != nil {
		o.commenter.SetCommitStatus(req.RepoOwner, req.RepoName, req.CommitSHA, "success", previewURL, "Preview environment ready")
		o.commenter.PostReady(dep)
	}

	if o.notifier != nil {
		o.notifier.NotifyReady(dep)
	}

	return dep, nil
}

// Sleep hibernates a running preview container stack to free host memory.
func (o *Orchestrator) Sleep(ctx context.Context, deploymentID string) error {
	lock := o.getLock(deploymentID)
	lock.Lock()
	defer lock.Unlock()

	dep, err := o.store.GetDeployment(deploymentID)
	if err != nil || dep == nil {
		return fmt.Errorf("deployment %s not found", deploymentID)
	}

	if dep.Status != model.StatusRunning {
		return nil
	}

	projectName := fmt.Sprintf("pf_pr%d_%s", dep.PRNumber, strings.ToLower(dep.RepoName))
	projectName = strings.ReplaceAll(projectName, "-", "_")
	workspaceDir := filepath.Join(o.config.WorkspacesDir, deploymentID)

	logLine := func(msg string) {
		formatted := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
		_ = o.store.AppendLog(deploymentID, formatted)
		o.broadcaster.Broadcast(deploymentID, formatted)
	}

	logLine("Hibernating preview containers to conserve host RAM...")

	baseFile := "docker-compose.yml"
	overrideFile := "docker-compose.previewforge.override.yml"
	if err := o.dockerRunner.Stop(ctx, projectName, workspaceDir, baseFile, overrideFile, logLine); err != nil {
		logLine(fmt.Sprintf("Warning: stop failed: %v", err))
	}

	dep.Status = model.StatusSleeping
	dep.HealthStatus = "sleeping"
	dep.CPUPercent = 0.0
	dep.MemoryMB = 0.0
	_ = o.store.SaveDeployment(dep)

	// Update proxy route to sleeping
	o.proxyRouter.SetRouteSleeping(dep.Subdomain, true)

	logLine("Preview environment is now hibernated. It will wake up on next HTTP request.")
	return nil
}

// Wake resumes a hibernated preview container stack on request.
func (o *Orchestrator) Wake(ctx context.Context, deploymentID string) error {
	lock := o.getLock(deploymentID)
	lock.Lock()
	defer lock.Unlock()

	dep, err := o.store.GetDeployment(deploymentID)
	if err != nil || dep == nil {
		return fmt.Errorf("deployment %s not found", deploymentID)
	}

	if dep.Status != model.StatusSleeping {
		return nil
	}

	projectName := fmt.Sprintf("pf_pr%d_%s", dep.PRNumber, strings.ToLower(dep.RepoName))
	projectName = strings.ReplaceAll(projectName, "-", "_")
	workspaceDir := filepath.Join(o.config.WorkspacesDir, deploymentID)

	logLine := func(msg string) {
		formatted := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
		_ = o.store.AppendLog(deploymentID, formatted)
		o.broadcaster.Broadcast(deploymentID, formatted)
	}

	logLine("Waking up hibernated preview stack...")

	baseFile := "docker-compose.yml"
	overrideFile := "docker-compose.previewforge.override.yml"
	if err := o.dockerRunner.Start(ctx, projectName, workspaceDir, baseFile, overrideFile, logLine); err != nil {
		return fmt.Errorf("failed to wake stack: %w", err)
	}

	// Verify health
	if err := o.healthChecker.WaitForReady(ctx, dep.Port, "/", 30*time.Second, logLine); err != nil {
		return fmt.Errorf("wake readiness check failed: %w", err)
	}

	dep.Status = model.StatusRunning
	dep.HealthStatus = "healthy"
	dep.LastActivityAt = time.Now().UTC()
	_ = o.store.SaveDeployment(dep)

	o.proxyRouter.SetRouteSleeping(dep.Subdomain, false)

	logLine("Preview environment resumed successfully.")
	return nil
}

// Exec executes a command inside a specific preview container service (for migrations or interactive web studio console).
func (o *Orchestrator) Exec(ctx context.Context, deploymentID, service string, command []string) (string, error) {
	dep, err := o.store.GetDeployment(deploymentID)
	if err != nil || dep == nil {
		return "", fmt.Errorf("deployment %s not found", deploymentID)
	}

	if dep.Status != model.StatusRunning {
		return "", fmt.Errorf("cannot exec into deployment with status %s", dep.Status)
	}

	if service == "" {
		service = dep.ServiceName
	}
	if service == "" {
		service = "web"
	}

	projectName := fmt.Sprintf("pf_pr%d_%s", dep.PRNumber, strings.ToLower(dep.RepoName))
	projectName = strings.ReplaceAll(projectName, "-", "_")

	return o.dockerRunner.Exec(ctx, projectName, service, command)
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

	if o.notifier != nil {
		o.notifier.NotifyDestroyed(repoOwner, repoName, prNumber)
	}

	return nil
}

func (o *Orchestrator) failDeployment(dep *model.Deployment, errMsg string, logLine func(string)) (*model.Deployment, error) {
	logLine(fmt.Sprintf("Deployment failed: %s", errMsg))

	dep.Status = model.StatusFailed
	dep.HealthStatus = "unhealthy"
	dep.ErrorMessage = errMsg
	_ = o.store.SaveDeployment(dep)

	logs, _ := o.store.GetLogs(dep.ID, 20)
	snippet := strings.Join(logs, "\n")

	if o.commenter != nil {
		o.commenter.SetCommitStatus(dep.RepoOwner, dep.RepoName, dep.CommitSHA, "failure", dep.PreviewURL, errMsg)
		o.commenter.PostFailed(dep, snippet)
	}

	if o.notifier != nil {
		o.notifier.NotifyFailed(dep)
	}

	return dep, fmt.Errorf("%s", errMsg)
}
