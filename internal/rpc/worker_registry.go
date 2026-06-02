package rpc

import (
	"sync"
	"time"
)

type WorkerInfo struct {
	LastHeartbeat time.Time
	CurrentTasks  int
}

type WorkerRegistry struct {
	workers map[string]*WorkerInfo
	mu      sync.RWMutex
}

type WorkerStat struct {
	WorkerID     string
	State        string
	LastSync     time.Time
	SyncCount    int64
	ErrorCount   int64
	LastError    string
	CurrentTasks int
}

type WorkerStats struct {
	TotalWorkers int
	Workers      []WorkerStat
}

func NewWorkerRegistry() *WorkerRegistry {
	return &WorkerRegistry{
		workers: make(map[string]*WorkerInfo),
	}
}

func (r *WorkerRegistry) Register(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers[workerID] = &WorkerInfo{
		LastHeartbeat: time.Now(),
		CurrentTasks:  0,
	}
}

func (r *WorkerRegistry) Heartbeat(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if info, exists := r.workers[workerID]; exists {
		info.LastHeartbeat = time.Now()
	}
}

func (r *WorkerRegistry) IncrementTaskCount(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if info, exists := r.workers[workerID]; exists {
		info.CurrentTasks++
	}
}

func (r *WorkerRegistry) DecrementTaskCount(workerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if info, exists := r.workers[workerID]; exists {
		if info.CurrentTasks > 0 {
			info.CurrentTasks--
		}
	}
}

func (r *WorkerRegistry) GetWorkerLoad(workerID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if info, exists := r.workers[workerID]; exists {
		return info.CurrentTasks
	}
	return 999
}

func (r *WorkerRegistry) GetLeastLoadedWorker() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var leastLoaded string
	minTasks := int(^uint(0) >> 1)

	for workerID, info := range r.workers {
		if info.CurrentTasks < minTasks {
			minTasks = info.CurrentTasks
			leastLoaded = workerID
		}
	}

	return leastLoaded
}

func (r *WorkerRegistry) GetDeadWorkers(timeout time.Duration) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var dead []string
	now := time.Now()
	for workerID, info := range r.workers {
		if now.Sub(info.LastHeartbeat) > timeout {
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

	for workerID, info := range r.workers {
		stats.Workers = append(stats.Workers, WorkerStat{
			WorkerID:     workerID,
			State:        "running",
			LastSync:     info.LastHeartbeat,
			CurrentTasks: info.CurrentTasks,
		})
	}

	return stats
}
