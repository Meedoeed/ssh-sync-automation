package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/repository"
	"github.com/google/uuid"
)

type TaskService struct {
	taskRepo repository.TaskRepository
}

func NewTaskService(taskRepo repository.TaskRepository) *TaskService {
	return &TaskService{
		taskRepo: taskRepo,
	}
}

func (s *TaskService) CreateTask(ctx context.Context, task *domain.SyncTask) error {
	if err := s.validateTask(task); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if task.ID == uuid.Nil {
		task.ID = uuid.New()
	}
	if task.Status == "" {
		task.Status = domain.StatusPending
	}
	if task.MaxAttempts == 0 {
		task.MaxAttempts = 5
	}

	if err := s.taskRepo.Create(ctx, task); err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	logger.Get().Info().
		Str("task_id", task.ID.String()).
		Str("server_id", task.ServerID.String()).
		Str("direction", string(task.Direction)).
		Str("file_name", task.FileName).
		Msg("Task created successfully")

	return nil
}

func (s *TaskService) GetTask(ctx context.Context, id uuid.UUID) (*domain.SyncTask, error) {
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}
	return task, nil
}

func (s *TaskService) ListServerTasks(ctx context.Context, serverID uuid.UUID, status *domain.SyncStatus, limit int) ([]*domain.SyncTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	tasks, err := s.taskRepo.ListByServer(ctx, serverID, status, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}
	return tasks, nil
}

func (s *TaskService) GetPendingTasks(ctx context.Context, limit int) ([]*domain.SyncTask, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	tasks, err := s.taskRepo.ListPending(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending tasks: %w", err)
	}
	return tasks, nil
}

func (s *TaskService) StartTask(ctx context.Context, id uuid.UUID) error {
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}

	if task.Status != domain.StatusPending {
		return fmt.Errorf("task status is %s, cannot start", task.Status)
	}

	now := time.Now()
	task.Status = domain.StatusProcessing
	task.StartedAt = &now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}

	logger.Get().Info().
		Str("task_id", id.String()).
		Msg("Task started")

	return nil
}

func (s *TaskService) UpdateTaskProgress(ctx context.Context, id uuid.UUID, bytesTransferred int64) error {
	if err := s.taskRepo.UpdateProgress(ctx, id, bytesTransferred); err != nil {
		return fmt.Errorf("failed to update progress: %w", err)
	}
	return nil
}

func (s *TaskService) CompleteTask(ctx context.Context, id uuid.UUID) error {
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}

	now := time.Now()
	task.Status = domain.StatusCompleted
	task.CompletedAt = &now
	task.ErrorMessage = nil
	task.WorkerID = nil // Очищаем ID воркера при завершении

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	logger.Get().Info().
		Str("task_id", id.String()).
		Int64("total_bytes", task.BytesTransferred).
		Msg("Task completed successfully")

	return nil
}

func (s *TaskService) FailTask(ctx context.Context, id uuid.UUID, errMsg string) error {
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}

	task.AttemptCount++

	if task.AttemptCount >= task.MaxAttempts {
		now := time.Now()
		task.Status = domain.StatusFailed
		task.CompletedAt = &now
		task.ErrorMessage = &errMsg
		task.WorkerID = nil // Очищаем ID воркера при окончательном провале

		logger.Get().Warn().
			Str("task_id", id.String()).
			Int("attempts", task.AttemptCount).
			Int("max_attempts", task.MaxAttempts).
			Str("error", errMsg).
			Msg("Task failed permanently")
	} else {
		task.Status = domain.StatusPending
		task.ErrorMessage = &errMsg
		task.WorkerID = nil // Очищаем ID воркера для перевыполнения

		logger.Get().Info().
			Str("task_id", id.String()).
			Int("attempt", task.AttemptCount).
			Int("max_attempts", task.MaxAttempts).
			Msg("Task will be retried")
	}

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to fail task: %w", err)
	}

	return nil
}

func (s *TaskService) CancelTask(ctx context.Context, id uuid.UUID) error {
	task, err := s.taskRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found")
	}

	if task.Status == domain.StatusCompleted {
		return fmt.Errorf("cannot cancel completed task")
	}

	now := time.Now()
	task.Status = domain.StatusCancelled
	task.CompletedAt = &now
	task.WorkerID = nil // Очищаем ID воркера при отмене

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to cancel task: %w", err)
	}

	logger.Get().Info().
		Str("task_id", id.String()).
		Msg("Task cancelled")

	return nil
}

func (s *TaskService) CleanupOldTasks(ctx context.Context, olderThan time.Duration) (int64, error) {
	count, err := s.taskRepo.DeleteCompletedOlderThan(ctx, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup tasks: %w", err)
	}

	if count > 0 {
		logger.Get().Info().
			Int64("deleted_count", count).
			Dur("older_than", olderThan).
			Msg("Cleaned up old tasks")
	}

	return count, nil
}

func (s *TaskService) validateTask(task *domain.SyncTask) error {
	if task.ServerID == uuid.Nil {
		return fmt.Errorf("server_id is required")
	}
	if task.Direction != domain.DirectionUpload && task.Direction != domain.DirectionDownload {
		return fmt.Errorf("invalid direction: %s", task.Direction)
	}
	if task.FileName == "" {
		return fmt.Errorf("file_name is required")
	}
	if task.RemotePath == "" {
		return fmt.Errorf("remote_path is required")
	}
	if task.LocalPath == "" {
		return fmt.Errorf("local_path is required")
	}
	return nil
}

func (s *TaskService) GetAllTasks(ctx context.Context, limit int) ([]*domain.SyncTask, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	tasks, err := s.taskRepo.ListAll(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tasks: %w", err)
	}

	return tasks, nil
}

// ReassignWorkerTasks переводит все задачи указанного воркера обратно в статус pending
func (s *TaskService) ReassignWorkerTasks(ctx context.Context, workerID string) (int, error) {
	tasks, err := s.taskRepo.GetTasksByWorker(ctx, workerID)
	if err != nil {
		return 0, fmt.Errorf("failed to get tasks by worker: %w", err)
	}

	reassignedCount := 0
	for _, task := range tasks {
		if err := s.taskRepo.ReassignTask(ctx, task.ID); err != nil {
			logger.Get().Error().
				Err(err).
				Str("task_id", task.ID.String()).
				Str("worker_id", workerID).
				Msg("Failed to reassign task")
			continue
		}
		reassignedCount++
		logger.Get().Info().
			Str("task_id", task.ID.String()).
			Str("worker_id", workerID).
			Msg("Task reassigned from dead worker")
	}

	return reassignedCount, nil
}
