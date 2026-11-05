package github

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

const stickyMarker = "<!-- previewforge:status -->"

// Commenter manages sticky GitHub PR comments, commit status checks, and native GitHub Deployments.
type Commenter struct {
	client        *Client
	deploymentsMu sync.Mutex
	deploymentIDs map[string]int64 // deployment ID -> GitHub Deployment ID
}

// NewCommenter constructs a Commenter.
func NewCommenter(client *Client) *Commenter {
	return &Commenter{
		client:        client,
		deploymentIDs: make(map[string]int64),
	}
}

// SetCommitStatus publishes a status check on the commit SHA.
func (c *Commenter) SetCommitStatus(owner, repo, sha, state, targetURL, description string) {
	if c.client == nil {
		return
	}
	err := c.client.CreateCommitStatus(owner, repo, sha, state, targetURL, description, "previewforge/deploy")
	if err != nil {
		log.Printf("[github] Failed to set commit status: %v", err)
	}
}

// PostBuilding posts or updates the sticky comment when deployment begins and registers GitHub Deployment.
func (c *Commenter) PostBuilding(d *model.Deployment) {
	body := fmt.Sprintf(`### PreviewForge Deployment
- **Status**: Building...
- **Branch**: %s
- **Commit**: `+"`%s`"+`
---
Building Docker Compose stack in an isolated network.
%s`, d.Branch, truncateSHA(d.CommitSHA), stickyMarker)

	c.upsertStickyComment(d.RepoOwner, d.RepoName, d.PRNumber, body)

	// Register Native GitHub Deployment
	if c.client != nil {
		envName := fmt.Sprintf("preview/pr-%d", d.PRNumber)
		ghDepID, err := c.client.CreateDeployment(d.RepoOwner, d.RepoName, d.Branch, envName)
		if err == nil && ghDepID > 0 {
			c.deploymentsMu.Lock()
			c.deploymentIDs[d.ID] = ghDepID
			c.deploymentsMu.Unlock()
			_ = c.client.CreateDeploymentStatus(d.RepoOwner, d.RepoName, ghDepID, "in_progress", d.PreviewURL, "Building preview stack")
		}
	}
}

// PostReady updates the sticky comment with the live URL when deployment is successful.
func (c *Commenter) PostReady(d *model.Deployment) {
	body := fmt.Sprintf(`### PreviewForge Deployment
- **Status**: Ready
- **Preview URL**: [%s](%s)
- **Branch**: %s
- **Commit**: `+"`%s`"+`
- **Build duration**: %ds
---
Ephemeral preview environment is live. Containers will be stopped when this pull request closes.
%s`, d.PreviewURL, d.PreviewURL, d.Branch, truncateSHA(d.CommitSHA), d.BuildDurationSec, stickyMarker)

	c.upsertStickyComment(d.RepoOwner, d.RepoName, d.PRNumber, body)

	if c.client != nil {
		c.deploymentsMu.Lock()
		ghDepID := c.deploymentIDs[d.ID]
		c.deploymentsMu.Unlock()
		if ghDepID > 0 {
			_ = c.client.CreateDeploymentStatus(d.RepoOwner, d.RepoName, ghDepID, "success", d.PreviewURL, "Preview environment ready")
		}
	}
}

// PostFailed updates the sticky comment when build or healthcheck fails.
func (c *Commenter) PostFailed(d *model.Deployment, logSnippet string) {
	if logSnippet != "" {
		logSnippet = fmt.Sprintf("\n<details><summary>Recent build logs</summary>\n\n```\n%s\n```\n</details>", logSnippet)
	}

	body := fmt.Sprintf(`### PreviewForge Deployment
- **Status**: Failed
- **Branch**: %s
- **Commit**: `+"`%s`"+`
- **Error**: %s
---%s
%s`, d.Branch, truncateSHA(d.CommitSHA), d.ErrorMessage, logSnippet, stickyMarker)

	c.upsertStickyComment(d.RepoOwner, d.RepoName, d.PRNumber, body)

	if c.client != nil {
		c.deploymentsMu.Lock()
		ghDepID := c.deploymentIDs[d.ID]
		c.deploymentsMu.Unlock()
		if ghDepID > 0 {
			_ = c.client.CreateDeploymentStatus(d.RepoOwner, d.RepoName, ghDepID, "failure", d.PreviewURL, d.ErrorMessage)
		}
	}
}

// PostDestroyed updates the sticky comment when the preview environment is torn down.
func (c *Commenter) PostDestroyed(repoOwner, repoName string, prNumber int) {
	body := fmt.Sprintf(`### PreviewForge Deployment
- **Status**: Closed
---
Ephemeral preview environment stopped and disk resources reclaimed.
%s`, stickyMarker)

	c.upsertStickyComment(repoOwner, repoName, prNumber, body)

	deploymentID := fmt.Sprintf("pr-%d-%s", prNumber, repoName)
	if c.client != nil {
		c.deploymentsMu.Lock()
		ghDepID := c.deploymentIDs[deploymentID]
		delete(c.deploymentIDs, deploymentID)
		c.deploymentsMu.Unlock()
		if ghDepID > 0 {
			_ = c.client.CreateDeploymentStatus(repoOwner, repoName, ghDepID, "inactive", "", "Preview environment destroyed")
		}
	}
}

func (c *Commenter) upsertStickyComment(owner, repo string, prNumber int, body string) {
	if c.client == nil {
		return
	}

	comments, err := c.client.ListPRComments(owner, repo, prNumber)
	if err != nil {
		log.Printf("[github] Failed to query PR comments for sticky update: %v", err)
		return
	}

	for _, comment := range comments {
		if strings.Contains(comment.Body, stickyMarker) {
			updateErr := c.client.UpdatePRComment(owner, repo, comment.ID, body)
			if updateErr != nil {
				log.Printf("[github] Failed to update sticky comment %d: %v", comment.ID, updateErr)
			}
			return
		}
	}

	// No existing comment found, create new
	createErr := c.client.CreatePRComment(owner, repo, prNumber, body)
	if createErr != nil {
		log.Printf("[github] Failed to create new sticky comment: %v", createErr)
	}
}

func truncateSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// FormatDuration formats seconds into a readable string.
func FormatDuration(d time.Duration) string {
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
