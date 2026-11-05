package github

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

const stickyMarker = "<!-- previewforge:status -->"

// Commenter manages sticky GitHub PR comments and commit status checks.
type Commenter struct {
	client *Client
}

// NewCommenter constructs a Commenter.
func NewCommenter(client *Client) *Commenter {
	return &Commenter{client: client}
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

// PostBuilding posts or updates the sticky comment when deployment begins.
func (c *Commenter) PostBuilding(d *model.Deployment) {
	body := fmt.Sprintf(`### PreviewForge Deployment
- **Status**: Building...
- **Branch**: %s
- **Commit**: `+"`%s`"+`
---
Building Docker Compose stack in an isolated network.
%s`, d.Branch, truncateSHA(d.CommitSHA), stickyMarker)

	c.upsertStickyComment(d.RepoOwner, d.RepoName, d.PRNumber, body)
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
}

// PostDestroyed updates the sticky comment when the preview environment is torn down.
func (c *Commenter) PostDestroyed(repoOwner, repoName string, prNumber int) {
	body := fmt.Sprintf(`### PreviewForge Deployment
- **Status**: Closed
---
Ephemeral preview environment stopped and disk resources reclaimed.
%s`, stickyMarker)

	c.upsertStickyComment(repoOwner, repoName, prNumber, body)
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
