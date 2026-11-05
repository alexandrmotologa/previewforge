package orchestrator

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// ContainerStats holds CPU and memory metrics for a running preview stack.
type ContainerStats struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemoryMB   float64 `json:"memory_mb"`
}

// DockerRunner manages Docker Compose operations for ephemeral stacks.
type DockerRunner interface {
	Up(ctx context.Context, projectName, workspaceDir, baseFile, overrideFile string, logFn func(string)) error
	Down(ctx context.Context, projectName, workspaceDir, baseFile, overrideFile string, logFn func(string)) error
	Stats(ctx context.Context, projectName string) (*ContainerStats, error)
}

// SystemDockerRunner executes real docker compose commands via OS process.
type SystemDockerRunner struct {
	mu sync.Mutex
}

// NewSystemDockerRunner constructs a SystemDockerRunner.
func NewSystemDockerRunner() *SystemDockerRunner {
	return &SystemDockerRunner{}
}

// Up builds and launches the compose preview environment.
func (d *SystemDockerRunner) Up(ctx context.Context, projectName, workspaceDir, baseFile, overrideFile string, logFn func(string)) error {
	args := []string{
		"compose",
		"-p", projectName,
		"-f", baseFile,
		"-f", overrideFile,
		"up", "-d", "--build",
	}

	logFn(fmt.Sprintf("Executing: docker %s", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workspaceDir

	return streamCommandOutput(cmd, logFn)
}

// Down stops and removes containers, networks, volumes, and locally built images.
func (d *SystemDockerRunner) Down(ctx context.Context, projectName, workspaceDir, baseFile, overrideFile string, logFn func(string)) error {
	args := []string{
		"compose",
		"-p", projectName,
		"-f", baseFile,
	}
	if overrideFile != "" {
		args = append(args, "-f", overrideFile)
	}
	args = append(args, "down", "-v", "--rmi", "local")

	logFn(fmt.Sprintf("Tearing down stack: docker %s", strings.Join(args, " ")))

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workspaceDir

	return streamCommandOutput(cmd, logFn)
}

// Stats returns aggregated resource usage for containers belonging to the project.
func (d *SystemDockerRunner) Stats(ctx context.Context, projectName string) (*ContainerStats, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--filter", fmt.Sprintf("label=com.docker.compose.project=%s", projectName), "--format", "{{.ID}}")
	out, err := cmd.Output()
	if err != nil {
		return &ContainerStats{}, nil
	}

	containerIDs := strings.Fields(strings.TrimSpace(string(out)))
	if len(containerIDs) == 0 {
		return &ContainerStats{}, nil
	}

	// Query docker stats for container IDs
	statsArgs := append([]string{"stats", "--no-stream", "--format", "{{.CPUPerc}}\t{{.MemUsage}}"}, containerIDs...)
	statsCmd := exec.CommandContext(ctx, "docker", statsArgs...)
	statsOut, err := statsCmd.Output()
	if err != nil {
		return &ContainerStats{}, nil
	}

	var totalCPU float64
	var totalMemMB float64

	scanner := bufio.NewScanner(strings.NewReader(string(statsOut)))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) >= 2 {
			cpuStr := strings.TrimSuffix(strings.TrimSpace(parts[0]), "%")
			var cpuVal float64
			fmt.Sscanf(cpuStr, "%f", &cpuVal)
			totalCPU += cpuVal

			// Memory usage format: "42.5MiB / 1.95GiB"
			memParts := strings.Split(parts[1], "/")
			if len(memParts) > 0 {
				memStr := strings.TrimSpace(memParts[0])
				if strings.HasSuffix(memStr, "GiB") {
					var g float64
					fmt.Sscanf(strings.TrimSuffix(memStr, "GiB"), "%f", &g)
					totalMemMB += g * 1024
				} else if strings.HasSuffix(memStr, "MiB") {
					var m float64
					fmt.Sscanf(strings.TrimSuffix(memStr, "MiB"), "%f", &m)
					totalMemMB += m
				} else if strings.HasSuffix(memStr, "KiB") {
					var k float64
					fmt.Sscanf(strings.TrimSuffix(memStr, "KiB"), "%f", &k)
					totalMemMB += k / 1024
				}
			}
		}
	}

	return &ContainerStats{
		CPUPercent: totalCPU,
		MemoryMB:   totalMemMB,
	}, nil
}

func streamCommandOutput(cmd *exec.Cmd, logFn func(string)) error {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	scanOutput := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) != "" {
				logFn(line)
			}
		}
	}

	go scanOutput(stdoutPipe)
	go scanOutput(stderrPipe)

	wg.Wait()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command exited with error: %w", err)
	}

	return nil
}
