package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/gen"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
)

var (
	workerID    string
	backendAddr string
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Запуск worker для выполнения синхронизации",
	Long: `Запускает worker для выполнения задач синхронизации.
	
Worker подключается к backend через RPC и получает задачи для выполнения.
Поддерживается масштабирование — можно запустить несколько воркеров.`,
	Run: runWorker,
}

func init() {
	workerCmd.Flags().StringVar(&workerID, "id", "", "Уникальный ID воркера (генерируется автоматически если не указан)")
	workerCmd.Flags().StringVar(&backendAddr, "backend", "http://localhost:8082", "Адрес backend RPC сервера")
}

func runWorker(cmd *cobra.Command, args []string) {
	log.Println("Запуск в режиме WORKER (исполнитель задач)")

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()
	logger.Init(cfg.Log.Level, true)
	log := logger.Get()
	log.Info().Msg("Starting SSH-SYNC-AUTOMATION in WORKER mode")

	if workerID == "" {
		workerID = generateWorkerID()
		log.Info().Str("generated_id", workerID).Msg("Auto-generated worker ID")
	}

	log.Info().
		Str("worker_id", workerID).
		Str("backend_addr", backendAddr).
		Msg("Worker starting")

	// Создаём RPC клиент
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	rpcClient := genconnect.NewBackendServiceClient(httpClient, backendAddr)

	// Регистрируемся в бэкенде
	ctx := context.Background()
	registerResp, err := rpcClient.RegisterWorker(ctx, connect.NewRequest(&gen.RegisterWorkerRequest{
		WorkerId: workerID,
	}))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to register worker")
	}
	log.Info().Bool("success", registerResp.Msg.Success).Str("message", registerResp.Msg.Message).Msg("Worker registered")

	// Запускаем heartbeat горутину
	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	go sendHeartbeat(heartbeatCtx, rpcClient, workerID)

	// Запускаем основной цикл получения и выполнения задач
	taskCtx, taskCancel := context.WithCancel(ctx)
	go taskLoop(taskCtx, rpcClient, workerID, cfg)

	// Ожидание сигнала завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down worker...")
	heartbeatCancel()
	taskCancel()
	time.Sleep(2 * time.Second)
	log.Info().Msg("Worker stopped")
}

func sendHeartbeat(ctx context.Context, client genconnect.BackendServiceClient, workerID string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := client.Heartbeat(ctx, connect.NewRequest(&gen.HeartbeatRequest{
				WorkerId: workerID,
			}))
			if err != nil {
				logger.Get().Warn().Err(err).Str("worker_id", workerID).Msg("Heartbeat failed")
			}
		}
	}
}

func taskLoop(ctx context.Context, client genconnect.BackendServiceClient, workerID string, cfg *config.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Запрашиваем задачу
		resp, err := client.GetTask(ctx, connect.NewRequest(&gen.GetTaskRequest{
			WorkerId: workerID,
		}))
		if err != nil {
			logger.Get().Warn().Err(err).Msg("Failed to get task")
			time.Sleep(5 * time.Second)
			continue
		}

		if !resp.Msg.HasTask {
			// Нет задач, ждём
			time.Sleep(5 * time.Second)
			continue
		}

		task := resp.Msg.Task
		logger.Get().Info().
			Str("task_id", task.Id).
			Str("file", task.FileName).
			Str("direction", task.Direction).
			Msg("Task received, starting execution")

		// Выполняем задачу
		err = executeTask(ctx, task, workerID, client, cfg)
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

func executeTask(ctx context.Context, task *gen.Task, workerID string, client genconnect.BackendServiceClient, cfg *config.Config) error {
	log := logger.Get()

	// 1. Получаем данные сервера через RPC
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

	log.Info().
		Str("task_id", task.Id).
		Str("server_name", server.Name).
		Str("server_host", server.Host).
		Msg("Got server info")

	// 2. Создаём SSH клиент
	sshClient := infrastructure.NewSSHClient(&cfg.Sync)

	// Конвертируем proto Server в domain.Server
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

	// 3. Подключаемся
	if err := sshClient.Connect(domainServer); err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", server.Name, err)
	}
	defer sshClient.Close()

	log.Info().
		Str("task_id", task.Id).
		Str("server", server.Name).
		Msg("Connected to server")

	// 4. Выполняем задачу в зависимости от направления
	if task.Direction == "download" {
		// Скачивание файла
		err = sshClient.DownloadWithRetryProgress(task.RemotePath, task.LocalPath, func(downloaded, total int64) {
			// Обновляем прогресс через RPC
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

		// Удаляем файл на сервере после успешного скачивания
		if err := sshClient.DeleteFile(task.RemotePath); err != nil {
			log.Warn().Err(err).Msg("Failed to delete remote file after download")
		}

		log.Info().
			Str("task_id", task.Id).
			Str("file", task.FileName).
			Msg("File downloaded successfully")

	} else if task.Direction == "upload" {
		// Загрузка файла
		err = sshClient.UploadWithRetryProgress(task.LocalPath, task.RemotePath, func(uploaded, total int64) {
			// Обновляем прогресс через RPC
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

		// Удаляем локальный файл после успешной загрузки
		if err := os.Remove(task.LocalPath); err != nil {
			log.Warn().Err(err).Msg("Failed to delete local file after upload")
		}

		log.Info().
			Str("task_id", task.Id).
			Str("file", task.FileName).
			Msg("File uploaded successfully")
	} else {
		return fmt.Errorf("unknown direction: %s", task.Direction)
	}

	return nil
}

func generateWorkerID() string {
	return "worker-" + time.Now().Format("20060102150405")
}
