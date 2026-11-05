package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/previewforge/internal/model"
	"github.com/alexandrmotologa/previewforge/internal/orchestrator"
	"github.com/alexandrmotologa/previewforge/internal/proxy"
	"github.com/alexandrmotologa/previewforge/internal/storage"
	"github.com/alexandrmotologa/previewforge/internal/webhook"
)

// Server coordinates HTTP traffic routing between the Web Studio, REST API, Webhooks, and Preview Proxies.
type Server struct {
	config       model.SystemConfig
	store        *storage.Store
	orchestrator *orchestrator.Orchestrator
	proxyRouter  proxy.ProxyRouter
	apiHandler   *APIHandler
	webhookHdlr  *webhook.Handler
	fileServer   http.Handler
	mux          *http.ServeMux
}

// NewServer constructs the application HTTP server.
func NewServer(
	cfg model.SystemConfig,
	store *storage.Store,
	orch *orchestrator.Orchestrator,
	proxyRouter proxy.ProxyRouter,
	webhookHdlr *webhook.Handler,
) *Server {
	apiHandler := NewAPIHandler(store, orch, cfg)
	fileServer := FileServer()

	s := &Server{
		config:       cfg,
		store:        store,
		orchestrator: orch,
		proxyRouter:  proxyRouter,
		apiHandler:   apiHandler,
		webhookHdlr:  webhookHdlr,
		fileServer:   fileServer,
		mux:          http.NewServeMux(),
	}

	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Health check
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"version": "1.0.0",
			"app":     "PreviewForge",
		})
	})

	// Webhook endpoint
	s.mux.Handle("/webhook", s.webhookHdlr)

	// REST API endpoints
	s.mux.HandleFunc("/api/deployments", func(w http.ResponseWriter, r *http.Request) {
		s.cors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		s.apiHandler.ListDeployments(w, r)
	})

	s.mux.HandleFunc("/api/deployments/", func(w http.ResponseWriter, r *http.Request) {
		s.cors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		path := r.URL.Path
		if strings.HasSuffix(path, "/logs/stream") {
			HandleLogStream(s.store, s.orchestrator.Broadcaster())(w, r)
			return
		}
		if strings.HasSuffix(path, "/logs") {
			s.apiHandler.GetLogs(w, r)
			return
		}
		if strings.HasSuffix(path, "/rebuild") {
			s.apiHandler.RebuildDeployment(w, r)
			return
		}
		if strings.HasSuffix(path, "/sleep") {
			s.apiHandler.SleepDeployment(w, r)
			return
		}
		if strings.HasSuffix(path, "/wake") {
			s.apiHandler.WakeDeployment(w, r)
			return
		}
		if strings.HasSuffix(path, "/exec") {
			s.apiHandler.ExecContainer(w, r)
			return
		}

		if r.Method == http.MethodDelete {
			s.apiHandler.DestroyDeployment(w, r)
			return
		}

		s.apiHandler.GetDeployment(w, r)
	})

	s.mux.HandleFunc("/api/system/stats", func(w http.ResponseWriter, r *http.Request) {
		s.cors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		s.apiHandler.GetSystemStats(w, r)
	})

	s.mux.HandleFunc("/api/system/config", func(w http.ResponseWriter, r *http.Request) {
		s.cors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		s.apiHandler.GetSystemConfig(w, r)
	})

	s.mux.HandleFunc("/api/simulator/deploy", func(w http.ResponseWriter, r *http.Request) {
		s.cors(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		s.apiHandler.SimulatorDeploy(w, r)
	})

	// Web studio static SPA
	s.mux.Handle("/", s.fileServer)
}

func (s *Server) cors(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
}

// ServeHTTP inspects the Host header: if it corresponds to an active preview stack,
// it delegates to the reverse proxy router; otherwise it serves Studio/API.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	// If hostname is registered in reverse proxy, forward traffic to container
	if _, isPreview := s.proxyRouter.GetTarget(host); isPreview {
		s.proxyRouter.ServeHTTP(w, r)
		return
	}

	// Otherwise, serve Web Studio and REST API
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe boots the HTTP server on configured address.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.config.PublicPort)
	return http.ListenAndServe(addr, s)
}
