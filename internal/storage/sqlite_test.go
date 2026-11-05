package storage

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

func TestStorageOperations(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_previewforge.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	// 1. Test Port Allocation
	port, err := store.AllocatePort("pr-1-test", 31000, 31005)
	if err != nil {
		t.Fatalf("failed to allocate port: %v", err)
	}
	if port != 31000 {
		t.Fatalf("expected port 31000, got %d", port)
	}

	// Idempotent allocation for same deployment
	portSame, err := store.AllocatePort("pr-1-test", 31000, 31005)
	if err != nil {
		t.Fatalf("failed to re-allocate port: %v", err)
	}
	if portSame != 31000 {
		t.Fatalf("expected port 31000 for same deployment, got %d", portSame)
	}

	// Next deployment gets next port
	port2, err := store.AllocatePort("pr-2-test", 31000, 31005)
	if err != nil {
		t.Fatalf("failed to allocate port 2: %v", err)
	}
	if port2 != 31001 {
		t.Fatalf("expected port 31001, got %d", port2)
	}

	// 2. Test Deployment Persistence
	dep := &model.Deployment{
		ID:               "pr-1-test",
		PRNumber:         1,
		PRTitle:          "Feature: New Checkout Flow",
		RepoOwner:        "alexandrmotologa",
		RepoName:         "ecommerce-api",
		Branch:           "feature/checkout",
		CommitSHA:        "a1b2c3d4e5",
		Author:           "alexandrmotologa",
		Status:           model.StatusBuilding,
		Port:             port,
		Subdomain:        "pr-1.preview.example.com",
		PreviewURL:       "https://pr-1.preview.example.com",
		ServiceName:      "web",
		BuildDurationSec: 0,
		HealthStatus:     "unknown",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := store.SaveDeployment(dep); err != nil {
		t.Fatalf("failed to save deployment: %v", err)
	}

	// Retrieve
	loaded, err := store.GetDeployment("pr-1-test")
	if err != nil {
		t.Fatalf("failed to get deployment: %v", err)
	}
	if loaded == nil || loaded.PRTitle != dep.PRTitle {
		t.Fatalf("loaded deployment mismatch: %+v", loaded)
	}

	// Query by PR
	byPR, err := store.GetDeploymentByPR("alexandrmotologa", "ecommerce-api", 1)
	if err != nil {
		t.Fatalf("failed to get deployment by PR: %v", err)
	}
	if byPR == nil || byPR.ID != dep.ID {
		t.Fatalf("expected deployment pr-1-test, got %+v", byPR)
	}

	// 3. Test Logs
	if err := store.AppendLog(dep.ID, "Cloning repository..."); err != nil {
		t.Fatalf("failed to append log: %v", err)
	}
	if err := store.AppendLog(dep.ID, "Building docker image..."); err != nil {
		t.Fatalf("failed to append log 2: %v", err)
	}

	logs, err := store.GetLogs(dep.ID, 100)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(logs))
	}

	// 4. Test Port Release
	if err := store.ReleasePort(port); err != nil {
		t.Fatalf("failed to release port: %v", err)
	}

	// New deployment should be able to reuse port 31000
	portReused, err := store.AllocatePort("pr-3-test", 31000, 31005)
	if err != nil {
		t.Fatalf("failed to allocate port: %v", err)
	}
	if portReused != 31000 {
		t.Fatalf("expected reused port 31000, got %d", portReused)
	}

	// 5. Delete deployment
	if err := store.DeleteDeployment("pr-1-test"); err != nil {
		t.Fatalf("failed to delete deployment: %v", err)
	}

	deleted, err := store.GetDeployment("pr-1-test")
	if err != nil {
		t.Fatalf("unexpected error getting deleted deployment: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected nil for deleted deployment, got %+v", deleted)
	}
}
