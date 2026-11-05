package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// StandaloneProxy is a high-throughput, embedded Go reverse proxy with host-based dynamic routing.
type StandaloneProxy struct {
	mu     sync.RWMutex
	routes map[string]int // lowercase hostname -> target port
}

// NewStandaloneProxy constructs a StandaloneProxy.
func NewStandaloneProxy() *StandaloneProxy {
	return &StandaloneProxy{
		routes: make(map[string]int),
	}
}

// RegisterRoute maps an incoming subdomain/hostname to an internal host port.
func (p *StandaloneProxy) RegisterRoute(subdomain string, targetPort int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cleanHost := strings.ToLower(strings.TrimSpace(subdomain))
	if cleanHost == "" {
		return fmt.Errorf("subdomain cannot be empty")
	}

	p.routes[cleanHost] = targetPort
	return nil
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
	p.mu.RLock()
	defer p.mu.RUnlock()

	cleanHost := strings.ToLower(strings.TrimSpace(host))
	if idx := strings.Index(cleanHost, ":"); idx != -1 {
		cleanHost = cleanHost[:idx]
	}

	port, exists := p.routes[cleanHost]
	return port, exists
}

// ServeHTTP handles incoming reverse proxy requests.
func (p *StandaloneProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	targetPort, found := p.GetTarget(r.Host)
	if !found {
		p.renderNotFound(w, r)
		return
	}

	targetURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", targetPort))
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// Custom director to preserve headers and pass proper forwarding metadata
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
		p.renderError(rw, req, targetPort, err)
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
