package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/alexandrmotologa/previewforge/internal/orchestrator"
	"github.com/alexandrmotologa/previewforge/internal/storage"
)

// HandleLogStream streams live build logs for a deployment over Server-Sent Events (SSE).
func HandleLogStream(store *storage.Store, broadcaster *orchestrator.LogBroadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}

		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// Path format: api/deployments/{id}/logs/stream
		if len(pathParts) < 4 {
			http.Error(w, "Invalid deployment ID", http.StatusBadRequest)
			return
		}
		deploymentID := pathParts[2]

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// 1. Send historical logs first
		historicalLogs, err := store.GetLogs(deploymentID, 500)
		if err == nil {
			for _, line := range historicalLogs {
				fmt.Fprintf(w, "data: %s\n\n", line)
			}
			flusher.Flush()
		}

		// 2. Subscribe to live log stream
		logChan := broadcaster.Subscribe(deploymentID)
		defer broadcaster.Unsubscribe(deploymentID, logChan)

		notify := r.Context().Done()

		for {
			select {
			case <-notify:
				return
			case line, ok := <-logChan:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			}
		}
	}
}
