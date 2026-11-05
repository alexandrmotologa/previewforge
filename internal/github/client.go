package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// IssueComment represents a GitHub issue or pull request comment.
type IssueComment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
}

// Client wraps GitHub REST API calls for status checks and PR comments.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient constructs a GitHub API client.
func NewClient(token string) *Client {
	return &Client{
		token:   token,
		baseURL: "https://api.github.com",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetBaseURL overrides the GitHub API endpoint (useful for mock testing).
func (c *Client) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

// CreateCommitStatus updates the commit status check (e.g. pending, success, failure).
func (c *Client) CreateCommitStatus(owner, repo, sha, state, targetURL, description, contextStr string) error {
	if c.token == "" {
		return nil
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/statuses/%s", c.baseURL, owner, repo, sha)
	payload := map[string]string{
		"state":       state,
		"target_url":  targetURL,
		"description": description,
		"context":     contextStr,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create commit status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api status check error %d: %s", resp.StatusCode, string(respBytes))
	}

	return nil
}

// ListPRComments retrieves comments on a pull request.
func (c *Client) ListPRComments(owner, repo string, prNumber int) ([]IssueComment, error) {
	if c.token == "" {
		return nil, nil
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.baseURL, owner, repo, prNumber)
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github api list comments error %d: %s", resp.StatusCode, string(respBytes))
	}

	var comments []IssueComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return nil, err
	}

	return comments, nil
}

// CreatePRComment posts a new comment on a pull request.
func (c *Client) CreatePRComment(owner, repo string, prNumber int, body string) error {
	if c.token == "" {
		return nil
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.baseURL, owner, repo, prNumber)
	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api post comment error %d: %s", resp.StatusCode, string(respBytes))
	}

	return nil
}

// UpdatePRComment modifies an existing comment by ID.
func (c *Client) UpdatePRComment(owner, repo string, commentID int64, body string) error {
	if c.token == "" {
		return nil
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d", c.baseURL, owner, repo, commentID)
	payload := map[string]string{"body": body}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api patch comment error %d: %s", resp.StatusCode, string(respBytes))
	}

	return nil
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "PreviewForge-Supervisor/1.0")
	req.Header.Set("Content-Type", "application/json")
}
