package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/alexandrmotologa/previewforge/internal/model"
)

// ComposeInspection holds resolved compose settings for the preview stack.
type ComposeInspection struct {
	BaseComposeFile string
	OverrideFile    string
	ServiceName     string
	ContainerPort   int
	HealthPath      string
	Environment     map[string]string
	PostDeploy      *model.PostDeployHook
	AuthUser        string
	AuthPassword    string
}

// ComposeBuilder inspects repositories and synthesizes Docker Compose overrides.
type ComposeBuilder struct{}

// NewComposeBuilder constructs a ComposeBuilder.
func NewComposeBuilder() *ComposeBuilder {
	return &ComposeBuilder{}
}

// InspectAndPrepare inspects the repository workspace, parses compose definitions, and writes an override file.
func (b *ComposeBuilder) InspectAndPrepare(workspaceDir, deploymentID string, prNumber, allocatedHostPort int, previewURL string) (*ComposeInspection, error) {
	// 1. Detect base compose file
	candidates := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	var baseFile string
	for _, c := range candidates {
		fullPath := filepath.Join(workspaceDir, c)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			baseFile = c
			break
		}
	}

	// 2. Read optional in-repo .previewforge.yml
	var repoConfig *model.RepoConfig
	configPath := filepath.Join(workspaceDir, ".previewforge.yml")
	if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
		repoConfig = parseSimpleRepoConfig(configPath)
		if repoConfig != nil && repoConfig.ComposeFile != "" {
			baseFile = repoConfig.ComposeFile
		}
	}

	if baseFile == "" {
		return nil, fmt.Errorf("no docker-compose.yml or compose.yaml found in workspace %s", workspaceDir)
	}

	baseContentBytes, err := os.ReadFile(filepath.Join(workspaceDir, baseFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file %s: %w", baseFile, err)
	}
	baseContent := string(baseContentBytes)

	// 3. Resolve primary service and container port
	serviceName := "web"
	containerPort := 3000
	healthPath := "/"
	envVars := make(map[string]string)
	var postDeploy *model.PostDeployHook
	var authUser, authPass string

	if repoConfig != nil {
		if repoConfig.Service != "" {
			serviceName = repoConfig.Service
		}
		if repoConfig.Port > 0 {
			containerPort = repoConfig.Port
		}
		if repoConfig.HealthPath != "" {
			healthPath = repoConfig.HealthPath
		}
		for k, v := range repoConfig.Env {
			envVars[k] = v
		}
		postDeploy = repoConfig.PostDeploy
		if repoConfig.BasicAuth != nil {
			authUser = repoConfig.BasicAuth.User
			authPass = repoConfig.BasicAuth.Password
		}
	} else {
		// Infer from compose file
		detectedService, detectedPort := detectPrimaryService(baseContent)
		if detectedService != "" {
			serviceName = detectedService
		}
		if detectedPort > 0 {
			containerPort = detectedPort
		}
	}

	// 4. Generate docker-compose.previewforge.override.yml
	overrideFileName := "docker-compose.previewforge.override.yml"
	overrideFullPath := filepath.Join(workspaceDir, overrideFileName)

	var sb strings.Builder
	sb.WriteString("services:\n")
	sb.WriteString(fmt.Sprintf("  %s:\n", serviceName))
	sb.WriteString("    ports:\n")
	sb.WriteString(fmt.Sprintf("      - \"%d:%d\"\n", allocatedHostPort, containerPort))
	sb.WriteString("    labels:\n")
	sb.WriteString("      previewforge.managed: \"true\"\n")
	sb.WriteString(fmt.Sprintf("      previewforge.pr: \"%d\"\n", prNumber))
	sb.WriteString(fmt.Sprintf("      previewforge.id: \"%s\"\n", deploymentID))
	sb.WriteString("    environment:\n")
	sb.WriteString("      - PREVIEWFORGE=true\n")
	sb.WriteString(fmt.Sprintf("      - PREVIEW_URL=%s\n", previewURL))
	sb.WriteString(fmt.Sprintf("      - PR_NUMBER=%d\n", prNumber))
	for k, v := range envVars {
		sb.WriteString(fmt.Sprintf("      - %s=%s\n", k, v))
	}

	if err := os.WriteFile(overrideFullPath, []byte(sb.String()), 0644); err != nil {
		return nil, fmt.Errorf("failed to write compose override file: %w", err)
	}

	return &ComposeInspection{
		BaseComposeFile: baseFile,
		OverrideFile:    overrideFileName,
		ServiceName:     serviceName,
		ContainerPort:   containerPort,
		HealthPath:      healthPath,
		Environment:     envVars,
		PostDeploy:      postDeploy,
		AuthUser:        authUser,
		AuthPassword:    authPass,
	}, nil
}

// detectPrimaryService searches compose content for common frontend/web services and exposed ports.
func detectPrimaryService(content string) (string, int) {
	lines := strings.Split(content, "\n")
	var inServices bool
	var currentService string
	var services []string
	servicePorts := make(map[string]int)

	serviceHeaderRegex := regexp.MustCompile(`^[ ]{2}([a-zA-Z0-9_-]+):[ ]*$`)
	portRegex := regexp.MustCompile(`["']?(\d+):(\d+)["']?`)
	singlePortRegex := regexp.MustCompile(`["']?(\d+)["']?`)

	var inPorts bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "services:" {
			inServices = true
			continue
		}

		if inServices {
			if matches := serviceHeaderRegex.FindStringSubmatch(line); len(matches) == 2 {
				currentService = matches[1]
				services = append(services, currentService)
				inPorts = false
				continue
			}

			if currentService != "" {
				if strings.HasPrefix(trimmed, "ports:") {
					inPorts = true
					continue
				}

				if inPorts {
					if strings.HasPrefix(trimmed, "-") {
						if pMatch := portRegex.FindStringSubmatch(trimmed); len(pMatch) == 3 {
							if targetP, err := strconv.Atoi(pMatch[2]); err == nil && targetP > 0 {
								servicePorts[currentService] = targetP
							}
						} else if spMatch := singlePortRegex.FindStringSubmatch(trimmed); len(spMatch) == 2 {
							if targetP, err := strconv.Atoi(spMatch[1]); err == nil && targetP > 0 {
								servicePorts[currentService] = targetP
							}
						}
					} else if !strings.HasPrefix(line, "    ") {
						inPorts = false
					}
				}
			}
		}
	}

	// Priority service names
	priorityNames := []string{"web", "frontend", "app", "ui", "client", "server", "api"}
	for _, pName := range priorityNames {
		for _, s := range services {
			if strings.EqualFold(s, pName) {
				port := servicePorts[s]
				if port == 0 {
					port = 3000
				}
				return s, port
			}
		}
	}

	// If any service has an exposed port, pick it
	for _, s := range services {
		if p, ok := servicePorts[s]; ok && p > 0 {
			return s, p
		}
	}

	// Fallback to first service in list
	if len(services) > 0 {
		return services[0], 3000
	}

	return "web", 3000
}

func parseSimpleRepoConfig(path string) *model.RepoConfig {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	cfg := &model.RepoConfig{
		Env: make(map[string]string),
	}

	lines := strings.Split(string(content), "\n")
	var inEnv, inPostDeploy, inBasicAuth bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "env:") {
			inEnv = true
			inPostDeploy = false
			inBasicAuth = false
			continue
		}
		if strings.HasPrefix(trimmed, "post_deploy:") {
			inPostDeploy = true
			inEnv = false
			inBasicAuth = false
			cfg.PostDeploy = &model.PostDeployHook{}
			continue
		}
		if strings.HasPrefix(trimmed, "basic_auth:") {
			inBasicAuth = true
			inEnv = false
			inPostDeploy = false
			cfg.BasicAuth = &struct {
				User     string `yaml:"user" json:"user"`
				Password string `yaml:"password" json:"password"`
			}{}
			continue
		}

		if inEnv {
			if strings.Contains(trimmed, ":") {
				parts := strings.SplitN(trimmed, ":", 2)
				k := strings.TrimSpace(parts[0])
				v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				cfg.Env[k] = v
			} else if !strings.HasPrefix(line, " ") {
				inEnv = false
			}
		}

		if inPostDeploy && cfg.PostDeploy != nil {
			if strings.HasPrefix(trimmed, "command:") {
				cfg.PostDeploy.Command = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "command:")), `"'`)
			} else if strings.HasPrefix(trimmed, "service:") {
				cfg.PostDeploy.Service = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "service:")), `"'`)
			} else if strings.HasPrefix(trimmed, "timeout_sec:") {
				if t, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trimmed, "timeout_sec:"))); err == nil {
					cfg.PostDeploy.TimeoutSec = t
				}
			} else if !strings.HasPrefix(line, " ") {
				inPostDeploy = false
			}
		}

		if inBasicAuth && cfg.BasicAuth != nil {
			if strings.HasPrefix(trimmed, "user:") {
				cfg.BasicAuth.User = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "user:")), `"'`)
			} else if strings.HasPrefix(trimmed, "password:") {
				cfg.BasicAuth.Password = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "password:")), `"'`)
			} else if !strings.HasPrefix(line, " ") {
				inBasicAuth = false
			}
		}

		if strings.HasPrefix(trimmed, "compose_file:") {
			cfg.ComposeFile = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "compose_file:")), `"'`)
		} else if strings.HasPrefix(trimmed, "service:") && !inPostDeploy {
			cfg.Service = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "service:")), `"'`)
		} else if strings.HasPrefix(trimmed, "port:") {
			if p, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trimmed, "port:"))); err == nil {
				cfg.Port = p
			}
		} else if strings.HasPrefix(trimmed, "health_path:") {
			cfg.HealthPath = strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "health_path:")), `"'`)
		}
	}

	return cfg
}
