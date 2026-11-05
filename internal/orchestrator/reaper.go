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

// Reaper runs background sweeps to terminate abandoned or timed-out preview stacks.
type Reaper struct {
	store         *storage.Store
	deployer      webhookDeployer
	workspacesDir string
	idleTimeout   time.Duration
	stopChan      chan struct{}
}

type webhookDeployer interface {
	Teardown(ctx context.Context, repoOwner, repoName string, prNumber int) error
}

// NewReaper constructs a Reaper instance.
func NewReaper(store *storage.Store, deployer webhookDeployer, workspacesDir string, idleTimeoutMinutes int) *Reaper {
	if idleTimeoutMinutes <= 0 {
		idleTimeoutMinutes = 1440 // 24 hours default
	}
	return &Reaper{
		store:         store,
		deployer:      deployer,
		workspacesDir: workspacesDir,
		idleTimeout:   time.Duration(idleTimeoutMinutes) * time.Minute,
		stopChan:      make(chan struct{}),
	}
}

// Start launches the background reaper ticker.
func (r *Reaper) Start(interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
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
		if (dep.Status == model.StatusRunning || dep.Status == model.StatusBuilding) && dep.UpdatedAt.Before(cutoff) {
			log.Printf("[reaper] Stale deployment %s (PR #%d) exceeded idle timeout of %s. Reaping...", dep.ID, dep.PRNumber, r.idleTimeout)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			_ = r.deployer.Teardown(ctx, dep.RepoOwner, dep.RepoName, dep.PRNumber)
			cancel()
		}
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
		if d.Status == model.StatusRunning || d.Status == model.StatusBuilding {
			activeIDs[d.ID] = true
		}
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirName := entry.Name()
			if !activeIDs[dirName] {
				fullPath := filepath.Join(r.workspacesDir, dirName)
				// Only remove if older than 2 hours to avoid race conditions with in-flight clones
				if info, err := os.Stat(fullPath); err == nil && time.Since(info.ModTime()) > 2*time.Hour {
					_ = os.RemoveAll(fullPath)
					log.Printf("[reaper] Pruned orphaned workspace: %s", fullPath)
				}
			}
		}
	}
}
