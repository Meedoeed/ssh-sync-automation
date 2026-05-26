package rpc

import (
	"sync"
	"time"
)

type WorkerRegistry struct {
	workers map[string]time.Time
	mu      sync.RWMutex
}

type WorkerStat struct {
	ServerID   string
	ServerName string
	State      string
	LastSync   time.Time
	SyncCount  int64
	ErrorCount int64
	LastError  string
}

type WorkerStats struct {
	TotalWorkers int
	Workers      []WorkerStat
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

func (r *WorkerRegistry) GetStats() WorkerStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := WorkerStats{
		TotalWorkers: len(r.workers),
		Workers:      make([]WorkerStat, 0, len(r.workers)),
	}

	for workerID, lastHeartbeat := range r.workers {
		stats.Workers = append(stats.Workers, WorkerStat{
			ServerID:   workerID,
			ServerName: workerID,
			State:      "running",
			LastSync:   lastHeartbeat,
			SyncCount:  0,
			ErrorCount: 0,
			LastError:  "",
		})
	}

	return stats
}
