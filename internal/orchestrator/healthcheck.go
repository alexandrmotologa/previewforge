package orchestrator

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// HealthChecker polls preview services until they respond to HTTP or TCP requests.
type HealthChecker struct {
	client *http.Client
}

// NewHealthChecker constructs a HealthChecker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		client: &http.Client{
			Timeout: 2 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects, 3xx means service is up and serving!
				return http.ErrUseLastResponse
			},
		},
	}
}

// WaitForReady repeatedly checks if the target port is accepting HTTP or TCP connections.
func (h *HealthChecker) WaitForReady(ctx context.Context, port int, healthPath string, timeout time.Duration, logFn func(string)) error {
	if healthPath == "" {
		healthPath = "/"
	}
	if !strings.HasPrefix(healthPath, "/") {
		healthPath = "/" + healthPath
	}

	targetURL := fmt.Sprintf("http://127.0.0.1:%d%s", port, healthPath)
	logFn(fmt.Sprintf("Awaiting service readiness on %s (timeout: %s)...", targetURL, timeout))

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var attempts int
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			attempts++

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
			if err == nil {
				resp, reqErr := h.client.Do(req)
				if reqErr == nil {
					_ = resp.Body.Close()
					// Any HTTP response (including 404 or 401) confirms the web container is running
					logFn(fmt.Sprintf("Service is ready (HTTP %d, attempt %d).", resp.StatusCode, attempts))
					return nil
				}
			}

			// Fallback: test TCP socket connectivity directly
			conn, tcpErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
			if tcpErr == nil {
				_ = conn.Close()
				logFn(fmt.Sprintf("Service socket is open (attempt %d).", attempts))
				return nil
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("service readiness timed out after %s (%d attempts)", timeout, attempts)
			}
		}
	}
}
