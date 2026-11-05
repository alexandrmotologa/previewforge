package orchestrator

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
	"github.com/alexandrmotologa/previewforge/internal/proxy"
	"github.com/alexandrmotologa/previewforge/internal/storage"
	"github.com/alexandrmotologa/previewforge/internal/webhook"
)

type mockDockerRunner struct {
	upCalled   bool
	downCalled bool
}

func (m *mockDockerRunner) Up(ctx context.Context, projectName, workspaceDir, baseFile, overrideFile string, logFn func(string)) error {
	m.upCalled = true
	logFn("Mock docker compose up success")
	return nil
}

func (m *mockDockerRunner) Down(ctx context.Context, projectName, workspaceDir, baseFile, overrideFile string, logFn func(string)) error {
	m.downCalled = true
	logFn("Mock docker compose down success")
	return nil
}

func (m *mockDockerRunner) Stats(ctx context.Context, projectName string) (*ContainerStats, error) {
	return &ContainerStats{CPUPercent: 1.5, MemoryMB: 64.0}, nil
}

type mockGitFetcher struct{}

func (m *mockGitFetcher) CloneOrUpdate(ctx context.Context, cloneURL, branch, commitSHA, targetDir string, logFn func(string)) error {
	logFn("Mock git checkout success")
	return nil
}

func TestOrchestratorLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "previewforge.db")
	workspacesDir := filepath.Join(tempDir, "workspaces")

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	targetPort := 32150

	cfg := model.SystemConfig{
		BaseDomain:     "preview.example.com",
		PublicPort:     9850,
		PortRangeStart: targetPort,
		PortRangeEnd:   targetPort,
		WorkspacesDir:  workspacesDir,
	}

	allocator := NewPortAllocator(store, targetPort, targetPort)
	gitCloner := &mockGitFetcher{}
	composeBuilder := NewComposeBuilder()
	dockerRunner := &mockDockerRunner{}
	healthChecker := NewHealthChecker()
	standaloneProxy := proxy.NewStandaloneProxy()
	broadcaster := NewLogBroadcaster()

	orchestrator := NewOrchestrator(
		store,
		cfg,
		allocator,
		gitCloner,
		composeBuilder,
		dockerRunner,
		healthChecker,
		standaloneProxy,
		nil,
		broadcaster,
	)

	// Pre-create the workspace with a mock docker-compose.yml
	workspaceDir := filepath.Join(workspacesDir, "pr-42-storefront")
	_ = os.MkdirAll(workspaceDir, 0755)
	composeContent := `
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
`
	_ = os.WriteFile(filepath.Join(workspaceDir, "docker-compose.yml"), []byte(composeContent), 0644)

	// Start a background server on targetPort after a brief delay simulating container startup
	go func() {
		time.Sleep(100 * time.Millisecond)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", targetPort))
		if err != nil {
			return
		}
		defer ln.Close()

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "OK from preview container")
		})
		srv := &http.Server{Handler: mux}
		go srv.Serve(ln)
		time.Sleep(2 * time.Second)
		_ = srv.Close()
	}()

	// 1. Deploy
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := webhook.DeployRequest{
		PRNumber:  42,
		PRTitle:   "Implement payment gateway",
		RepoOwner: "alexandrmotologa",
		RepoName:  "storefront",
		CloneURL:  "https://github.com/alexandrmotologa/storefront.git",
		Branch:    "feat/payments",
		CommitSHA: "abcdef123456",
		Author:    "alexandrmotologa",
	}

	dep, err := orchestrator.Deploy(ctx, req)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	if dep.Status != model.StatusRunning {
		t.Fatalf("expected status running, got %s", dep.Status)
	}

	if dep.Port != targetPort {
		t.Fatalf("expected port %d, got %d", targetPort, dep.Port)
	}

	// Verify route in proxy
	foundPort, found := standaloneProxy.GetTarget(dep.Subdomain)
	if !found || foundPort != targetPort {
		t.Fatalf("expected proxy route to port %d, got %d", targetPort, foundPort)
	}

	// 2. Teardown
	err = orchestrator.Teardown(ctx, req.RepoOwner, req.RepoName, req.PRNumber)
	if err != nil {
		t.Fatalf("teardown failed: %v", err)
	}

	// Verify proxy route removed
	_, foundAfter := standaloneProxy.GetTarget(dep.Subdomain)
	if foundAfter {
		t.Fatalf("expected proxy route to be removed after teardown")
	}

	// Verify status in DB is destroyed
	loadedDep, _ := store.GetDeployment(dep.ID)
	if loadedDep.Status != model.StatusDestroyed {
		t.Fatalf("expected status destroyed, got %s", loadedDep.Status)
	}
}
