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

type ServerService struct {
	serverRepo repository.ServerRepository
	statusRepo repository.ServerStatusRepository
}

func NewServerService(
	serverRepo repository.ServerRepository,
	statusRepo repository.ServerStatusRepository,
) *ServerService {
	return &ServerService{
		serverRepo: serverRepo,
		statusRepo: statusRepo,
	}
}

func (s *ServerService) CreateServer(ctx context.Context, server *domain.Server) error {
	if err := s.validateServer(server); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	existing, err := s.serverRepo.GetByName(ctx, server.Name)
	if err != nil {
		return fmt.Errorf("failed to check server name: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("server with name %s already exists", server.Name)
	}

	if err := s.serverRepo.Create(ctx, server); err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	logger.Get().Info().
		Str("server_id", server.ID.String()).
		Str("server_name", server.Name).
		Msg("Server created successfully")

	return nil
}

func (s *ServerService) GetServer(ctx context.Context, id uuid.UUID) (*domain.Server, error) {
	server, err := s.serverRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}
	if server == nil {
		return nil, fmt.Errorf("server not found")
	}
	return server, nil
}

func (s *ServerService) ListServers(ctx context.Context, activeOnly bool) ([]*domain.Server, error) {
	servers, err := s.serverRepo.List(ctx, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	return servers, nil
}

func (s *ServerService) UpdateServer(ctx context.Context, server *domain.Server) error {
	existing, err := s.serverRepo.GetByID(ctx, server.ID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("server not found")
	}

	if err := s.validateServer(server); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if existing.Name != server.Name {
		nameExists, err := s.serverRepo.GetByName(ctx, server.Name)
		if err != nil {
			return fmt.Errorf("failed to check server name: %w", err)
		}
		if nameExists != nil {
			return fmt.Errorf("server with name %s already exists", server.Name)
		}
	}

	if err := s.serverRepo.Update(ctx, server); err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	logger.Get().Info().
		Str("server_id", server.ID.String()).
		Str("server_name", server.Name).
		Msg("Server updated successfully")

	return nil
}

func (s *ServerService) DeleteServer(ctx context.Context, id uuid.UUID) error {
	existing, err := s.serverRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("server not found")
	}

	if err := s.serverRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	logger.Get().Info().
		Str("server_id", id.String()).
		Str("server_name", existing.Name).
		Msg("Server deleted successfully")

	return nil
}

func (s *ServerService) UpdateServerStatus(ctx context.Context, serverID uuid.UUID, status string, errMsg *string) error {
	serverStatus := &domain.ServerStatus{
		ID:           uuid.New(),
		ServerID:     serverID,
		Status:       status,
		LastChecked:  time.Now(),
		ErrorMessage: errMsg,
	}

	if err := s.statusRepo.Create(ctx, serverStatus); err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	if status == "online" {
		if err := s.serverRepo.UpdateLastSeen(ctx, serverID); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("server_id", serverID.String()).
				Msg("Failed to update last_seen")
		}
	}

	return nil
}

func (s *ServerService) GetServerStatus(ctx context.Context, serverID uuid.UUID) (*domain.ServerStatus, error) {
	status, err := s.statusRepo.GetLatestForServer(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server status: %w", err)
	}
	return status, nil
}

func (s *ServerService) validateServer(server *domain.Server) error {
	if server.Name == "" {
		return fmt.Errorf("server name is required")
	}
	if server.Host == "" {
		return fmt.Errorf("host is required")
	}
	if server.Port < 1 || server.Port > 65535 {
		return fmt.Errorf("invalid port: %d", server.Port)
	}
	if server.Username == "" {
		return fmt.Errorf("username is required")
	}

	switch server.AuthType {
	case "password":
		if server.Password == nil || *server.Password == "" {
			return fmt.Errorf("password is required for password auth")
		}
	case "key":
		if server.PrivateKey == nil || *server.PrivateKey == "" {
			return fmt.Errorf("private key is required for key auth")
		}
	default:
		return fmt.Errorf("invalid auth type: %s, must be 'password' or 'key'", server.AuthType)
	}

	return nil
}
