package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/repository"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/google/uuid"
)

type State string

const (
	StateStopped State = "stopped"
	StateRunning State = "running"
	StateError   State = "error"
)

type Worker struct {
	mu          sync.RWMutex
	id          uuid.UUID
	serverID    uuid.UUID
	serverName  string
	state       State
	sshClient   infrastructure.SSHClientInterface
	syncService *service.SyncService
	statusRepo  repository.ServerStatusRepository

	interval   time.Duration
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	lastSync   time.Time
	syncCount  int64
	errorCount int64
	lastError  string
}

type Config struct {
	Interval    time.Duration
	SyncService *service.SyncService
	StatusRepo  repository.ServerStatusRepository
}

func NewWorker(
	serverID uuid.UUID,
	serverName string,
	sshClient infrastructure.SSHClientInterface,
	cfg *Config,
) *Worker {
	return &Worker{
		id:          uuid.New(),
		serverID:    serverID,
		serverName:  serverName,
		state:       StateStopped,
		sshClient:   sshClient,
		syncService: cfg.SyncService,
		statusRepo:  cfg.StatusRepo,
		interval:    cfg.Interval,
	}
}

func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state == StateRunning {
		return fmt.Errorf("worker already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel
	w.state = StateRunning

	w.wg.Add(1)
	go w.run(ctx)

	logger.Get().Info().
		Str("worker_id", w.id.String()).
		Str("server_id", w.serverID.String()).
		Str("server_name", w.serverName).
		Dur("interval", w.interval).
		Msg("Worker started")

	return nil
}

func (w *Worker) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state != StateRunning {
		return fmt.Errorf("worker is not running")
	}

	w.state = StateStopped
	if w.cancelFunc != nil {
		w.cancelFunc()
	}
	w.wg.Wait()

	if err := w.sshClient.Close(); err != nil {
		logger.Get().Warn().
			Err(err).
			Str("server_name", w.serverName).
			Msg("Error closing SSH connection")
	}

	logger.Get().Info().
		Str("worker_id", w.id.String()).
		Str("server_name", w.serverName).
		Msg("Worker stopped")

	return nil
}

func (w *Worker) GetState() State {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.state
}

func (w *Worker) GetStats() (lastSync time.Time, syncCount, errorCount int64, lastError string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastSync, w.syncCount, w.errorCount, w.lastError
}

func (w *Worker) GetServerID() uuid.UUID {
	return w.serverID
}

func (w *Worker) GetServerName() string {
	return w.serverName
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.runSync(ctx)

	for {
		select {
		case <-ctx.Done():
			w.updateStatus(ctx, "offline", nil)
			return

		case <-ticker.C:
			w.mu.RLock()
			state := w.state
			w.mu.RUnlock()

			if state != StateRunning {
				continue
			}

			w.runSync(ctx)
		}
	}
}

func (w *Worker) runSync(ctx context.Context) {
	startTime := time.Now()

	logger.Get().Debug().
		Str("server_name", w.serverName).
		Msg("Starting sync cycle")

	w.updateStatus(ctx, "syncing", nil)

	if !w.sshClient.IsConnected() {
		logger.Get().Warn().
			Str("server_name", w.serverName).
			Msg("SSH connection lost, attempting to reconnect...")

		reconnectCtx := context.Background()

		err := w.syncService.SyncServer(reconnectCtx, w.serverID, w.sshClient)

		w.mu.Lock()
		w.lastSync = time.Now()
		if err != nil {
			w.errorCount++
			w.lastError = err.Error()
			w.updateStatus(ctx, "error", &w.lastError)
			logger.Get().Error().
				Err(err).
				Str("server_name", w.serverName).
				Dur("duration", time.Since(startTime)).
				Msg("Sync cycle failed after reconnect attempt")
		} else {
			w.syncCount++
			w.lastError = ""
			w.updateStatus(ctx, "online", nil)
			logger.Get().Info().
				Str("server_name", w.serverName).
				Dur("duration", time.Since(startTime)).
				Msg("Sync cycle completed successfully after reconnect")
		}
		w.mu.Unlock()
		return
	}

	err := w.syncService.SyncServer(ctx, w.serverID, w.sshClient)

	w.mu.Lock()
	w.lastSync = time.Now()
	if err != nil {
		w.errorCount++
		w.lastError = err.Error()
		w.updateStatus(ctx, "error", &w.lastError)
		logger.Get().Error().
			Err(err).
			Str("server_name", w.serverName).
			Dur("duration", time.Since(startTime)).
			Msg("Sync cycle failed")
	} else {
		w.syncCount++
		w.lastError = ""
		w.updateStatus(ctx, "online", nil)
		logger.Get().Info().
			Str("server_name", w.serverName).
			Dur("duration", time.Since(startTime)).
			Msg("Sync cycle completed successfully")
	}
	w.mu.Unlock()
}

func (w *Worker) updateStatus(ctx context.Context, status string, errMsg *string) {
	serverStatus := &domain.ServerStatus{
		ID:           uuid.New(),
		ServerID:     w.serverID,
		Status:       status,
		LastChecked:  time.Now(),
		ErrorMessage: errMsg,
	}

	if err := w.statusRepo.Create(ctx, serverStatus); err != nil {
		logger.Get().Warn().
			Err(err).
			Str("server_name", w.serverName).
			Msg("Failed to update server status")
	}
}
