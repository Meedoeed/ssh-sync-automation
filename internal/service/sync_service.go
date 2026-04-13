// internal/service/sync_service.go
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

// SyncServer выполняет синхронизацию для конкретного сервера
func (s *SyncService) SyncServer(ctx context.Context, serverID uuid.UUID, sshClient infrastructure.SSHClientInterface) error {
	// Получаем сервер
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

	// 1. Синхронизация download (забираем файлы из done/)
	if err := s.syncDownload(ctx, server, sshClient); err != nil {
		logger.Get().Error().
			Err(err).
			Str("server_id", serverID.String()).
			Msg("Download sync failed")
		// Не возвращаем ошибку, продолжаем с upload
	}

	// 2. Синхронизация upload (отправляем файлы в tasks/)
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

// syncDownload забирает файлы из удаленной done/ директории
func (s *SyncService) syncDownload(ctx context.Context, server *domain.Server, sshClient infrastructure.SSHClientInterface) error {
	remotePath := "done/"
	localPath := filepath.Join(s.baseLocalDir, "done", server.Name)

	// Создаем локальную директорию
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// Получаем список файлов на сервере
	files, err := sshClient.ListFiles(remotePath)
	if err != nil {
		// Обновляем статус сервера
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

	// Скачиваем каждый файл
	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		remoteFilePath := filepath.Join(remotePath, file)
		localFilePath := filepath.Join(localPath, file)

		// Создаем задачу
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

		// Выполняем скачивание с ретраями
		if err := sshClient.DownloadWithRetry(remoteFilePath, localFilePath); err != nil {
			logger.Get().Error().
				Err(err).
				Str("file", file).
				Msg("Failed to download file")

			// Обновляем статус задачи
			errMsg := err.Error()
			if err := s.taskRepo.UpdateStatus(ctx, task.ID, domain.StatusFailed, &errMsg); err != nil {
				logger.Get().Warn().Err(err).Msg("Failed to update task status")
			}
			continue
		}

		// Удаляем файл на сервере после успешного скачивания
		if err := sshClient.DeleteFile(remoteFilePath); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("file", file).
				Msg("Failed to delete remote file after download")
		}

		// Обновляем статус задачи
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

// syncUpload отправляет файлы в удаленную tasks/ директорию
func (s *SyncService) syncUpload(ctx context.Context, server *domain.Server, sshClient infrastructure.SSHClientInterface) error {
	localPath := filepath.Join(s.baseLocalDir, "tasks", server.Name)
	remotePath := "tasks/"

	// Проверяем существование локальной директории
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		logger.Get().Debug().
			Str("server", server.Name).
			Msg("No tasks directory, nothing to upload")
		return nil
	}

	// Читаем локальные файлы
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

	// Загружаем каждый файл
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

		// Создаем задачу
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

		// Выполняем загрузку с ретраями
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

		// Удаляем локальный файл после успешной загрузки
		if err := os.Remove(localFilePath); err != nil {
			logger.Get().Warn().
				Err(err).
				Str("file", file.Name()).
				Msg("Failed to delete local file after upload")
		}

		// Обновляем статус задачи
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

// updateServerStatus обновляет статус сервера
func (s *SyncService) updateServerStatus(ctx context.Context, serverID uuid.UUID, status string, errMsg *string) error {
	serverStatus := &domain.ServerStatus{
		ID:           uuid.New(),
		ServerID:     serverID,
		Status:       status,
		ErrorMessage: errMsg,
	}

	return s.statusRepo.Create(ctx, serverStatus)
}
