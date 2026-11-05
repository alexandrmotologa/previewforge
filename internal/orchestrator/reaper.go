package orchestrator

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
	"github.com/alexandrmotologa/previewforge/internal/storage"
)

// Reaper runs background sweeps to terminate abandoned stacks, hibernate idle stacks, and prune disk space.
type Reaper struct {
	store         *storage.Store
	orchestrator  *Orchestrator
	dockerRunner  DockerRunner
	workspacesDir string
	idleTimeout   time.Duration
	stopChan      chan struct{}
}

// NewReaper constructs a Reaper instance.
func NewReaper(store *storage.Store, orch *Orchestrator, dockerRunner DockerRunner, workspacesDir string, idleTimeoutMinutes int) *Reaper {
	if idleTimeoutMinutes <= 0 {
		idleTimeoutMinutes = 1440 // 24 hours default
	}
	return &Reaper{
		store:         store,
		orchestrator:  orch,
		dockerRunner:  dockerRunner,
		workspacesDir: workspacesDir,
		idleTimeout:   time.Duration(idleTimeoutMinutes) * time.Minute,
		stopChan:      make(chan struct{}),
	}
}

// Start launches the background reaper ticker.
func (r *Reaper) Start(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-r.stopChan:
				return
			case <-ticker.C:
				r.sweepStaleDeployments()
				r.sweepIdleSleepingDeployments()
				r.sweepDiskSpaceGuard()
				r.sweepOrphanedWorkspaces()
			}
		}
	}()
}

// Stop shuts down the reaper background loop.
func (r *Reaper) Stop() {
	close(r.stopChan)
}

func (r *Reaper) sweepStaleDeployments() {
	deployments, err := r.store.ListDeployments()
	if err != nil {
		log.Printf("[reaper] Failed to list deployments: %v", err)
		return
	}

	cutoff := time.Now().UTC().Add(-r.idleTimeout)

	for _, dep := range deployments {
		if (dep.Status == model.StatusRunning || dep.Status == model.StatusSleeping || dep.Status == model.StatusBuilding) && dep.UpdatedAt.Before(cutoff) {
			log.Printf("[reaper] Stale deployment %s (PR #%d) exceeded idle timeout of %s. Reaping...", dep.ID, dep.PRNumber, r.idleTimeout)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			_ = r.orchestrator.Teardown(ctx, dep.RepoOwner, dep.RepoName, dep.PRNumber)
			cancel()
		}
	}
}

func (r *Reaper) sweepIdleSleepingDeployments() {
	deployments, err := r.store.ListDeployments()
	if err != nil {
		return
	}

	for _, dep := range deployments {
		if dep.Status == model.StatusRunning && dep.AutoSleepMinutes > 0 {
			idleDuration := time.Duration(dep.AutoSleepMinutes) * time.Minute
			if time.Since(dep.LastActivityAt) > idleDuration {
				log.Printf("[reaper] Deployment %s (PR #%d) inactive for %s. Hibernating...", dep.ID, dep.PRNumber, idleDuration)
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				_ = r.orchestrator.Sleep(ctx, dep.ID)
				cancel()
			}
		}
	}
}

func (r *Reaper) sweepDiskSpaceGuard() {
	if r.workspacesDir == "" || r.dockerRunner == nil {
		return
	}

	usage := GetDiskUsage(r.workspacesDir)
	// If disk utilization exceeds 85% or free disk is below 3GB, trigger auto-prune
	if usage.Percent > 85.0 || (usage.TotalGB > 0 && usage.FreeGB < 3.0) {
		log.Printf("[reaper] Disk Space Alert: %.1f%% used (%.1f GB free). Running proactive Docker cache & image prune...",
			usage.Percent, usage.FreeGB)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		_ = r.dockerRunner.Prune(ctx)
		cancel()
	}
}

func (r *Reaper) sweepOrphanedWorkspaces() {
	if r.workspacesDir == "" {
		return
	}

	entries, err := os.ReadDir(r.workspacesDir)
	if err != nil {
		return
	}

	deployments, err := r.store.ListDeployments()
	if err != nil {
		return
	}

	activeIDs := make(map[string]bool)
	for _, d := range deployments {
		if d.Status == model.StatusRunning || d.Status == model.StatusSleeping || d.Status == model.StatusBuilding {
			activeIDs[d.ID] = true
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirName := entry.Name()
			if !activeIDs[dirName] {
				fullPath := filepath.Join(r.workspacesDir, dirName)
				if info, err := os.Stat(fullPath); err == nil && time.Since(info.ModTime()) > 2*time.Hour {
					_ = os.RemoveAll(fullPath)
					log.Printf("[reaper] Pruned orphaned workspace: %s", fullPath)
				}
			}
		}
	}
}
