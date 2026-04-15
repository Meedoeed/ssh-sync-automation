package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/repository"
	"github.com/google/uuid"
)

type SyncService struct {
	serverRepo   repository.ServerRepository
	taskRepo     repository.TaskRepository
	statusRepo   repository.ServerStatusRepository
	baseLocalDir string
}

func NewSyncService(
	serverRepo repository.ServerRepository,
	taskRepo repository.TaskRepository,
	statusRepo repository.ServerStatusRepository,
	baseLocalDir string,
) *SyncService {
	return &SyncService{
		serverRepo:   serverRepo,
		taskRepo:     taskRepo,
		statusRepo:   statusRepo,
		baseLocalDir: baseLocalDir,
	}
}

func (s *SyncService) SyncServer(ctx context.Context, serverID uuid.UUID, sshClient infrastructure.SSHClientInterface) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}
	if server == nil {
		return fmt.Errorf("server not found")
	}

	logger.Get().Info().
		Str("server_id", serverID.String()).
		Str("server_name", server.Name).
		Msg("Starting sync for server")

	if err := s.syncDownload(ctx, server, sshClient); err != nil {
		logger.Get().Error().
			Err(err).
			Str("server_id", serverID.String()).
			Msg("Download sync failed")
	}

	if err := s.syncUpload(ctx, server, sshClient); err != nil {
		logger.Get().Error().
			Err(err).
			Str("server_id", serverID.String()).
			Msg("Upload sync failed")
		return err
	}

	logger.Get().Info().
		Str("server_id", serverID.String()).
		Str("server_name", server.Name).
		Msg("Sync completed for server")

	return nil
}

func (s *SyncService) syncDownload(ctx context.Context, server *domain.Server, sshClient infrastructure.SSHClientInterface) error {
	remotePath := "done/"
	localPath := filepath.Join(s.baseLocalDir, "done", server.Name)

	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	files, err := sshClient.ListFiles(remotePath)
	if err != nil {
		errMsg := err.Error()
		_ = s.updateServerStatus(ctx, server.ID, "error", &errMsg)
		return fmt.Errorf("failed to list files: %w", err)
	}

	if len(files) == 0 {
		logger.Get().Debug().
			Str("server", server.Name).
			Msg("No files to download")
		return nil
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		remoteFilePath := filepath.Join(remotePath, file)
		localFilePath := filepath.Join(localPath, file)

		task := &domain.SyncTask{
			ID:         uuid.New(),
			ServerID:   server.ID,
			Direction:  domain.DirectionDownload,
			FileName:   file,
			RemotePath: remoteFilePath,
			LocalPath:  localFilePath,
			Status:     domain.StatusPending,
		}

		if err := s.taskRepo.Create(ctx, task); err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file).
				Msg("Failed to create download task")
			continue
		}

		if err := sshClient.DownloadWithRetry(remoteFilePath, localFilePath); err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file).
				Msg("Failed to download file")

			errMsg := err.Error()
			if err := s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusFailed, &errMsg); err != nil {
				logger.Get().Warn().Err(err).Msg("Failed to update task status")
			}
			continue
		}

		if err := sshClient.DeleteFile(remoteFilePath); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("file", file).
				Msg("Failed to delete remote file after download")
		}

		if err := s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusCompleted, nil); err != nil {
			logger.Get().Warn().Err(err).Msg("Failed to update task status")
		}

		logger.Get().Info().
			Str("server", server.Name).
			Str("file", file).
			Msg("File downloaded successfully")
	}

	return nil
}

func (s *SyncService) syncUpload(ctx context.Context, server *domain.Server, sshClient infrastructure.SSHClientInterface) error {
	localPath := filepath.Join(s.baseLocalDir, "tasks", server.Name)
	remotePath := "tasks/"

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		logger.Get().Debug().
			Str("server", server.Name).
			Msg("No tasks directory, nothing to upload")
		return nil
	}

	files, err := os.ReadDir(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local directory: %w", err)
	}

	if len(files) == 0 {
		logger.Get().Debug().
			Str("server", server.Name).
			Msg("No files to upload")
		return nil
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		localFilePath := filepath.Join(localPath, file.Name())
		remoteFilePath := filepath.Join(remotePath, file.Name())

		task := &domain.SyncTask{
			ID:         uuid.New(),
			ServerID:   server.ID,
			Direction:  domain.DirectionUpload,
			FileName:   file.Name(),
			RemotePath: remoteFilePath,
			LocalPath:  localFilePath,
			Status:     domain.StatusPending,
		}

		if err := s.taskRepo.Create(ctx, task); err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file.Name()).
				Msg("Failed to create upload task")
			continue
		}

		if err := sshClient.UploadWithRetry(localFilePath, remoteFilePath); err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file.Name()).
				Msg("Failed to upload file")

			errMsg := err.Error()
			if err := s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusFailed, &errMsg); err != nil {
				logger.Get().Warn().Err(err).Msg("Failed to update task status")
			}
			continue
		}

		if err := os.Remove(localFilePath); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("file", file.Name()).
				Msg("Failed to delete local file after upload")
		}

		if err := s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusCompleted, nil); err != nil {
			logger.Get().Warn().Err(err).Msg("Failed to update task status")
		}

		logger.Get().Info().
			Str("server", server.Name).
			Str("file", file.Name()).
			Msg("File uploaded successfully")
	}

	return nil
}

func (s *SyncService) updateServerStatus(ctx context.Context, serverID uuid.UUID, status string, errMsg *string) error {
	serverStatus := &domain.ServerStatus{
		ID:           uuid.New(),
		ServerID:     serverID,
		Status:       status,
		ErrorMessage: errMsg,
	}

	return s.statusRepo.Create(ctx, serverStatus)
}
