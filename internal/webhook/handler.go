package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

// Deployer executes deployments and teardowns for pull requests.
type Deployer interface {
	Deploy(ctx context.Context, req DeployRequest) (*model.Deployment, error)
	Teardown(ctx context.Context, repoOwner, repoName string, prNumber int) error
}

// DeployRequest contains parameters for creating or updating a preview deployment.
type DeployRequest struct {
	PRNumber  int
	PRTitle   string
	RepoOwner string
	RepoName  string
	CloneURL  string
	Branch    string
	CommitSHA string
	Author    string
}

// Handler handles incoming GitHub webhook notifications.
type Handler struct {
	deployer            Deployer
	webhookSecret       string
	requireLabel        string
	allowUntrustedForks bool
}

// NewHandler constructs a new webhook handler.
func NewHandler(deployer Deployer, webhookSecret, requireLabel string, allowUntrustedForks bool) *Handler {
	return &Handler{
		deployer:            deployer,
		webhookSecret:       webhookSecret,
		requireLabel:        requireLabel,
		allowUntrustedForks: allowUntrustedForks,
	}
}

// ServeHTTP implements http.Handler for /webhook.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	sigHeader := r.Header.Get("X-Hub-Signature-256")
	if !VerifySignature(h.webhookSecret, body, sigHeader) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "ping" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "pong"})
		return
	}

	if eventType != "pull_request" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ignored", "reason": "unhandled event type"})
		return
	}

	payload, err := ParsePullRequestPayload(body)
	if err != nil {
		http.Error(w, "Invalid pull request payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	repoOwner := payload.Repository.Owner.Login
	repoName := payload.Repository.Name
	prNumber := payload.Number

	// Security check for forks
	if !IsForkAllowed(payload.PullRequest.Head.Repo.Fork, h.allowUntrustedForks, repoOwner, payload.Sender.Login) {
		log.Printf("[webhook] Untrusted fork PR #%d from %s rejected", prNumber, payload.Sender.Login)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "rejected", "reason": "untrusted fork"})
		return
	}

	// Label requirement check
	if h.requireLabel != "" && !payload.PullRequest.HasLabel(h.requireLabel) && payload.Action != "closed" {
		log.Printf("[webhook] PR #%d ignored: missing required label '%s'", prNumber, h.requireLabel)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ignored", "reason": "missing required label"})
		return
	}

	switch payload.Action {
	case "opened", "synchronize", "reopened", "labeled":
		req := DeployRequest{
			PRNumber:  prNumber,
			PRTitle:   payload.PullRequest.Title,
			RepoOwner: repoOwner,
			RepoName:  repoName,
			CloneURL:  payload.Repository.CloneURL,
			Branch:    payload.PullRequest.Head.Ref,
			CommitSHA: payload.PullRequest.Head.SHA,
			Author:    payload.PullRequest.User.Login,
		}

		// Asynchronous deploy to prevent webhook timeout
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			if _, err := h.deployer.Deploy(ctx, req); err != nil {
				log.Printf("[webhook] Deployment failed for PR #%d: %v", prNumber, err)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "queued",
			"action":    payload.Action,
			"pr_number": prNumber,
		})

	case "closed", "unlabeled":
		if payload.Action == "unlabeled" && h.requireLabel != "" && payload.PullRequest.HasLabel(h.requireLabel) {
			// If unlabeled event occurred but required label is still present, do nothing
			w.WriteHeader(http.StatusOK)
			return
		}

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			if err := h.deployer.Teardown(ctx, repoOwner, repoName, prNumber); err != nil {
				log.Printf("[webhook] Teardown failed for PR #%d: %v", prNumber, err)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "teardown_queued",
			"action":    payload.Action,
			"pr_number": prNumber,
		})

	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ignored",
			"action": payload.Action,
		})
	}
}
