package notification

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

func TestNotifier(t *testing.T) {
	var mu sync.Mutex
	var receivedDiscordPayload map[string]any
	var receivedSlackPayload map[string]any

	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedDiscordPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer discordServer.Close()

	slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedSlackPayload)
		w.WriteHeader(http.StatusOK)
	}))
	defer slackServer.Close()

	notifier := NewNotifier(discordServer.URL, slackServer.URL)

	dep := &model.Deployment{
		ID:               "pr-12-test",
		PRNumber:         12,
		PRTitle:          "Feature: New Cart",
		RepoOwner:        "alexandrmotologa",
		RepoName:         "cart-service",
		Branch:           "feature/cart",
		CommitSHA:        "1234567890",
		PreviewURL:       "https://pr-12.preview.example.com",
		BuildDurationSec: 18,
	}

	// 1. Notify Ready
	notifier.NotifyReady(dep)

	mu.Lock()
	if receivedDiscordPayload == nil {
		t.Fatalf("expected discord payload to be received")
	}
	if receivedSlackPayload == nil {
		t.Fatalf("expected slack payload to be received")
	}
	mu.Unlock()

	// 2. Notify Failed
	dep.ErrorMessage = "Build step failed"
	notifier.NotifyFailed(dep)

	// 3. Notify Destroyed
	notifier.NotifyDestroyed("alexandrmotologa", "cart-service", 12)
}
