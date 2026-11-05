package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/config"
	"github.com/alexandrmotologa/previewforge/internal/github"
	"github.com/alexandrmotologa/previewforge/internal/model"
	"github.com/alexandrmotologa/previewforge/internal/orchestrator"
	"github.com/alexandrmotologa/previewforge/internal/proxy"
	"github.com/alexandrmotologa/previewforge/internal/server"
	"github.com/alexandrmotologa/previewforge/internal/storage"
	"github.com/alexandrmotologa/previewforge/internal/webhook"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "serve":
		runServe(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "destroy":
		runDestroy(os.Args[2:])
	case "prune":
		runPrune(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("PreviewForge v%s (Go 1.23 + React 19)\n", version)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`PreviewForge - Ephemeral PR Preview Supervisor

Usage:
  previewforge <command> [flags]

Commands:
  serve      Run the preview supervisor daemon, web studio, and reverse proxy
  list       Display active preview environments in a formatted table
  destroy    Tear down a preview environment and remove its Docker containers
  prune      Prune orphaned workspaces and release inactive container stacks
  version    Show PreviewForge version`)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)

	defaultCfg := config.DefaultConfig()

	domainFlag := fs.String("domain", defaultCfg.BaseDomain, "Base wildcard subdomain (e.g. preview.yourdomain.com or localhost)")
	portFlag := fs.Int("port", defaultCfg.PublicPort, "HTTP port for supervisor and reverse proxy")
	proxyModeFlag := fs.String("proxy-mode", defaultCfg.ProxyMode, "Reverse proxy engine: standalone or caddy")
	caddyURLFlag := fs.String("caddy-url", defaultCfg.CaddyAdminURL, "Caddy Admin API endpoint")
	dataDirFlag := fs.String("data-dir", "./data", "Directory for SQLite database and workspaces")
	tokenFlag := fs.String("github-token", defaultCfg.GitHubToken, "GitHub Personal Access Token for PR comments and status checks")
	secretFlag := fs.String("webhook-secret", defaultCfg.WebhookSecret, "GitHub Webhook HMAC SHA-256 secret")
	portStartFlag := fs.Int("port-start", defaultCfg.PortRangeStart, "Start of dynamic host port allocation range")
	portEndFlag := fs.Int("port-end", defaultCfg.PortRangeEnd, "End of dynamic host port allocation range")
	idleTimeoutFlag := fs.Int("idle-timeout", defaultCfg.IdleTimeoutMinutes, "Idle timeout in minutes before auto-reaping previews")
	requireLabelFlag := fs.String("require-label", defaultCfg.RequireLabel, "Only deploy PRs that contain this label")
	allowForksFlag := fs.Bool("allow-forks", defaultCfg.AllowUntrustedForks, "Allow automatic deployments from untrusted forks")

	_ = fs.Parse(args)

	cfg := model.SystemConfig{
		BaseDomain:          *domainFlag,
		PublicPort:          *portFlag,
		ProxyMode:           *proxyModeFlag,
		CaddyAdminURL:       *caddyURLFlag,
		PortRangeStart:      *portStartFlag,
		PortRangeEnd:        *portEndFlag,
		GitHubToken:         *tokenFlag,
		WebhookSecret:       *secretFlag,
		WorkspacesDir:       filepath.Join(*dataDirFlag, "workspaces"),
		IdleTimeoutMinutes:  *idleTimeoutFlag,
		RequireLabel:        *requireLabelFlag,
		AllowUntrustedForks: *allowForksFlag,
	}

	dbPath := filepath.Join(*dataDirFlag, "previewforge.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer store.Close()

	// Select Proxy Engine
	var proxyRouter proxy.ProxyRouter
	if cfg.ProxyMode == "caddy" {
		proxyRouter = proxy.NewCaddyManager(cfg.CaddyAdminURL)
	} else {
		proxyRouter = proxy.NewStandaloneProxy()
	}

	// Re-register active routes from DB into proxy upon daemon startup
	existingDeps, err := store.ListDeployments()
	if err == nil {
		for _, dep := range existingDeps {
			if dep.Status == model.StatusRunning && dep.Port > 0 && dep.Subdomain != "" {
				_ = proxyRouter.RegisterRoute(dep.Subdomain, dep.Port)
			}
		}
	}

	// Initialize Orchestrator dependencies
	portAllocator := orchestrator.NewPortAllocator(store, cfg.PortRangeStart, cfg.PortRangeEnd)
	gitCloner := orchestrator.NewSystemGitCloner(cfg.GitHubToken)
	composeBuilder := orchestrator.NewComposeBuilder()
	dockerRunner := orchestrator.NewSystemDockerRunner()
	healthChecker := orchestrator.NewHealthChecker()
	broadcaster := orchestrator.NewLogBroadcaster()

	var commenter *github.Commenter
	if cfg.GitHubToken != "" {
		ghClient := github.NewClient(cfg.GitHubToken)
		commenter = github.NewCommenter(ghClient)
	}

	orch := orchestrator.NewOrchestrator(
		store,
		cfg,
		portAllocator,
		gitCloner,
		composeBuilder,
		dockerRunner,
		healthChecker,
		proxyRouter,
		commenter,
		broadcaster,
	)

	// Start background reaper for stale environments
	reaper := orchestrator.NewReaper(store, orch, cfg.WorkspacesDir, cfg.IdleTimeoutMinutes)
	reaper.Start(10 * time.Minute)
	defer reaper.Stop()

	// Webhook handler
	webhookHandler := webhook.NewHandler(orch, cfg.WebhookSecret, cfg.RequireLabel, cfg.AllowUntrustedForks)

	// HTTP Server (Serving Webhook + REST API + Web Studio + Dynamic Proxy)
	srv := server.NewServer(cfg, store, orch, proxyRouter, webhookHandler)

	banner := fmt.Sprintf(`
  ____                 _                 _____                    
 |  _ \ _ __ _____   _(_) _____      __ |  ___|__  _ __ __ _  ___ 
 | |_) | '__/ _ \ \ / / |/ _ \ \ /\ / / | |_ / _ \| '__/ _`+"`"+` |/ _ \
 |  __/| | |  __/\ V /| |  __/\ V  V /  |  _| (_) | | | (_| |  __/
 |_|   |_|  \___| \_/ |_|\___| \_/\_/   |_|  \___/|_|  \__, |\___|
                                                       |___/      
 Ephemeral Pull Request Preview Environment Supervisor v%s

 Daemon listening on :%d
 Base Domain:        *.%s
 Proxy Engine:       %s
 Host Ports Pool:    %d - %d
 Database:           %s
 Workspaces:         %s
 Webhook URL:        http://<host>:%d/webhook
 Studio Dashboard:   http://localhost:%d
`, version, cfg.PublicPort, cfg.BaseDomain, cfg.ProxyMode, cfg.PortRangeStart, cfg.PortRangeEnd, dbPath, cfg.WorkspacesDir, cfg.PublicPort, cfg.PublicPort)

	fmt.Print(banner)

	// Graceful shutdown listener
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Server shutdown: %v", err)
		}
	}()

	<-stopChan
	log.Println("\n[previewforge] Shutting down supervisor daemon gracefully...")
}

func runList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	dataDirFlag := fs.String("data-dir", "./data", "Directory containing previewforge.db")
	_ = fs.Parse(args)

	dbPath := filepath.Join(*dataDirFlag, "previewforge.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", dbPath, err)
	}
	defer store.Close()

	deployments, err := store.ListDeployments()
	if err != nil {
		log.Fatalf("Failed to list deployments: %v", err)
	}

	if len(deployments) == 0 {
		fmt.Println("No preview deployments found.")
		return
	}

	fmt.Printf("%-18s %-6s %-18s %-16s %-6s %-10s %-28s\n",
		"DEPLOYMENT ID", "PR #", "REPOSITORY", "BRANCH", "PORT", "STATUS", "SUBDOMAIN")
	fmt.Println(string(make([]byte, 110)))

	for _, d := range deployments {
		repo := fmt.Sprintf("%s/%s", d.RepoOwner, d.RepoName)
		fmt.Printf("%-18s %-6d %-18s %-16s %-6d %-10s %-28s\n",
			truncate(d.ID, 18), d.PRNumber, truncate(repo, 18), truncate(d.Branch, 16), d.Port, d.Status, d.Subdomain)
	}
}

func runDestroy(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: previewforge destroy <deployment_id_or_pr_number> [--data-dir ./data]")
		os.Exit(1)
	}

	target := args[0]
	dataDir := "./data"
	for i, arg := range args {
		if arg == "--data-dir" && i+1 < len(args) {
			dataDir = args[i+1]
		}
	}

	dbPath := filepath.Join(dataDir, "previewforge.db")
	store, err := storage.NewStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()

	dep, err := store.GetDeployment(target)
	if err != nil || dep == nil {
		// Try finding by PR number
		deps, _ := store.ListDeployments()
		for _, d := range deps {
			if fmt.Sprintf("%d", d.PRNumber) == target {
				dep = &d
				break
			}
		}
	}

	if dep == nil {
		log.Fatalf("Deployment '%s' not found.", target)
	}

	fmt.Printf("Destroying preview deployment %s (PR #%d)...\n", dep.ID, dep.PRNumber)

	cfg := config.DefaultConfig()
	cfg.WorkspacesDir = filepath.Join(dataDir, "workspaces")
	allocator := orchestrator.NewPortAllocator(store, cfg.PortRangeStart, cfg.PortRangeEnd)
	dockerRunner := orchestrator.NewSystemDockerRunner()
	proxyRouter := proxy.NewStandaloneProxy()

	orch := orchestrator.NewOrchestrator(
		store, cfg, allocator, nil, nil, dockerRunner, nil, proxyRouter, nil, orchestrator.NewLogBroadcaster(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := orch.Teardown(ctx, dep.RepoOwner, dep.RepoName, dep.PRNumber); err != nil {
		log.Fatalf("Teardown error: %v", err)
	}

	fmt.Println("Preview environment destroyed successfully.")
}

func runPrune(args []string) {
	dataDir := "./data"
	for i, arg := range args {
		if arg == "--data-dir" && i+1 < len(args) {
			dataDir = args[i+1]
		}
	}

	workspacesDir := filepath.Join(dataDir, "workspaces")
	fmt.Printf("Pruning orphaned workspaces from %s...\n", workspacesDir)

	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		fmt.Println("No workspaces directory found to prune.")
		return
	}

	var count int
	for _, entry := range entries {
		if entry.IsDir() {
			p := filepath.Join(workspacesDir, entry.Name())
			_ = os.RemoveAll(p)
			count++
		}
	}

	fmt.Printf("Cleaned %d workspaces.\n", count)
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
