package repository

import (
	"context"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/google/uuid"
)

type ServerRepository interface {
	Create(ctx context.Context, server *domain.Server) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Server, error)
	GetByName(ctx context.Context, name string) (*domain.Server, error)
	List(ctx context.Context, activeOnly bool) ([]*domain.Server, error)
	Update(ctx context.Context, server *domain.Server) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateLastSeen(ctx context.Context, id uuid.UUID) error
}

type ServerStatusRepository interface {
	Create(ctx context.Context, status *domain.ServerStatus) error
	GetLatestForServer(ctx context.Context, serverID uuid.UUID) (*domain.ServerStatus, error)
	ListByServer(ctx context.Context, serverID uuid.UUID, limit int) ([]*domain.ServerStatus, error)
}
