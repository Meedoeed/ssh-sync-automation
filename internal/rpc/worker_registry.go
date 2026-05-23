package rpc

import (
	"sync"
	"time"
)

type WorkerRegistry struct {
	workers map[string]time.Time
	mu      sync.RWMutex
}

func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		workers: make(map[string]time.Time),
	}
}

func (r *WorkerRegistry) Register(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers[workerID] = time.Now()
}

func (r *WorkerRegistry) Heartbeat(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers[workerID] = time.Now()
}

func (r *WorkerRegistry) GetDeadWorkers(timeout time.Duration) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var dead []string
	now := time.Now()
	for workerID, lastHeartbeat := range r.workers {
		if now.Sub(lastHeartbeat) > timeout {
			dead = append(dead, workerID)
		}
	}
	return dead
}

func (r *WorkerRegistry) Unregister(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.workers, workerID)
}
