package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

// Notifier dispatches event broadcasts to Discord and Slack webhooks.
type Notifier struct {
	discordURL string
	slackURL   string
	httpClient *http.Client
}

// NewNotifier constructs a Notifier.
func NewNotifier(discordURL, slackURL string) *Notifier {
	return &Notifier{
		discordURL: discordURL,
		slackURL:   slackURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// NotifyReady broadcasts that a preview deployment is live.
func (n *Notifier) NotifyReady(d *model.Deployment) {
	title := fmt.Sprintf("✅ Preview Live: %s/%s PR #%d", d.RepoOwner, d.RepoName, d.PRNumber)
	desc := fmt.Sprintf("**Title**: %s\n**URL**: %s\n**Branch**: `%s`\n**Commit**: `%s`\n**Duration**: %ds",
		d.PRTitle, d.PreviewURL, d.Branch, truncate(d.CommitSHA, 7), d.BuildDurationSec)

	n.sendDiscord(title, desc, 0x22c55e, d.PreviewURL)
	n.sendSlack(fmt.Sprintf("%s\n%s", title, desc))
}

// NotifyFailed broadcasts that a preview build failed.
func (n *Notifier) NotifyFailed(d *model.Deployment) {
	title := fmt.Sprintf("❌ Preview Build Failed: %s/%s PR #%d", d.RepoOwner, d.RepoName, d.PRNumber)
	desc := fmt.Sprintf("**Title**: %s\n**Branch**: `%s`\n**Error**: %s",
		d.PRTitle, d.Branch, d.ErrorMessage)

	n.sendDiscord(title, desc, 0xef4444, "")
	n.sendSlack(fmt.Sprintf("%s\n%s", title, desc))
}

// NotifyDestroyed broadcasts that a preview environment was cleaned up.
func (n *Notifier) NotifyDestroyed(repoOwner, repoName string, prNumber int) {
	title := fmt.Sprintf("🧹 Preview Environment Cleaned: %s/%s PR #%d", repoOwner, repoName, prNumber)
	desc := "Containers stopped, networks removed, and disk space reclaimed."

	n.sendDiscord(title, desc, 0x64748b, "")
	n.sendSlack(fmt.Sprintf("%s\n%s", title, desc))
}

func (n *Notifier) sendDiscord(title, description string, color int, url string) {
	if n.discordURL == "" {
		return
	}

	payload := map[string]any{
		"embeds": []map[string]any{
			{
				"title":       title,
				"description": description,
				"color":       color,
				"url":         url,
				"footer": map[string]string{
					"text": "PreviewForge Ephemeral Supervisor",
				},
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	n.postJSON(n.discordURL, payload)
}

func (n *Notifier) sendSlack(text string) {
	if n.slackURL == "" {
		return
	}

	payload := map[string]string{
		"text": text,
	}

	n.postJSON(n.slackURL, payload)
}

func (n *Notifier) postJSON(endpoint string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
