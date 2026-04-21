package repository

import (
	"context"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/google/uuid"
)

type TaskRepository interface {
	Create(ctx context.Context, task *domain.SyncTask) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.SyncTask, error)
	ListByServer(ctx context.Context, serverID uuid.UUID, status *domain.SyncStatus, limit int) ([]*domain.SyncTask, error)
	ListPending(ctx context.Context, limit int) ([]*domain.SyncTask, error)
	ListAll(ctx context.Context, limit int) ([]*domain.SyncTask, error) // добавить
	Update(ctx context.Context, task *domain.SyncTask) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SyncStatus, errMsg *string) error
	UpdateProgress(ctx context.Context, id uuid.UUID, bytesTransferred int64) error
	IncrementAttempt(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteCompletedOlderThan(ctx context.Context, olderThan time.Duration) (int64, error)
}
