package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStandaloneProxyFeatures(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "test-app")
		fmt.Fprintf(w, "Hello from PR preview! Path: %s", r.URL.Path)
	}))
	defer backend.Close()

	parts := strings.Split(backend.URL, ":")
	backendPort, _ := strconv.Atoi(parts[len(parts)-1])

	proxy := NewStandaloneProxy()

	// 1. Test Anti-Robots header and forward
	t.Run("Anti-robots header and forward", func(t *testing.T) {
		_ = proxy.RegisterRoute("pr-1.preview.example.com", backendPort)

		req := httptest.NewRequest(http.MethodGet, "http://pr-1.preview.example.com/", nil)
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		robots := w.Header().Get("X-Robots-Tag")
		if !strings.Contains(robots, "noindex") {
			t.Fatalf("expected X-Robots-Tag header, got %s", robots)
		}
	})

	// 2. Test Basic Auth Protection
	t.Run("Basic Auth protection", func(t *testing.T) {
		_ = proxy.RegisterRouteWithConfig("pr-auth.preview.example.com", RouteConfig{
			TargetPort:   backendPort,
			AuthUser:     "tester",
			AuthPassword: "secret-password",
		})

		// Unauthenticated request -> 401
		reqUnauth := httptest.NewRequest(http.MethodGet, "http://pr-auth.preview.example.com/", nil)
		wUnauth := httptest.NewRecorder()
		proxy.ServeHTTP(wUnauth, reqUnauth)
		if wUnauth.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", wUnauth.Code)
		}

		// Authenticated request with Basic Auth -> 200 OK
		reqAuth := httptest.NewRequest(http.MethodGet, "http://pr-auth.preview.example.com/", nil)
		reqAuth.SetBasicAuth("tester", "secret-password")
		wAuth := httptest.NewRecorder()
		proxy.ServeHTTP(wAuth, reqAuth)
		if wAuth.Code != http.StatusOK {
			t.Fatalf("expected 200 OK with correct auth, got %d", wAuth.Code)
		}

		// Authenticated via ?preview_key= query param -> 200 OK
		reqQuery := httptest.NewRequest(http.MethodGet, "http://pr-auth.preview.example.com/?preview_key=secret-password", nil)
		wQuery := httptest.NewRecorder()
		proxy.ServeHTTP(wQuery, reqQuery)
		if wQuery.Code != http.StatusOK {
			t.Fatalf("expected 200 OK with query key, got %d", wQuery.Code)
		}
	})

	// 3. Test Wake-on-Request
	t.Run("Wake on request", func(t *testing.T) {
		var wakeTriggered bool
		proxy.SetWakeHandler(func(depID string) error {
			if depID == "pr-sleep-test" {
				wakeTriggered = true
			}
			return nil
		})

		_ = proxy.RegisterRouteWithConfig("pr-sleep.preview.example.com", RouteConfig{
			TargetPort:   backendPort,
			DeploymentID: "pr-sleep-test",
			IsSleeping:   true,
		})

		req := httptest.NewRequest(http.MethodGet, "http://pr-sleep.preview.example.com/", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202 Accepted while waking up, got %d", w.Code)
		}

		bodyBytes, _ := io.ReadAll(w.Body)
		if !strings.Contains(string(bodyBytes), "Waking Up Preview Stack") {
			t.Fatalf("expected waking up HTML template, got %s", string(bodyBytes))
		}

		// Allow async goroutine to execute
		time.Sleep(50 * time.Millisecond)
		if !wakeTriggered {
			t.Fatalf("expected wake handler to be triggered")
		}
	})
}
