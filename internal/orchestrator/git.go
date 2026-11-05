package orchestrator

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitFetcher defines the contract for fetching pull request source code.
type GitFetcher interface {
	CloneOrUpdate(ctx context.Context, cloneURL, branch, commitSHA, targetDir string, logFn func(string)) error
}

// SystemGitCloner manages shallow cloning and branch updates via the git CLI.
type SystemGitCloner struct {
	githubToken string
}

// NewSystemGitCloner constructs a SystemGitCloner.
func NewSystemGitCloner(token string) *SystemGitCloner {
	return &SystemGitCloner{githubToken: token}
}

// CloneOrUpdate pulls the latest PR commit into the target workspace directory.
func (g *SystemGitCloner) CloneOrUpdate(ctx context.Context, cloneURL, branch, commitSHA, targetDir string, logFn func(string)) error {
	authenticatedURL := g.injectToken(cloneURL)

	// If workspace already has .git, fetch and checkout
	gitDir := filepath.Join(targetDir, ".git")
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		logFn(fmt.Sprintf("Existing workspace found at %s. Fetching commit %s...", targetDir, commitSHA))
		return g.updateExisting(ctx, authenticatedURL, branch, commitSHA, targetDir, logFn)
	}

	// Clean target directory if partially populated
	_ = os.RemoveAll(targetDir)
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("failed to create parent dir: %w", err)
	}

	logFn(fmt.Sprintf("Cloning %s (branch: %s, commit: %s)...", cloneURL, branch, commitSHA))

	// Shallow clone
	args := []string{"clone", "--depth", "1", "--branch", branch, authenticatedURL, targetDir}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = filepath.Dir(targetDir)

	out, err := cmd.CombinedOutput()
	if err != nil {
		// If branch clone failed (e.g. detached PR head), try fetching the specific commit
		logFn(fmt.Sprintf("Direct branch clone failed: %s. Attempting shallow clone without branch...", strings.TrimSpace(string(out))))
		_ = os.RemoveAll(targetDir)

		initCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", authenticatedURL, targetDir)
		if initOut, initErr := initCmd.CombinedOutput(); initErr != nil {
			return fmt.Errorf("git clone failed: %s: %w", string(initOut), initErr)
		}

		if commitSHA != "" {
			fetchCmd := exec.CommandContext(ctx, "git", "fetch", "--depth", "1", "origin", commitSHA)
			fetchCmd.Dir = targetDir
			_ = fetchCmd.Run()

			checkoutCmd := exec.CommandContext(ctx, "git", "checkout", "-f", commitSHA)
			checkoutCmd.Dir = targetDir
			if checkOut, checkErr := checkoutCmd.CombinedOutput(); checkErr != nil {
				logFn(fmt.Sprintf("Warning: checkout %s failed (%s), using default branch", commitSHA, strings.TrimSpace(string(checkOut))))
			}
		}
	} else {
		logFn("Repository cloned successfully.")
	}

	return nil
}

func (g *SystemGitCloner) updateExisting(ctx context.Context, authenticatedURL, branch, commitSHA, targetDir string, logFn func(string)) error {
	fetchArgs := []string{"fetch", "--depth", "1", authenticatedURL, branch}
	fetchCmd := exec.CommandContext(ctx, "git", fetchArgs...)
	fetchCmd.Dir = targetDir
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		logFn(fmt.Sprintf("Fetch failed: %s. Re-cloning workspace...", strings.TrimSpace(string(out))))
		_ = os.RemoveAll(targetDir)
		return g.CloneOrUpdate(ctx, authenticatedURL, branch, commitSHA, targetDir, logFn)
	}

	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", "-f", "FETCH_HEAD")
	checkoutCmd.Dir = targetDir
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout failed: %s: %w", string(out), err)
	}

	logFn("Workspace updated to latest commit.")
	return nil
}

func (g *SystemGitCloner) injectToken(rawURL string) string {
	if g.githubToken == "" {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	if parsed.Scheme == "https" && strings.Contains(parsed.Host, "github.com") {
		parsed.User = url.UserPassword("x-access-token", g.githubToken)
		return parsed.String()
	}

	return rawURL
}
