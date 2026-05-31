package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/gen"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
)

func GenerateWorkerID() string {
	return "worker-" + time.Now().Format("20060102150405")
}

func RunHeartbeat(ctx context.Context, client genconnect.BackendServiceClient, workerID string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var consecutiveErrors int

	for {
		select {
		case <-ctx.Done():
			logger.Get().Debug().Str("worker_id", workerID).Msg("Heartbeat stopped")
			return
		case <-ticker.C:
			_, err := client.Heartbeat(ctx, connect.NewRequest(&gen.HeartbeatRequest{
				WorkerId: workerID,
			}))

			if err != nil {
				consecutiveErrors++
				delay := time.Duration(consecutiveErrors*2) * time.Second
				if delay > 30*time.Second {
					delay = 30 * time.Second
				}

				logger.Get().Warn().
					Err(err).
					Str("worker_id", workerID).
					Int("consecutive_errors", consecutiveErrors).
					Dur("delay", delay).
					Msg("Heartbeat failed, will retry")

				time.Sleep(delay)

				_, err = client.Heartbeat(ctx, connect.NewRequest(&gen.HeartbeatRequest{
					WorkerId: workerID,
				}))
				if err == nil {
					consecutiveErrors = 0
					logger.Get().Debug().Str("worker_id", workerID).Msg("Heartbeat recovered")
				}
			} else {
				consecutiveErrors = 0
			}
		}
	}
}

func RunSchedulerLoop(ctx context.Context, client genconnect.BackendServiceClient, cfg *config.Config, dataDir string) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Get().Debug().Msg("Scheduler loop stopped")
			return
		case <-ticker.C:
			RunSchedulerCycle(ctx, client, cfg, dataDir)
		}
	}
}

func RunSchedulerCycle(ctx context.Context, client genconnect.BackendServiceClient, cfg *config.Config, dataDir string) {
	log := logger.Get()

	var serversResp *connect.Response[gen.GetServersResponse]
	var err error

	for i := 0; i < 3; i++ {
		serversResp, err = client.GetServers(ctx, connect.NewRequest(&gen.GetServersRequest{
			ActiveOnly: true,
		}))
		if err == nil {
			break
		}
		log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to get servers, retrying...")
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to get servers from backend after retries")
		return
	}

	servers := serversResp.Msg.Servers
	log.Debug().Int("count", len(servers)).Msg("Got servers from backend")

	for _, server := range servers {
		log.Debug().Str("server", server.Name).Msg("Checking server for files")

		if err := CheckRemoteFiles(ctx, client, server, cfg, dataDir); err != nil {
			log.Error().Err(err).Str("server", server.Name).Msg("Failed to check remote files")
		}

		if err := CheckLocalFiles(ctx, client, server, dataDir); err != nil {
			log.Error().Err(err).Str("server", server.Name).Msg("Failed to check local files")
		}
	}
}

func CheckRemoteFiles(ctx context.Context, client genconnect.BackendServiceClient, server *gen.Server, cfg *config.Config, dataDir string) error {
	log := logger.Get()

	serverID, err := uuid.Parse(server.Id)
	if err != nil {
		return fmt.Errorf("invalid server ID: %w", err)
	}

	domainServer := &domain.Server{
		ID:       serverID,
		Name:     server.Name,
		Host:     server.Host,
		Port:     int(server.Port),
		Username: server.Username,
		AuthType: server.AuthType,
	}

	if server.AuthType == "password" {
		domainServer.Password = &server.Password
	} else if server.AuthType == "key" {
		domainServer.PrivateKey = &server.PrivateKey
	}

	sshClient := infrastructure.NewSSHClient(&cfg.Sync)

	if err := ConnectWithRetry(sshClient, domainServer); err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", server.Name, err)
	}
	defer sshClient.Close()

	files, err := sshClient.ListFiles("done/")
	if err != nil {
		return fmt.Errorf("failed to list files in done/: %w", err)
	}

	log.Debug().Str("server", server.Name).Int("files", len(files)).Msg("Found files in ~/done/")

	for _, fileName := range files {
		remotePath := "done/" + fileName
		localPath := dataDir + "/done/" + server.Name + "/" + fileName

		var checkResp *connect.Response[gen.CheckTaskExistsResponse]
		for i := 0; i < 3; i++ {
			checkResp, err = client.CheckTaskExists(ctx, connect.NewRequest(&gen.CheckTaskExistsRequest{
				ServerId:   server.Id,
				Direction:  "download",
				RemotePath: remotePath,
			}))
			if err == nil {
				break
			}
			log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to check task existence, retrying...")
			time.Sleep(time.Duration(i+1) * time.Second)
		}
		if err != nil {
			log.Warn().Err(err).Str("server", server.Name).Str("file", fileName).Msg("Failed to check if task exists, skipping")
			continue
		}

		if checkResp.Msg.Exists {
			log.Debug().Str("server", server.Name).Str("file", fileName).Msg("Task already exists, skipping")
			continue
		}

		fileSize, err := sshClient.GetFileSize(remotePath)
		if err != nil {
			log.Warn().Err(err).Str("server", server.Name).Str("file", fileName).Msg("Failed to get file size, using 0")
			fileSize = 0
		}

		for i := 0; i < 3; i++ {
			_, err = client.CreateTask(ctx, connect.NewRequest(&gen.CreateTaskRequest{
				ServerId:   server.Id,
				Direction:  "download",
				FileName:   fileName,
				RemotePath: remotePath,
				LocalPath:  localPath,
				FileSize:   fileSize,
			}))
			if err == nil {
				break
			}
			log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to create download task, retrying...")
			time.Sleep(time.Duration(i+1) * time.Second)
		}
		if err != nil {
			log.Error().Err(err).Str("server", server.Name).Str("file", fileName).Msg("Failed to create download task")
		} else {
			log.Info().Str("server", server.Name).Str("file", fileName).Msg("Download task created")
		}
	}

	return nil
}

func CheckLocalFiles(ctx context.Context, client genconnect.BackendServiceClient, server *gen.Server, dataDir string) error {
	log := logger.Get()

	tasksDir := dataDir + "/tasks/" + server.Name

	if _, err := CheckDirExists(tasksDir); err != nil {
		log.Debug().Str("server", server.Name).Str("path", tasksDir).Msg("Tasks directory does not exist")
		return nil
	}

	files, err := ReadDirFiles(tasksDir)
	if err != nil {
		return fmt.Errorf("failed to read tasks directory: %w", err)
	}

	log.Debug().Str("server", server.Name).Int("files", len(files)).Msg("Found files in local tasks directory")

	for _, fileName := range files {
		localPath := tasksDir + "/" + fileName
		remotePath := "tasks/" + fileName

		var checkResp *connect.Response[gen.CheckTaskExistsResponse]
		for i := 0; i < 3; i++ {
			checkResp, err = client.CheckTaskExists(ctx, connect.NewRequest(&gen.CheckTaskExistsRequest{
				ServerId:   server.Id,
				Direction:  "upload",
				RemotePath: remotePath,
			}))
			if err == nil {
				break
			}
			log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to check task existence, retrying...")
			time.Sleep(time.Duration(i+1) * time.Second)
		}
		if err != nil {
			log.Warn().Err(err).Str("server", server.Name).Str("file", fileName).Msg("Failed to check if task exists, skipping")
			continue
		}

		if checkResp.Msg.Exists {
			log.Debug().Str("server", server.Name).Str("file", fileName).Msg("Task already exists, skipping")
			continue
		}

		fileInfo, err := GetFileInfo(localPath)
		fileSize := int64(0)
		if err == nil {
			fileSize = fileInfo.Size()
		}

		// Создаём задачу (с retry)
		for i := 0; i < 3; i++ {
			_, err = client.CreateTask(ctx, connect.NewRequest(&gen.CreateTaskRequest{
				ServerId:   server.Id,
				Direction:  "upload",
				FileName:   fileName,
				RemotePath: remotePath,
				LocalPath:  localPath,
				FileSize:   fileSize,
			}))
			if err == nil {
				break
			}
			log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to create upload task, retrying...")
			time.Sleep(time.Duration(i+1) * time.Second)
		}
		if err != nil {
			log.Error().Err(err).Str("server", server.Name).Str("file", fileName).Msg("Failed to create upload task")
		} else {
			log.Info().Str("server", server.Name).Str("file", fileName).Msg("Upload task created")
		}
	}

	return nil
}

func RunTaskLoop(ctx context.Context, client genconnect.BackendServiceClient, workerID string, cfg *config.Config) {
	baseDelay := 2 * time.Second
	maxDelay := 60 * time.Second
	attempt := 0

	for {
		select {
		case <-ctx.Done():
			logger.Get().Debug().Str("worker_id", workerID).Msg("Task loop stopped")
			return
		default:
		}

		resp, err := client.GetTask(ctx, connect.NewRequest(&gen.GetTaskRequest{
			WorkerId: workerID,
		}))

		if err != nil {
			delay := baseDelay * time.Duration(1<<attempt)
			if delay > maxDelay {
				delay = maxDelay
			}
			attempt++

			logger.Get().Warn().
				Err(err).
				Str("worker_id", workerID).
				Int("attempt", attempt).
				Dur("next_retry", delay).
				Msg("Failed to get task, retrying...")

			time.Sleep(delay)
			continue
		}

		attempt = 0

		if !resp.Msg.HasTask {
			time.Sleep(5 * time.Second)
			continue
		}

		task := resp.Msg.Task
		logger.Get().Info().
			Str("task_id", task.Id).
			Str("file", task.FileName).
			Str("direction", task.Direction).
			Msg("Task received, starting execution")

		err = ExecuteTask(ctx, task, workerID, client, cfg)
		if err != nil {
			logger.Get().Error().Err(err).Str("task_id", task.Id).Msg("Task execution failed")

			_, failErr := client.FailTask(ctx, connect.NewRequest(&gen.FailTaskRequest{
				TaskId:       task.Id,
				WorkerId:     workerID,
				ErrorMessage: err.Error(),
			}))
			if failErr != nil {
				logger.Get().Error().Err(failErr).Msg("Failed to report task failure")
			}
		} else {
			_, completeErr := client.CompleteTask(ctx, connect.NewRequest(&gen.CompleteTaskRequest{
				TaskId:   task.Id,
				WorkerId: workerID,
			}))
			if completeErr != nil {
				logger.Get().Error().Err(completeErr).Msg("Failed to report task completion")
			}
		}
	}
}

func ExecuteTask(ctx context.Context, task *gen.Task, workerID string, client genconnect.BackendServiceClient, cfg *config.Config) error {
	log := logger.Get()

	if task.Direction == "probe_done" || task.Direction == "probe_tasks" {
		probeTask := &gen.ProbeTask{
			Id:       task.Id,
			ServerId: task.ServerId,
			TaskType: task.Direction,
		}
		return ExecuteProbeTask(ctx, probeTask, workerID, client, cfg)
	}

	var serverResp *connect.Response[gen.GetServerResponse]
	var err error

	for i := 0; i < 3; i++ {
		serverResp, err = client.GetServer(ctx, connect.NewRequest(&gen.GetServerRequest{
			ServerId: task.ServerId,
		}))
		if err == nil {
			break
		}
		log.Warn().Err(err).Int("attempt", i+1).Msg("Failed to get server info, retrying...")
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to get server info: %w", err)
	}

	server := serverResp.Msg.Server
	if server == nil {
		return fmt.Errorf("server not found: %s", task.ServerId)
	}

	log.Info().Str("task_id", task.Id).Str("server_name", server.Name).Str("server_host", server.Host).Msg("Got server info")

	sshClient := infrastructure.NewSSHClient(&cfg.Sync)

	serverID, err := uuid.Parse(server.Id)
	if err != nil {
		return fmt.Errorf("invalid server ID: %w", err)
	}

	domainServer := &domain.Server{
		ID:       serverID,
		Name:     server.Name,
		Host:     server.Host,
		Port:     int(server.Port),
		Username: server.Username,
		AuthType: server.AuthType,
		IsActive: server.IsActive,
	}

	if server.AuthType == "password" {
		domainServer.Password = &server.Password
	} else if server.AuthType == "key" {
		domainServer.PrivateKey = &server.PrivateKey
	}

	if err := ConnectWithRetry(sshClient, domainServer); err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", server.Name, err)
	}
	defer sshClient.Close()

	log.Info().Str("task_id", task.Id).Str("server", server.Name).Msg("Connected to server")

	if task.Direction == "download" {
		remotePath := task.RemotePath
		localPath := task.LocalPath

		if err := EnsureLocalDir(localPath); err != nil {
			return err
		}

		log.Info().Str("task_id", task.Id).Str("remote_path", remotePath).Str("local_path", localPath).Msg("Downloading file")

		err = sshClient.DownloadWithRetryProgress(remotePath, localPath, func(downloaded, total int64) {
			_, updateErr := client.UpdateTaskProgress(ctx, connect.NewRequest(&gen.UpdateProgressRequest{
				TaskId:           task.Id,
				WorkerId:         workerID,
				BytesTransferred: downloaded,
			}))
			if updateErr != nil {
				log.Warn().Err(updateErr).Msg("Failed to update progress")
			}
		})

		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}

		if err := sshClient.DeleteFile(remotePath); err != nil {
			log.Warn().Err(err).Msg("Failed to delete remote file after download")
		}

		log.Info().Str("task_id", task.Id).Str("file", task.FileName).Msg("File downloaded successfully")

	} else if task.Direction == "upload" {
		remotePath := task.RemotePath
		localPath := task.LocalPath

		log.Info().Str("task_id", task.Id).Str("local_path", localPath).Str("remote_path", remotePath).Msg("Uploading file")

		err = sshClient.UploadWithRetryProgress(localPath, remotePath, func(uploaded, total int64) {
			_, updateErr := client.UpdateTaskProgress(ctx, connect.NewRequest(&gen.UpdateProgressRequest{
				TaskId:           task.Id,
				WorkerId:         workerID,
				BytesTransferred: uploaded,
			}))
			if updateErr != nil {
				log.Warn().Err(updateErr).Msg("Failed to update progress")
			}
		})

		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}

		if err := RemoveFile(localPath); err != nil {
			log.Warn().Err(err).Msg("Failed to delete local file after upload")
		}

		log.Info().Str("task_id", task.Id).Str("file", task.FileName).Msg("File uploaded successfully")
	} else {
		return fmt.Errorf("unknown direction: %s", task.Direction)
	}

	return nil
}

func ConnectWithRetry(sshClient infrastructure.SSHClientInterface, server *domain.Server) error {
	maxAttempts := 5
	baseDelay := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := sshClient.Connect(server)
		if err == nil {
			if attempt > 1 {
				logger.Get().Info().
					Str("server", server.Name).
					Int("attempts", attempt).
					Msg("Successfully connected after retry")
			}
			return nil
		}

		delay := baseDelay * time.Duration(attempt*attempt)
		logger.Get().Warn().
			Err(err).
			Str("server", server.Name).
			Int("attempt", attempt).
			Int("max_attempts", maxAttempts).
			Dur("next_retry", delay).
			Msg("Connection attempt failed, retrying...")

		if attempt < maxAttempts {
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("failed to connect to server %s after %d attempts", server.Name, maxAttempts)
}

func CheckDirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func ReadDirFiles(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}
	return files, nil
}

func GetFileInfo(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func EnsureLocalDir(localPath string) error {
	dir := localPath
	if idx := strings.LastIndex(localPath, "/"); idx != -1 {
		dir = localPath[:idx]
	} else if idx := strings.LastIndex(localPath, "\\"); idx != -1 {
		dir = localPath[:idx]
	}
	if dir != localPath {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

func RemoveFile(path string) error {
	return os.Remove(path)
}

func RunProbeTaskLoop(ctx context.Context, client genconnect.BackendServiceClient, workerID string, cfg *config.Config) {
	for {
		select {
		case <-ctx.Done():
			logger.Get().Debug().Str("worker_id", workerID).Msg("Probe task loop stopped")
			return
		default:
		}

		resp, err := client.GetProbeTask(ctx, connect.NewRequest(&gen.GetProbeTaskRequest{
			WorkerId: workerID,
		}))
		if err != nil {
			logger.Get().Warn().Err(err).Msg("Failed to get probe task")
			time.Sleep(5 * time.Second)
			continue
		}

		if !resp.Msg.HasTask {
			time.Sleep(5 * time.Second)
			continue
		}

		task := resp.Msg.Task
		logger.Get().Info().
			Str("task_id", task.Id).
			Str("task_type", task.TaskType).
			Msg("Probe task received, starting execution")

		err = ExecuteProbeTask(ctx, task, workerID, client, cfg)
		if err != nil {
			logger.Get().Error().Err(err).Str("task_id", task.Id).Msg("Probe task execution failed")
			_, _ = client.ReportProbeResult(ctx, connect.NewRequest(&gen.ReportProbeResultRequest{
				ProbeTaskId: task.Id,
				WorkerId:    workerID,
				Error:       err.Error(),
			}))
		}
	}
}

func ExecuteProbeTask(ctx context.Context, task *gen.ProbeTask, workerID string, client genconnect.BackendServiceClient, cfg *config.Config) error {
	log := logger.Get()

	serverResp, err := client.GetServer(ctx, connect.NewRequest(&gen.GetServerRequest{
		ServerId: task.ServerId,
	}))
	if err != nil {
		return fmt.Errorf("failed to get server info: %w", err)
	}

	server := serverResp.Msg.Server
	if server == nil {
		return fmt.Errorf("server not found: %s", task.ServerId)
	}

	sshClient := infrastructure.NewSSHClient(&cfg.Sync)

	serverID, err := uuid.Parse(server.Id)
	if err != nil {
		return fmt.Errorf("invalid server ID: %w", err)
	}

	domainServer := &domain.Server{
		ID:       serverID,
		Name:     server.Name,
		Host:     server.Host,
		Port:     int(server.Port),
		Username: server.Username,
		AuthType: server.AuthType,
		IsActive: server.IsActive,
	}

	if server.AuthType == "password" {
		domainServer.Password = &server.Password
	} else if server.AuthType == "key" {
		domainServer.PrivateKey = &server.PrivateKey
	}

	if err := ConnectWithRetry(sshClient, domainServer); err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", server.Name, err)
	}
	defer sshClient.Close()

	var files []string

	// Исправлено: проверяем правильные типы
	if task.TaskType == "probe_done" {
		files, err = sshClient.ListFiles("done/")
		if err != nil {
			return fmt.Errorf("failed to list files in done/: %w", err)
		}
		log.Debug().Str("server", server.Name).Int("files", len(files)).Msg("Found files in ~/done/")
	} else if task.TaskType == "probe_tasks" {
		localPath := "./data/tasks/" + server.Name
		files, err = ReadDirFiles(localPath)
		if err != nil {
			log.Debug().Str("server", server.Name).Msg("No local tasks directory")
			files = []string{}
		} else {
			log.Debug().Str("server", server.Name).Int("files", len(files)).Msg("Found files in local tasks directory")
		}
	} else {
		return fmt.Errorf("unknown probe task type: %s", task.TaskType)
	}

	_, err = client.ReportProbeResult(ctx, connect.NewRequest(&gen.ReportProbeResultRequest{
		ProbeTaskId: task.Id,
		WorkerId:    workerID,
		FilesFound:  files,
	}))
	if err != nil {
		return fmt.Errorf("failed to report probe result: %w", err)
	}

	log.Info().
		Str("task_id", task.Id).
		Str("task_type", task.TaskType).
		Int("files_found", len(files)).
		Msg("Probe task completed")

	return nil
}
