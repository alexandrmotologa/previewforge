package proxy

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// StandaloneProxy is a high-throughput, embedded Go reverse proxy with host-based dynamic routing,
// Basic Auth protection, anti-indexation headers, and wake-on-request hibernation support.
type StandaloneProxy struct {
	mu              sync.RWMutex
	routes          map[string]RouteConfig
	wakeHandler     func(deploymentID string) error
	activityHandler func(deploymentID string)
}

// NewStandaloneProxy constructs a StandaloneProxy.
func NewStandaloneProxy() *StandaloneProxy {
	return &StandaloneProxy{
		routes: make(map[string]RouteConfig),
	}
}

// SetWakeHandler attaches the callback triggered when traffic hits a sleeping preview.
func (p *StandaloneProxy) SetWakeHandler(fn func(deploymentID string) error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.wakeHandler = fn
}

// SetActivityHandler attaches the callback called to record HTTP traffic for idle tracking.
func (p *StandaloneProxy) SetActivityHandler(fn func(deploymentID string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activityHandler = fn
}

// RegisterRoute maps an incoming subdomain/hostname to an internal host port.
func (p *StandaloneProxy) RegisterRoute(subdomain string, targetPort int) error {
	return p.RegisterRouteWithConfig(subdomain, RouteConfig{
		TargetPort: targetPort,
	})
}

// RegisterRouteWithConfig registers a route with authentication and deployment metadata.
func (p *StandaloneProxy) RegisterRouteWithConfig(subdomain string, cfg RouteConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cleanHost := strings.ToLower(strings.TrimSpace(subdomain))
	if cleanHost == "" {
		return fmt.Errorf("subdomain cannot be empty")
	}

	p.routes[cleanHost] = cfg
	return nil
}

// SetRouteSleeping updates the hibernation state of a route.
func (p *StandaloneProxy) SetRouteSleeping(subdomain string, sleeping bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cleanHost := strings.ToLower(strings.TrimSpace(subdomain))
	if cfg, ok := p.routes[cleanHost]; ok {
		cfg.IsSleeping = sleeping
		p.routes[cleanHost] = cfg
	}
}

// RemoveRoute clears the routing entry for the given subdomain.
func (p *StandaloneProxy) RemoveRoute(subdomain string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cleanHost := strings.ToLower(strings.TrimSpace(subdomain))
	delete(p.routes, cleanHost)
	return nil
}

// GetTarget returns the target port for a given hostname.
func (p *StandaloneProxy) GetTarget(host string) (int, bool) {
	cfg, ok := p.GetRouteConfig(host)
	if !ok {
		return 0, false
	}
	return cfg.TargetPort, true
}

// GetRouteConfig returns the full RouteConfig for a given hostname.
func (p *StandaloneProxy) GetRouteConfig(host string) (RouteConfig, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cleanHost := strings.ToLower(strings.TrimSpace(host))
	if idx := strings.Index(cleanHost, ":"); idx != -1 {
		cleanHost = cleanHost[:idx]
	}

	cfg, exists := p.routes[cleanHost]
	return cfg, exists
}

// ServeHTTP handles incoming reverse proxy requests.
func (p *StandaloneProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route, found := p.GetRouteConfig(r.Host)
	if !found {
		p.renderNotFound(w, r)
		return
	}

	// Always inject anti-indexation robots headers on preview environments
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet")

	// 1. Basic Auth Enforcement
	if route.AuthUser != "" && route.AuthPassword != "" {
		user, pass, ok := r.BasicAuth()
		keyParam := r.URL.Query().Get("preview_key")

		isAuth := false
		if ok && subtle.ConstantTimeCompare([]byte(user), []byte(route.AuthUser)) == 1 &&
			subtle.ConstantTimeCompare([]byte(pass), []byte(route.AuthPassword)) == 1 {
			isAuth = true
		} else if keyParam != "" && subtle.ConstantTimeCompare([]byte(keyParam), []byte(route.AuthPassword)) == 1 {
			isAuth = true
		}

		if !isAuth {
			w.Header().Set("WWW-Authenticate", `Basic realm="PreviewForge Protected Preview"`)
			http.Error(w, "Unauthorized preview access", http.StatusUnauthorized)
			return
		}
	}

	// 2. Hibernation / Wake-on-Request
	if route.IsSleeping {
		p.mu.RLock()
		wakeFn := p.wakeHandler
		p.mu.RUnlock()

		if wakeFn != nil && route.DeploymentID != "" {
			go func() {
				_ = wakeFn(route.DeploymentID)
			}()
		}
		p.renderWakingUp(w, r, route.DeploymentID)
		return
	}

	// 3. Record activity for idle tracking
	p.mu.RLock()
	activityFn := p.activityHandler
	p.mu.RUnlock()

	if activityFn != nil && route.DeploymentID != "" {
		activityFn(route.DeploymentID)
	}

	targetURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", route.TargetPort))
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Host", r.Host)
		if r.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
	}

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		p.renderError(rw, req, route.TargetPort, err)
	}

	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
	}

	proxy.ServeHTTP(w, r)
}

func (p *StandaloneProxy) renderWakingUp(w http.ResponseWriter, r *http.Request, deploymentID string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<meta http-equiv="refresh" content="2">
	<title>Waking Up Preview Stack</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0b0f19; color: #f8fafc; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
		.card { background: #131b2e; border: 1px solid #1e293b; border-radius: 12px; padding: 2.5rem; max-width: 460px; text-align: center; box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.5); }
		h1 { font-size: 1.25rem; font-weight: 600; margin: 1rem 0 0.5rem 0; color: #f1f5f9; }
		p { color: #94a3b8; font-size: 0.875rem; line-height: 1.5; margin: 0 0 1.5rem 0; }
		.spinner { display: inline-block; width: 40px; height: 40px; border: 3px solid rgba(34, 197, 94, 0.2); border-radius: 50%%; border-top-color: #22c55e; animation: spin 0.8s linear infinite; }
		@keyframes spin { to { transform: rotate(360deg); } }
		.badge { display: inline-block; background: rgba(34, 197, 94, 0.1); color: #4ade80; border: 1px solid rgba(34, 197, 94, 0.2); padding: 0.25rem 0.75rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 500; }
	</style>
</head>
<body>
	<div class="card">
		<div class="spinner"></div>
		<h1>Waking Up Preview Stack</h1>
		<p>This environment was hibernated to save VPS memory. Resuming Docker containers now...</p>
		<div class="badge">Auto-reloading in 2 seconds</div>
	</div>
</body>
</html>`)
}

func (p *StandaloneProxy) renderNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<title>Preview Environment Not Found</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
		.card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 2.5rem; max-width: 480px; text-align: center; }
		h1 { font-size: 1.25rem; font-weight: 600; margin: 0 0 1rem 0; color: #f1f5f9; }
		p { color: #94a3b8; font-size: 0.875rem; line-height: 1.5; margin: 0 0 1.5rem 0; }
		code { background: #0f172a; padding: 0.2rem 0.4rem; border-radius: 4px; font-family: monospace; color: #38bdf8; font-size: 0.8rem; }
		.badge { display: inline-block; background: #ef4444; color: white; padding: 0.25rem 0.75rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 500; margin-bottom: 1rem; }
	</style>
</head>
<body>
	<div class="card">
		<div class="badge">404 Not Found</div>
		<h1>Preview Environment Inactive</h1>
		<p>No active preview deployment was found for <code>%s</code>.</p>
		<p>The pull request may have been closed, or the container stack is still initializing. Check the pull request discussion on GitHub for status updates.</p>
	</div>
</body>
</html>`, r.Host)
}

func (p *StandaloneProxy) renderError(w http.ResponseWriter, r *http.Request, port int, err error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8">
	<title>Preview Starting Up</title>
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f172a; color: #f8fafc; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
		.card { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 2.5rem; max-width: 480px; text-align: center; }
		h1 { font-size: 1.25rem; font-weight: 600; margin: 0 0 1rem 0; color: #f1f5f9; }
		p { color: #94a3b8; font-size: 0.875rem; line-height: 1.5; margin: 0 0 1.5rem 0; }
		code { background: #0f172a; padding: 0.2rem 0.4rem; border-radius: 4px; font-family: monospace; color: #38bdf8; font-size: 0.8rem; }
		.badge { display: inline-block; background: #f59e0b; color: #0f172a; padding: 0.25rem 0.75rem; border-radius: 9999px; font-size: 0.75rem; font-weight: 600; margin-bottom: 1rem; }
	</style>
</head>
<body>
	<div class="card">
		<div class="badge">502 Initializing</div>
		<h1>Preview Environment Starting</h1>
		<p>The preview container on port <code>%d</code> is currently warming up.</p>
		<p>Please refresh this page in a few seconds once the web server completes initialization.</p>
	</div>
</body>
</html>`, port)
}
