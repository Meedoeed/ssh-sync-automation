package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/repository"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/google/uuid"
)

type Pool struct {
	mu          sync.RWMutex
	workers     map[uuid.UUID]*Worker
	serverRepo  repository.ServerRepository
	statusRepo  repository.ServerStatusRepository
	syncService *service.SyncService
	syncCfg     *config.SyncCfg
	interval    time.Duration
}

type PoolConfig struct {
	Interval    time.Duration
	SyncCfg     *config.SyncCfg
	ServerRepo  repository.ServerRepository
	StatusRepo  repository.ServerStatusRepository
	SyncService *service.SyncService
}

func NewPool(cfg *PoolConfig) *Pool {
	return &Pool{
		workers:     make(map[uuid.UUID]*Worker),
		serverRepo:  cfg.ServerRepo,
		statusRepo:  cfg.StatusRepo,
		syncService: cfg.SyncService,
		syncCfg:     cfg.SyncCfg,
		interval:    cfg.Interval,
	}
}

func (p *Pool) Start(ctx context.Context) error {
	logger.Get().Info().Msg("Starting worker pool")

	servers, err := p.serverRepo.List(ctx, true)
	if err != nil {
		return fmt.Errorf("failed to list active servers: %w", err)
	}

	logger.Get().Info().
		Int("server_count", len(servers)).
		Msg("Found active servers, creating workers")

	for _, server := range servers {
		if err := p.AddWorker(ctx, server); err != nil {
			logger.Get().Error().
				Err(err).
				Str("server_name", server.Name).
				Msg("Failed to create worker for server")
		}
	}

	logger.Get().Info().
		Int("worker_count", len(p.workers)).
		Msg("Worker pool started")

	return nil
}

func (p *Pool) AddWorker(ctx context.Context, server *domain.Server) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.workers[server.ID]; exists {
		return fmt.Errorf("worker for server %s already exists", server.Name)
	}

	sshClient := infrastructure.NewSSHClient(&config.SyncCfg{
		SSHConTimeout: p.syncCfg.SSHConTimeout,
		SSHKeepAlive:  p.syncCfg.SSHKeepAlive,
		RetryMaxAtmpt: p.syncCfg.RetryMaxAtmpt,
		RetryDelay:    p.syncCfg.RetryDelay,
	})

	if err := sshClient.Connect(server); err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	workerCfg := &Config{
		Interval:    p.interval,
		SyncService: p.syncService,
		StatusRepo:  p.statusRepo,
	}

	worker := NewWorker(server.ID, server.Name, sshClient, workerCfg)

	if err := worker.Start(ctx); err != nil {
		sshClient.Close()
		return fmt.Errorf("failed to start worker: %w", err)
	}

	p.workers[server.ID] = worker

	logger.Get().Info().
		Str("server_id", server.ID.String()).
		Str("server_name", server.Name).
		Msg("Worker added for server")

	return nil
}

func (p *Pool) RemoveWorker(serverID uuid.UUID) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	worker, exists := p.workers[serverID]
	if !exists {
		return fmt.Errorf("worker for server %s not found", serverID)
	}

	if err := worker.Stop(); err != nil {
		return fmt.Errorf("failed to stop worker: %w", err)
	}

	delete(p.workers, serverID)

	logger.Get().Info().
		Str("server_id", serverID.String()).
		Str("server_name", worker.GetServerName()).
		Msg("Worker removed")

	return nil
}

func (p *Pool) GetWorker(serverID uuid.UUID) (*Worker, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	worker, exists := p.workers[serverID]
	if !exists {
		return nil, fmt.Errorf("worker not found for server %s", serverID)
	}

	return worker, nil
}

func (p *Pool) ListWorkers() []*Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	workers := make([]*Worker, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, w)
	}

	return workers
}

func (p *Pool) StopAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for serverID, worker := range p.workers {
		if err := worker.Stop(); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("server_id", serverID.String()).
				Msg("Error stopping worker")
		}
	}

	p.workers = make(map[uuid.UUID]*Worker)

	logger.Get().Info().Msg("All workers stopped")
}

func (p *Pool) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := map[string]interface{}{
		"total_workers": len(p.workers),
		"workers":       make([]map[string]interface{}, 0),
	}

	workersStats := make([]map[string]interface{}, 0, len(p.workers))
	for _, worker := range p.workers {
		lastSync, syncCount, errorCount, lastError := worker.GetStats()
		workerStats := map[string]interface{}{
			"server_id":   worker.GetServerID().String(),
			"server_name": worker.GetServerName(),
			"state":       worker.GetState(),
			"last_sync":   lastSync,
			"sync_count":  syncCount,
			"error_count": errorCount,
			"last_error":  lastError,
		}
		workersStats = append(workersStats, workerStats)
	}
	stats["workers"] = workersStats

	return stats
}
