package orchestrator

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/alexandrmotologa/previewforge/internal/storage"
)

// PortAllocator manages concurrent allocation of host ports for preview containers.
type PortAllocator struct {
	store     *storage.Store
	startPort int
	endPort   int
	mu        sync.Mutex
}

// NewPortAllocator constructs a new port allocator backed by SQLite.
func NewPortAllocator(store *storage.Store, startPort, endPort int) *PortAllocator {
	return &PortAllocator{
		store:     store,
		startPort: startPort,
		endPort:   endPort,
	}
}

// Allocate reserves a free host port that is both unallocated in the DB and free on the OS network stack.
func (p *PortAllocator) Allocate(deploymentID string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for attempts := 0; attempts < 50; attempts++ {
		candidatePort, err := p.store.AllocatePort(deploymentID, p.startPort, p.endPort)
		if err != nil {
			return 0, err
		}

		if isPortAvailable(candidatePort) {
			return candidatePort, nil
		}

		// Port is reserved in OS by external process; release and retry
		_ = p.store.ReleasePort(candidatePort)
		time.Sleep(10 * time.Millisecond)
	}

	return 0, fmt.Errorf("unable to find an unblocked host port in range %d-%d", p.startPort, p.endPort)
}

// Release marks a port as free for future allocations.
func (p *PortAllocator) Release(port int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.store.ReleasePort(port)
}

// ReleaseByDeployment frees any port tied to the deployment.
func (p *PortAllocator) ReleaseByDeployment(deploymentID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.store.ReleasePortByDeployment(deploymentID)
}

func isPortAvailable(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
