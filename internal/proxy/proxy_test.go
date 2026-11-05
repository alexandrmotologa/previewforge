package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestStandaloneProxy(t *testing.T) {
	// 1. Create a dummy backend web server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "test-app")
		fmt.Fprintf(w, "Hello from PR preview! Path: %s", r.URL.Path)
	}))
	defer backend.Close()

	// Extract port from backend URL
	parts := strings.Split(backend.URL, ":")
	backendPort, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		t.Fatalf("failed to parse backend port: %v", err)
	}

	proxy := NewStandaloneProxy()

	// 2. Test 404 when route does not exist
	t.Run("Route not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "http://pr-99.preview.example.com/", nil)
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", w.Code)
		}
	})

	// 3. Register route and test forwarding
	t.Run("Forward to backend", func(t *testing.T) {
		err := proxy.RegisterRoute("pr-42.preview.example.com", backendPort)
		if err != nil {
			t.Fatalf("failed to register route: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "http://pr-42.preview.example.com/checkout", nil)
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		respBody, _ := io.ReadAll(w.Body)
		if !strings.Contains(string(respBody), "Hello from PR preview! Path: /checkout") {
			t.Fatalf("unexpected body: %s", string(respBody))
		}
	})

	// 4. Test Route Removal
	t.Run("Remove route", func(t *testing.T) {
		_ = proxy.RemoveRoute("pr-42.preview.example.com")

		req := httptest.NewRequest(http.MethodGet, "http://pr-42.preview.example.com/", nil)
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404 after route removal, got %d", w.Code)
		}
	})
}
