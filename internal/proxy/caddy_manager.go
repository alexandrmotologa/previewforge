package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CaddyManager coordinates route updates with Caddy's dynamic JSON Admin API.
type CaddyManager struct {
	adminURL   string
	fallback   *StandaloneProxy
	httpClient *http.Client
	mu         sync.Mutex
	routes     map[string]int
}

// NewCaddyManager constructs a CaddyManager.
func NewCaddyManager(adminURL string) *CaddyManager {
	if adminURL == "" {
		adminURL = "http://127.0.0.1:2019"
	}
	return &CaddyManager{
		adminURL: strings.TrimRight(adminURL, "/"),
		fallback: NewStandaloneProxy(),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		routes: make(map[string]int),
	}
}

// SetWakeHandler attaches wake callback.
func (c *CaddyManager) SetWakeHandler(fn func(deploymentID string) error) {
	c.fallback.SetWakeHandler(fn)
}

// SetActivityHandler attaches activity callback.
func (c *CaddyManager) SetActivityHandler(fn func(deploymentID string)) {
	c.fallback.SetActivityHandler(fn)
}

// RegisterRoute registers the subdomain in the local routing table and pushes to Caddy.
func (c *CaddyManager) RegisterRoute(subdomain string, targetPort int) error {
	return c.RegisterRouteWithConfig(subdomain, RouteConfig{TargetPort: targetPort})
}

// RegisterRouteWithConfig registers the route with extended options.
func (c *CaddyManager) RegisterRouteWithConfig(subdomain string, cfg RouteConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleanHost := strings.ToLower(strings.TrimSpace(subdomain))
	c.routes[cleanHost] = cfg.TargetPort
	_ = c.fallback.RegisterRouteWithConfig(cleanHost, cfg)

	return c.syncToCaddy()
}

// SetRouteSleeping updates hibernation status.
func (c *CaddyManager) SetRouteSleeping(subdomain string, sleeping bool) {
	c.fallback.SetRouteSleeping(subdomain, sleeping)
}

// RemoveRoute clears the subdomain from the routing table and synchronizes with Caddy.
func (c *CaddyManager) RemoveRoute(subdomain string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleanHost := strings.ToLower(strings.TrimSpace(subdomain))
	delete(c.routes, cleanHost)
	_ = c.fallback.RemoveRoute(cleanHost)

	return c.syncToCaddy()
}

// GetTarget checks the fallback proxy table.
func (c *CaddyManager) GetTarget(host string) (int, bool) {
	return c.fallback.GetTarget(host)
}

// GetRouteConfig returns route config.
func (c *CaddyManager) GetRouteConfig(host string) (RouteConfig, bool) {
	return c.fallback.GetRouteConfig(host)
}

// ServeHTTP delegates to the embedded reverse proxy handler.
func (c *CaddyManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.fallback.ServeHTTP(w, r)
}

// CaddyRoute represents a single Caddy HTTP server route definition.
type CaddyRoute struct {
	Match  []map[string][]string    `json:"match"`
	Handle []map[string]interface{} `json:"handle"`
}

func (c *CaddyManager) syncToCaddy() error {
	var caddyRoutes []CaddyRoute

	for host, port := range c.routes {
		route := CaddyRoute{
			Match: []map[string][]string{
				{"host": {host}},
			},
			Handle: []map[string]interface{}{
				{
					"handler": "reverse_proxy",
					"upstreams": []map[string]string{
						{"dial": fmt.Sprintf("127.0.0.1:%d", port)},
					},
				},
			},
		}
		caddyRoutes = append(caddyRoutes, route)
	}

	payload, err := json.Marshal(caddyRoutes)
	if err != nil {
		return fmt.Errorf("failed to encode caddy routes: %w", err)
	}

	targetURL := fmt.Sprintf("%s/config/apps/http/servers/previewforge/routes", c.adminURL)
	req, err := http.NewRequest(http.MethodPatch, targetURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[caddy] Warning: Caddy Admin API at %s unreachable (%v). Traffic routed via internal proxy.", c.adminURL, err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[caddy] Warning: Caddy Admin API returned status %d for route sync", resp.StatusCode)
	}

	return nil
}
