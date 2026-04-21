package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

	if !sshClient.IsConnected() {
		logger.Get().Warn().
			Str("server_name", server.Name).
			Msg("SSH connection lost, attempting to reconnect...")

		if err := sshClient.Close(); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("server_name", server.Name).
				Msg("Error closing old connection")
		}

		if err := sshClient.Connect(server); err != nil {
			logger.Get().Error().
				Err(err).
				Str("server_name", server.Name).
				Msg("Failed to reconnect")

			errMsg := err.Error()
			s.updateServerStatus(ctx, server.ID, "error", &errMsg)
			return fmt.Errorf("failed to reconnect: %w", err)
		}

		logger.Get().Info().
			Str("server_name", server.Name).
			Msg("SSH reconnected successfully")
	}

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

	s.updateServerStatus(ctx, server.ID, "online", nil)

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

		fileSize := s.getRemoteFileSize(sshClient, remoteFilePath)

		task := &domain.SyncTask{
			ID:               uuid.New(),
			ServerID:         server.ID,
			Direction:        domain.DirectionDownload,
			FileName:         file,
			RemotePath:       remoteFilePath,
			LocalPath:        localFilePath,
			FileSize:         fileSize,
			BytesTransferred: 0,
			Status:           domain.StatusPending,
		}

		if err := s.taskRepo.Create(ctx, task); err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file).
				Msg("Failed to create download task")
			continue
		}

		s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusProcessing, nil)

		err = sshClient.DownloadWithRetryProgress(remoteFilePath, localFilePath, func(downloaded, total int64) {
			s.taskRepo.UpdateProgress(ctx, task.ID, downloaded)
		})

		if err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file).
				Msg("Failed to download file")

			errMsg := err.Error()
			s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusFailed, &errMsg)
			continue
		}

		if err := sshClient.DeleteFile(remoteFilePath); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("file", file).
				Msg("Failed to delete remote file after download")
		}

		s.taskRepo.UpdateProgress(ctx, task.ID, fileSize)
		s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusCompleted, nil)

		logger.Get().Info().
			Str("server", server.Name).
			Str("file", file).
			Int64("size", fileSize).
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

		fileInfo, err := os.Stat(localFilePath)
		var fileSize int64 = 0
		if err == nil {
			fileSize = fileInfo.Size()
		}

		task := &domain.SyncTask{
			ID:               uuid.New(),
			ServerID:         server.ID,
			Direction:        domain.DirectionUpload,
			FileName:         file.Name(),
			RemotePath:       remoteFilePath,
			LocalPath:        localFilePath,
			FileSize:         fileSize,
			BytesTransferred: 0,
			Status:           domain.StatusPending,
		}

		if err := s.taskRepo.Create(ctx, task); err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file.Name()).
				Msg("Failed to create upload task")
			continue
		}

		s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusProcessing, nil)

		err = sshClient.UploadWithRetryProgress(localFilePath, remoteFilePath, func(uploaded, total int64) {
			s.taskRepo.UpdateProgress(ctx, task.ID, uploaded)

		})

		if err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file.Name()).
				Msg("Failed to upload file")

			errMsg := err.Error()
			s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusFailed, &errMsg)
			continue
		}

		if err := os.Remove(localFilePath); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("file", file.Name()).
				Msg("Failed to delete local file after upload")
		}

		s.taskRepo.UpdateProgress(ctx, task.ID, fileSize)
		s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusCompleted, nil)

		logger.Get().Info().
			Str("server", server.Name).
			Str("file", file.Name()).
			Int64("size", fileSize).
			Msg("File uploaded successfully")
	}

	return nil
}

func (s *SyncService) getRemoteFileSize(sshClient infrastructure.SSHClientInterface, remotePath string) int64 {
	size, err := sshClient.GetFileSize(remotePath)
	if err != nil {
		logger.Get().Warn().
			Err(err).
			Str("remote_path", remotePath).
			Msg("Failed to get remote file size")
		return 0
	}
	return size
}

func (s *SyncService) updateServerStatus(ctx context.Context, serverID uuid.UUID, status string, errMsg *string) error {
	serverStatus := &domain.ServerStatus{
		ID:           uuid.New(),
		ServerID:     serverID,
		Status:       status,
		LastChecked:  time.Now(),
		ErrorMessage: errMsg,
	}

	return s.statusRepo.Create(ctx, serverStatus)
}

func (s *SyncService) GetServer(ctx context.Context, serverID uuid.UUID) (*domain.Server, error) {
	return s.serverRepo.GetByID(ctx, serverID)
}
