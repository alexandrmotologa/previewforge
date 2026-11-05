package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/model"
	_ "modernc.org/sqlite"
)

// Store handles persistent SQLite storage for deployments, ports, and logs.
type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewStore initializes a SQLite database connection and runs migrations.
func NewStore(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory %s: %w", dir, err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite single-writer stability

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return store, nil
}

// Close terminates the database connection.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS deployments (
			id TEXT PRIMARY KEY,
			pr_number INTEGER NOT NULL,
			pr_title TEXT NOT NULL,
			repo_owner TEXT NOT NULL,
			repo_name TEXT NOT NULL,
			branch TEXT NOT NULL,
			commit_sha TEXT NOT NULL,
			author TEXT NOT NULL,
			status TEXT NOT NULL,
			port INTEGER NOT NULL,
			subdomain TEXT NOT NULL,
			preview_url TEXT NOT NULL,
			service_name TEXT NOT NULL,
			build_duration_sec INTEGER NOT NULL DEFAULT 0,
			health_status TEXT NOT NULL DEFAULT 'unknown',
			error_message TEXT NOT NULL DEFAULT '',
			last_activity_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			auto_sleep_minutes INTEGER NOT NULL DEFAULT 60,
			cpu_percent REAL NOT NULL DEFAULT 0.0,
			memory_mb REAL NOT NULL DEFAULT 0.0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_deployments_repo_pr ON deployments (repo_owner, repo_name, pr_number);`,
		`CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments (status);`,

		`CREATE TABLE IF NOT EXISTS port_allocations (
			port INTEGER PRIMARY KEY,
			deployment_id TEXT NOT NULL,
			allocated_at DATETIME NOT NULL,
			in_use INTEGER NOT NULL DEFAULT 1
		);`,

		`CREATE TABLE IF NOT EXISTS deployment_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			deployment_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			line TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_logs_deployment_id ON deployment_logs (deployment_id);`,

		`CREATE TABLE IF NOT EXISTS system_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}

	// Schema updates for existing tables
	_ = s.execOptional(`ALTER TABLE deployments ADD COLUMN last_activity_at DATETIME DEFAULT CURRENT_TIMESTAMP`)
	_ = s.execOptional(`ALTER TABLE deployments ADD COLUMN auto_sleep_minutes INTEGER DEFAULT 60`)
	_ = s.execOptional(`ALTER TABLE deployments ADD COLUMN cpu_percent REAL DEFAULT 0.0`)
	_ = s.execOptional(`ALTER TABLE deployments ADD COLUMN memory_mb REAL DEFAULT 0.0`)

	return nil
}

func (s *Store) execOptional(query string) error {
	_, err := s.db.Exec(query)
	return err
}

// SaveDeployment persists or updates a deployment.
func (s *Store) SaveDeployment(d *model.Deployment) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.LastActivityAt.IsZero() {
		d.LastActivityAt = now
	}
	d.UpdatedAt = now

	query := `INSERT INTO deployments (
		id, pr_number, pr_title, repo_owner, repo_name, branch, commit_sha, author,
		status, port, subdomain, preview_url, service_name, build_duration_sec,
		health_status, error_message, last_activity_at, auto_sleep_minutes,
		cpu_percent, memory_mb, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		pr_number = excluded.pr_number,
		pr_title = excluded.pr_title,
		repo_owner = excluded.repo_owner,
		repo_name = excluded.repo_name,
		branch = excluded.branch,
		commit_sha = excluded.commit_sha,
		author = excluded.author,
		status = excluded.status,
		port = excluded.port,
		subdomain = excluded.subdomain,
		preview_url = excluded.preview_url,
		service_name = excluded.service_name,
		build_duration_sec = excluded.build_duration_sec,
		health_status = excluded.health_status,
		error_message = excluded.error_message,
		last_activity_at = excluded.last_activity_at,
		auto_sleep_minutes = excluded.auto_sleep_minutes,
		cpu_percent = excluded.cpu_percent,
		memory_mb = excluded.memory_mb,
		updated_at = excluded.updated_at;`

	_, err := s.db.Exec(query,
		d.ID, d.PRNumber, d.PRTitle, d.RepoOwner, d.RepoName, d.Branch, d.CommitSHA, d.Author,
		string(d.Status), d.Port, d.Subdomain, d.PreviewURL, d.ServiceName, d.BuildDurationSec,
		d.HealthStatus, d.ErrorMessage, d.LastActivityAt, d.AutoSleepMinutes,
		d.CPUPercent, d.MemoryMB, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

// GetDeployment retrieves a deployment by its identifier.
func (s *Store) GetDeployment(id string) (*model.Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, pr_number, pr_title, repo_owner, repo_name, branch, commit_sha, author,
		status, port, subdomain, preview_url, service_name, build_duration_sec,
		health_status, error_message, last_activity_at, auto_sleep_minutes,
		cpu_percent, memory_mb, created_at, updated_at
		FROM deployments WHERE id = ?`

	row := s.db.QueryRow(query, id)
	var d model.Deployment
	var statusStr string

	err := row.Scan(
		&d.ID, &d.PRNumber, &d.PRTitle, &d.RepoOwner, &d.RepoName, &d.Branch, &d.CommitSHA, &d.Author,
		&statusStr, &d.Port, &d.Subdomain, &d.PreviewURL, &d.ServiceName, &d.BuildDurationSec,
		&d.HealthStatus, &d.ErrorMessage, &d.LastActivityAt, &d.AutoSleepMinutes,
		&d.CPUPercent, &d.MemoryMB, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Status = model.DeploymentStatus(statusStr)
	return &d, nil
}

// GetDeploymentByPR retrieves a deployment for a specific repository and pull request number.
func (s *Store) GetDeploymentByPR(repoOwner, repoName string, prNumber int) (*model.Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, pr_number, pr_title, repo_owner, repo_name, branch, commit_sha, author,
		status, port, subdomain, preview_url, service_name, build_duration_sec,
		health_status, error_message, last_activity_at, auto_sleep_minutes,
		cpu_percent, memory_mb, created_at, updated_at
		FROM deployments WHERE repo_owner = ? AND repo_name = ? AND pr_number = ?
		ORDER BY created_at DESC LIMIT 1`

	row := s.db.QueryRow(query, repoOwner, repoName, prNumber)
	var d model.Deployment
	var statusStr string

	err := row.Scan(
		&d.ID, &d.PRNumber, &d.PRTitle, &d.RepoOwner, &d.RepoName, &d.Branch, &d.CommitSHA, &d.Author,
		&statusStr, &d.Port, &d.Subdomain, &d.PreviewURL, &d.ServiceName, &d.BuildDurationSec,
		&d.HealthStatus, &d.ErrorMessage, &d.LastActivityAt, &d.AutoSleepMinutes,
		&d.CPUPercent, &d.MemoryMB, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Status = model.DeploymentStatus(statusStr)
	return &d, nil
}

// ListDeployments returns all deployments ordered by creation timestamp.
func (s *Store) ListDeployments() ([]model.Deployment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT id, pr_number, pr_title, repo_owner, repo_name, branch, commit_sha, author,
		status, port, subdomain, preview_url, service_name, build_duration_sec,
		health_status, error_message, last_activity_at, auto_sleep_minutes,
		cpu_percent, memory_mb, created_at, updated_at
		FROM deployments ORDER BY created_at DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Deployment
	for rows.Next() {
		var d model.Deployment
		var statusStr string
		err := rows.Scan(
			&d.ID, &d.PRNumber, &d.PRTitle, &d.RepoOwner, &d.RepoName, &d.Branch, &d.CommitSHA, &d.Author,
			&statusStr, &d.Port, &d.Subdomain, &d.PreviewURL, &d.ServiceName, &d.BuildDurationSec,
			&d.HealthStatus, &d.ErrorMessage, &d.LastActivityAt, &d.AutoSleepMinutes,
			&d.CPUPercent, &d.MemoryMB, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		d.Status = model.DeploymentStatus(statusStr)
		list = append(list, d)
	}
	return list, nil
}

// TouchActivity updates the last_activity_at timestamp for a deployment.
func (s *Store) TouchActivity(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE deployments SET last_activity_at = ? WHERE id = ?`, time.Now().UTC(), id)
	return err
}

// UpdateStats updates CPU and Memory metrics for a deployment.
func (s *Store) UpdateStats(id string, cpuPercent, memoryMB float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE deployments SET cpu_percent = ?, memory_mb = ? WHERE id = ?`, cpuPercent, memoryMB, id)
	return err
}

// DeleteDeployment removes a deployment record.
func (s *Store) DeleteDeployment(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM deployment_logs WHERE deployment_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM port_allocations WHERE deployment_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM deployments WHERE id = ?`, id); err != nil {
		return err
	}

	return tx.Commit()
}

// AllocatePort finds the first available port in the range and marks it reserved.
func (s *Store) AllocatePort(deploymentID string, startPort, endPort int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already allocated to this deployment
	var existingPort int
	err := s.db.QueryRow(`SELECT port FROM port_allocations WHERE deployment_id = ? AND in_use = 1`, deploymentID).Scan(&existingPort)
	if err == nil && existingPort >= startPort && existingPort <= endPort {
		return existingPort, nil
	}

	// Query currently used ports in range
	rows, err := s.db.Query(`SELECT port FROM port_allocations WHERE in_use = 1 AND port >= ? AND port <= ?`, startPort, endPort)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	usedPorts := make(map[int]bool)
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err == nil {
			usedPorts[p] = true
		}
	}

	// Find first free port
	allocatedPort := 0
	for port := startPort; port <= endPort; port++ {
		if !usedPorts[port] {
			allocatedPort = port
			break
		}
	}

	if allocatedPort == 0 {
		return 0, fmt.Errorf("no available ports in range %d-%d", startPort, endPort)
	}

	now := time.Now().UTC()
	query := `INSERT INTO port_allocations (port, deployment_id, allocated_at, in_use)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(port) DO UPDATE SET
		deployment_id = excluded.deployment_id,
		allocated_at = excluded.allocated_at,
		in_use = 1`

	if _, err := s.db.Exec(query, allocatedPort, deploymentID, now); err != nil {
		return 0, fmt.Errorf("failed to save port reservation: %w", err)
	}

	return allocatedPort, nil
}

// ReleasePort frees an allocated port.
func (s *Store) ReleasePort(port int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE port_allocations SET in_use = 0 WHERE port = ?`, port)
	return err
}

// ReleasePortByDeployment frees any port allocated to the given deployment.
func (s *Store) ReleasePortByDeployment(deploymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`UPDATE port_allocations SET in_use = 0 WHERE deployment_id = ?`, deploymentID)
	return err
}

// ListPortAllocations returns all active port records.
func (s *Store) ListPortAllocations() ([]model.PortAllocation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT port, deployment_id, allocated_at, in_use FROM port_allocations WHERE in_use = 1 ORDER BY port ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.PortAllocation
	for rows.Next() {
		var p model.PortAllocation
		var inUseInt int
		if err := rows.Scan(&p.Port, &p.DeploymentID, &p.AllocatedAt, &inUseInt); err != nil {
			return nil, err
		}
		p.InUse = inUseInt == 1
		list = append(list, p)
	}
	return list, nil
}

// AppendLog writes a log line for a deployment.
func (s *Store) AppendLog(deploymentID string, line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`INSERT INTO deployment_logs (deployment_id, timestamp, line) VALUES (?, ?, ?)`,
		deploymentID, time.Now().UTC(), line,
	)
	return err
}

// GetLogs returns the stored log lines for a deployment up to limit.
func (s *Store) GetLogs(deploymentID string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 1000
	}

	query := `SELECT line FROM deployment_logs WHERE deployment_id = ? ORDER BY id ASC LIMIT ?`
	rows, err := s.db.Query(query, deploymentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		logs = append(logs, line)
	}
	return logs, nil
}

// CountDeployments returns count of active, sleeping and total deployments.
func (s *Store) CountDeployments() (active int, sleeping int, total int, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	err = s.db.QueryRow(`SELECT COUNT(*) FROM deployments`).Scan(&total)
	if err != nil {
		return 0, 0, 0, err
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM deployments WHERE status IN ('running', 'building', 'pending')`).Scan(&active)
	if err != nil {
		return 0, 0, 0, err
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM deployments WHERE status = 'sleeping'`).Scan(&sleeping)
	if err != nil {
		return 0, 0, 0, err
	}

	return active, sleeping, total, nil
}
