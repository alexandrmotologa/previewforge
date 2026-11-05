package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

type mockDeployer struct {
	mu           sync.Mutex
	deployedReq  *DeployRequest
	teardownRepo string
	teardownPR   int
}

func (m *mockDeployer) Deploy(ctx context.Context, req DeployRequest) (*model.Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deployedReq = &req
	return &model.Deployment{
		ID:        "test-dep",
		PRNumber:  req.PRNumber,
		RepoOwner: req.RepoOwner,
		RepoName:  req.RepoName,
	}, nil
}

func (m *mockDeployer) Teardown(ctx context.Context, repoOwner, repoName string, prNumber int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.teardownRepo = repoOwner + "/" + repoName
	m.teardownPR = prNumber
	return nil
}

func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHandler(t *testing.T) {
	secret := "super-secret-key"
	deployer := &mockDeployer{}
	handler := NewHandler(deployer, secret, "", false)

	// 1. Test Ping Event
	t.Run("Ping event", func(t *testing.T) {
		body := []byte(`{"zen": "Mind your words, they become actions."}`)
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "ping")
		req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for ping, got %d", w.Code)
		}
	})

	// 2. Test Invalid Signature
	t.Run("Invalid signature", func(t *testing.T) {
		body := []byte(`{"action": "opened"}`)
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "pull_request")
		req.Header.Set("X-Hub-Signature-256", "sha256=invalid-signature")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for bad signature, got %d", w.Code)
		}
	})

	// 3. Test Pull Request Opened Event
	t.Run("Pull request opened", func(t *testing.T) {
		payload := PullRequestPayload{
			Action: "opened",
			Number: 42,
			PullRequest: PullRequest{
				Number: 42,
				Title:  "Add stripe integration",
				User:   User{Login: "developer"},
				Head: PRHead{
					Ref: "feat/stripe",
					SHA: "deadbeef1234",
					Repo: Repository{
						Fork: false,
					},
				},
			},
			Repository: Repository{
				Name:     "storefront",
				CloneURL: "https://github.com/alexandrmotologa/storefront.git",
				Owner:    User{Login: "alexandrmotologa"},
			},
			Sender: User{Login: "developer"},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "pull_request")
		req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d", w.Code)
		}
	})

	// 4. Test Pull Request Closed Event
	t.Run("Pull request closed", func(t *testing.T) {
		payload := PullRequestPayload{
			Action: "closed",
			Number: 42,
			PullRequest: PullRequest{
				Number: 42,
				Title:  "Add stripe integration",
				Merged: true,
			},
			Repository: Repository{
				Name:  "storefront",
				Owner: User{Login: "alexandrmotologa"},
			},
			Sender: User{Login: "developer"},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "pull_request")
		req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted, got %d", w.Code)
		}
	})

	// 5. Test Required Label Filter
	t.Run("Required label filter", func(t *testing.T) {
		labelHandler := NewHandler(deployer, secret, "preview-ready", false)

		payload := PullRequestPayload{
			Action: "opened",
			Number: 99,
			PullRequest: PullRequest{
				Number: 99,
				Title:  "Draft Work",
				Labels: []Label{{Name: "wip"}},
			},
			Repository: Repository{
				Name:  "storefront",
				Owner: User{Login: "alexandrmotologa"},
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
		req.Header.Set("X-GitHub-Event", "pull_request")
		req.Header.Set("X-Hub-Signature-256", signPayload(secret, body))
		w := httptest.NewRecorder()

		labelHandler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK ignored, got %d", w.Code)
		}
	})
}
