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
	upCalled    bool
	downCalled  bool
	stopCalled  bool
	startCalled bool
	execCmd     string
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

func (m *mockDockerRunner) Stop(ctx context.Context, projectName, workspaceDir, baseFile, overrideFile string, logFn func(string)) error {
	m.stopCalled = true
	logFn("Mock docker compose stop success")
	return nil
}

func (m *mockDockerRunner) Start(ctx context.Context, projectName, workspaceDir, baseFile, overrideFile string, logFn func(string)) error {
	m.startCalled = true
	logFn("Mock docker compose start success")
	return nil
}

func (m *mockDockerRunner) Exec(ctx context.Context, projectName, service string, command []string) (string, error) {
	m.execCmd = fmt.Sprintf("%s: %v", service, command)
	return "Migration applied successfully", nil
}

func (m *mockDockerRunner) Prune(ctx context.Context) error {
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
		nil,
		broadcaster,
	)

	// Pre-create the workspace with a mock docker-compose.yml and .previewforge.yml
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

	repoCfg := `
service: "web"
port: 80
post_deploy:
  command: "npm run db:migrate"
  service: "web"
`
	_ = os.WriteFile(filepath.Join(workspaceDir, ".previewforge.yml"), []byte(repoCfg), 0644)

	// Start a background server on targetPort simulating running container
	srvDone := make(chan struct{})
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
		<-srvDone
		_ = srv.Close()
	}()
	defer close(srvDone)

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

	// Verify post-deploy hook was executed
	if dockerRunner.execCmd == "" {
		t.Fatalf("expected post-deploy hook to execute")
	}

	// 2. Test Container Exec endpoint
	execOut, err := orchestrator.Exec(ctx, dep.ID, "web", []string{"echo", "hello"})
	if err != nil || execOut != "Migration applied successfully" {
		t.Fatalf("exec failed: %v, out: %s", err, execOut)
	}

	// 3. Test Sleep (Hibernation)
	err = orchestrator.Sleep(ctx, dep.ID)
	if err != nil {
		t.Fatalf("sleep failed: %v", err)
	}
	if !dockerRunner.stopCalled {
		t.Fatalf("expected docker compose stop to be called")
	}

	sleepingDep, _ := store.GetDeployment(dep.ID)
	if sleepingDep.Status != model.StatusSleeping {
		t.Fatalf("expected status sleeping, got %s", sleepingDep.Status)
	}

	// 4. Test Wake (Resumption)
	err = orchestrator.Wake(ctx, dep.ID)
	if err != nil {
		t.Fatalf("wake failed: %v", err)
	}
	if !dockerRunner.startCalled {
		t.Fatalf("expected docker compose start to be called")
	}

	wokenDep, _ := store.GetDeployment(dep.ID)
	if wokenDep.Status != model.StatusRunning {
		t.Fatalf("expected status running, got %s", wokenDep.Status)
	}

	// 5. Teardown
	err = orchestrator.Teardown(ctx, req.RepoOwner, req.RepoName, req.PRNumber)
	if err != nil {
		t.Fatalf("teardown failed: %v", err)
	}

	loadedDep, _ := store.GetDeployment(dep.ID)
	if loadedDep.Status != model.StatusDestroyed {
		t.Fatalf("expected status destroyed, got %s", loadedDep.Status)
	}
}
