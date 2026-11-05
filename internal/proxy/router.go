package proxy

import "net/http"

// ProxyRouter manages routing from ephemeral preview subdomains to allocated container ports.
type ProxyRouter interface {
	RegisterRoute(subdomain string, targetPort int) error
	RemoveRoute(subdomain string) error
	GetTarget(host string) (int, bool)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}
