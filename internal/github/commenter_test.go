package github

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

func TestGitHubCommenterSticky(t *testing.T) {
	var mu sync.Mutex
	var postedComments []string
	var patchedComments = make(map[int64]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if strings.HasSuffix(r.URL.Path, "/issues/42/comments") && r.Method == http.MethodGet {
			// If we have posted comments, return them
			var list []IssueComment
			for i, body := range postedComments {
				list = append(list, IssueComment{
					ID:   int64(i + 1),
					Body: body,
				})
			}
			_ = json.NewEncoder(w).Encode(list)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/issues/42/comments") && r.Method == http.MethodPost {
			var payload map[string]string
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &payload)
			postedComments = append(postedComments, payload["body"])
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": len(postedComments)})
			return
		}

		if strings.Contains(r.URL.Path, "/issues/comments/") && r.Method == http.MethodPatch {
			var payload map[string]string
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &payload)
			patchedComments[1] = payload["body"]
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
			return
		}

		if strings.Contains(r.URL.Path, "/statuses/") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient("mock-token")
	client.SetBaseURL(server.URL)

	commenter := NewCommenter(client)

	dep := &model.Deployment{
		ID:               "pr-42-test",
		PRNumber:         42,
		RepoOwner:        "alexandrmotologa",
		RepoName:         "web-app",
		Branch:           "feature/auth",
		CommitSHA:        "1234567890abcdef",
		Status:           model.StatusBuilding,
		PreviewURL:       "https://pr-42.preview.example.com",
		BuildDurationSec: 25,
	}

	// 1. First call: Should POST new comment
	commenter.PostBuilding(dep)

	mu.Lock()
	if len(postedComments) != 1 {
		t.Fatalf("expected 1 posted comment, got %d", len(postedComments))
	}
	if !strings.Contains(postedComments[0], stickyMarker) {
		t.Fatalf("comment missing sticky marker")
	}
	mu.Unlock()

	// 2. Second call: Should PATCH existing comment instead of creating a second comment!
	commenter.PostReady(dep)

	mu.Lock()
	if len(postedComments) != 1 {
		t.Fatalf("expected still 1 posted comment (updated via PATCH), got %d", len(postedComments))
	}
	if patchedComments[1] == "" || !strings.Contains(patchedComments[1], "Status**: Ready") {
		t.Fatalf("comment was not updated to Ready: %s", patchedComments[1])
	}
	mu.Unlock()
}
