package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeBuilder(t *testing.T) {
	tempDir := t.TempDir()

	composeContent := `
services:
  database:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: secret
  web:
    build: .
    ports:
      - "8080:3000"
    depends_on:
      - database
`
	err := os.WriteFile(filepath.Join(tempDir, "docker-compose.yml"), []byte(composeContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test compose file: %v", err)
	}

	builder := NewComposeBuilder()
	inspection, err := builder.InspectAndPrepare(tempDir, "pr-42-test", 42, 31050, "https://pr-42.preview.example.com")
	if err != nil {
		t.Fatalf("inspect failed: %v", err)
	}

	if inspection.ServiceName != "web" {
		t.Fatalf("expected service name web, got %s", inspection.ServiceName)
	}

	if inspection.ContainerPort != 3000 {
		t.Fatalf("expected container port 3000, got %d", inspection.ContainerPort)
	}

	// Verify override file exists and contains port mapping
	overrideBytes, err := os.ReadFile(filepath.Join(tempDir, inspection.OverrideFile))
	if err != nil {
		t.Fatalf("failed to read generated override file: %v", err)
	}

	overrideStr := string(overrideBytes)
	if !strings.Contains(overrideStr, "31050:3000") {
		t.Fatalf("override file does not contain mapped port 31050:3000: %s", overrideStr)
	}
	if !strings.Contains(overrideStr, "previewforge.pr: \"42\"") {
		t.Fatalf("override file does not contain PR label: %s", overrideStr)
	}
}
