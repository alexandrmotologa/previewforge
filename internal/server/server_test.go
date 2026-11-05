package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
	"github.com/alexandrmotologa/previewforge/internal/proxy"
	"github.com/alexandrmotologa/previewforge/internal/storage"
	"github.com/alexandrmotologa/previewforge/internal/webhook"
)

type mockOrchDeployer struct{}

func (m *mockOrchDeployer) Deploy(ctx context.Context, req webhook.DeployRequest) (*model.Deployment, error) {
	return &model.Deployment{
		ID:        "pr-1-test",
		PRNumber:  req.PRNumber,
		RepoName:  req.RepoName,
		RepoOwner: req.RepoOwner,
	}, nil
}

func (m *mockOrchDeployer) Teardown(ctx context.Context, repoOwner, repoName string, prNumber int) error {
	return nil
}

func TestServerEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "previewforge_server.db")

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	cfg := model.SystemConfig{
		BaseDomain:     "preview.example.com",
		PublicPort:     9850,
		PortRangeStart: 31000,
		PortRangeEnd:   32000,
		WorkspacesDir:  tempDir,
	}

	standaloneProxy := proxy.NewStandaloneProxy()
	webhookHandler := webhook.NewHandler(&mockOrchDeployer{}, "", "", false)

	server := NewServer(cfg, store, nil, standaloneProxy, webhookHandler)

	// Pre-populate a running deployment
	_ = store.SaveDeployment(&model.Deployment{
		ID:             "pr-10-api-service",
		PRNumber:       10,
		PRTitle:        "Feature: Add GraphQL",
		RepoOwner:      "alexandrmotologa",
		RepoName:       "api-service",
		Branch:         "feat/graphql",
		CommitSHA:      "deadbeef",
		Author:         "developer",
		Status:         model.StatusRunning,
		Port:           31001,
		Subdomain:      "pr-10.preview.example.com",
		PreviewURL:     "https://pr-10.preview.example.com",
		ServiceName:    "web",
		LastActivityAt: time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	})

	// 1. Test /healthz
	t.Run("Health check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var resp map[string]string
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp["status"] != "healthy" {
			t.Fatalf("expected healthy, got %s", resp["status"])
		}
	})

	// 2. Test /api/deployments
	t.Run("List deployments", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/deployments", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var list []model.Deployment
		_ = json.NewDecoder(w.Body).Decode(&list)
		if len(list) != 1 {
			t.Fatalf("expected 1 deployment, got %d", len(list))
		}
	})

	// 3. Test /api/system/stats
	t.Run("System stats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/system/stats", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var stats model.SystemStats
		_ = json.NewDecoder(w.Body).Decode(&stats)
		if stats.PortPoolTotal <= 0 {
			t.Fatalf("expected positive port pool total, got %d", stats.PortPoolTotal)
		}
	})

	// 4. Test /api/system/config
	t.Run("System config", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/system/config", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var configResp map[string]any
		_ = json.NewDecoder(w.Body).Decode(&configResp)
		if configResp["base_domain"] != "preview.example.com" {
			t.Fatalf("expected base_domain preview.example.com, got %v", configResp["base_domain"])
		}
	})

	// 5. Test Host Header Proxy Delegation
	t.Run("Host header proxy delegation", func(t *testing.T) {
		dummyBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Response from PR container")
		}))
		defer dummyBackend.Close()

		backendParts := strings.Split(dummyBackend.URL, ":")
		backendPort := 0
		fmt.Sscanf(backendParts[len(backendParts)-1], "%d", &backendPort)

		_ = standaloneProxy.RegisterRoute("pr-77.preview.example.com", backendPort)

		req := httptest.NewRequest(http.MethodGet, "http://pr-77.preview.example.com/", nil)
		req.Host = "pr-77.preview.example.com"
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK forwarded from backend, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "Response from PR container") {
			t.Fatalf("unexpected body: %s", w.Body.String())
		}
	})

	// 6. Test Simulator Deploy Endpoint
	t.Run("Simulator deploy", func(t *testing.T) {
		simPayload := map[string]any{
			"pr_number":  10,
			"pr_title":   "Add GraphQL support",
			"repo_owner": "alexandrmotologa",
			"repo_name":  "api-service",
			"branch":     "feat/graphql",
		}
		bodyBytes, _ := json.Marshal(simPayload)

		req := httptest.NewRequest(http.MethodPost, "/api/simulator/deploy", bytes.NewReader(bodyBytes))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d", w.Code)
		}

		var resp map[string]any
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp["status"] != "queued" {
			t.Fatalf("expected queued, got %v", resp["status"])
		}
	})
}
