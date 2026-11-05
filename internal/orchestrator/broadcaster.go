package orchestrator

import (
	"sync"
)

// LogBroadcaster manages active listener channels for real-time SSE log streaming.
type LogBroadcaster struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan string]bool
}

// NewLogBroadcaster constructs a LogBroadcaster.
func NewLogBroadcaster() *LogBroadcaster {
	return &LogBroadcaster{
		subscribers: make(map[string]map[chan string]bool),
	}
}

// Subscribe returns a channel that receives live log lines for a deployment.
func (b *LogBroadcaster) Subscribe(deploymentID string) chan string {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan string, 100)
	if _, ok := b.subscribers[deploymentID]; !ok {
		b.subscribers[deploymentID] = make(map[chan string]bool)
	}
	b.subscribers[deploymentID][ch] = true
	return ch
}

// Unsubscribe removes a channel listener.
func (b *LogBroadcaster) Unsubscribe(deploymentID string, ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.subscribers[deploymentID]; ok {
		delete(subs, ch)
		close(ch)
		if len(subs) == 0 {
			delete(b.subscribers, deploymentID)
		}
	}
}

// Broadcast sends a log line to all active listeners for the deployment.
func (b *LogBroadcaster) Broadcast(deploymentID, line string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if subs, ok := b.subscribers[deploymentID]; ok {
		for ch := range subs {
			select {
			case ch <- line:
			default:
				// Channel buffer full, skip to prevent blocking
			}
		}
	}
}
