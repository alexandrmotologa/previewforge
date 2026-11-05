package proxy

import "net/http"

// RouteConfig holds metadata and security controls for a preview route.
type RouteConfig struct {
	TargetPort   int
	DeploymentID string
	AuthUser     string
	AuthPassword string
	IsSleeping   bool
}

// ProxyRouter manages routing from ephemeral preview subdomains to allocated container ports.
type ProxyRouter interface {
	RegisterRoute(subdomain string, targetPort int) error
	RegisterRouteWithConfig(subdomain string, cfg RouteConfig) error
	SetRouteSleeping(subdomain string, sleeping bool)
	RemoveRoute(subdomain string) error
	GetTarget(host string) (int, bool)
	GetRouteConfig(host string) (RouteConfig, bool)
	SetWakeHandler(fn func(deploymentID string) error)
	SetActivityHandler(fn func(deploymentID string))
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}
