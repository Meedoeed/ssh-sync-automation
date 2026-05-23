package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
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
	schedulerBackendAddr string
	schedulerInterval    time.Duration
	dataDir              string
)

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Запуск планировщика задач",
	Long: `Запускает планировщик для создания задач синхронизации.
	
Планировщик периодически:
  1. Получает список активных серверов из бэкенда
  2. Проверяет наличие файлов в ~/done/ на каждом сервере
  3. Проверяет наличие файлов в локальной директории ./data/tasks/<server_name>/
  4. Создаёт задачи через RPC`,
	Run: runScheduler,
}

func init() {
	schedulerCmd.Flags().StringVar(&schedulerBackendAddr, "backend", "http://localhost:8082", "Адрес backend RPC сервера")
	schedulerCmd.Flags().DurationVar(&schedulerInterval, "interval", 30*time.Second, "Интервал между циклами планировщика")
	schedulerCmd.Flags().StringVar(&dataDir, "data-dir", "./data", "Директория для хранения данных")
}

func runScheduler(cmd *cobra.Command, args []string) {
	log.Println("Запуск в режиме SCHEDULER (планировщик задач)")

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()
	logger.Init(cfg.Log.Level, true)
	log := logger.Get()
	log.Info().Msg("Starting SSH-SYNC-AUTOMATION in SCHEDULER mode")

	log.Info().
		Dur("interval", schedulerInterval).
		Str("backend_addr", schedulerBackendAddr).
		Str("data_dir", dataDir).
		Msg("Scheduler started")

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	rpcClient := genconnect.NewBackendServiceClient(httpClient, schedulerBackendAddr)

	ctx, cancel := context.WithCancel(context.Background())

	go schedulerLoop(ctx, rpcClient, cfg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down scheduler...")
	cancel()
	time.Sleep(2 * time.Second)
	log.Info().Msg("Scheduler stopped")
}

func schedulerLoop(ctx context.Context, client genconnect.BackendServiceClient, cfg *config.Config) {
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Get().Debug().Msg("Scheduler loop stopped")
			return
		case <-ticker.C:
			logger.Get().Debug().Msg("Scheduler tick: checking for files")
			runSchedulerCycle(ctx, client, cfg)
		}
	}
}

func runSchedulerCycle(ctx context.Context, client genconnect.BackendServiceClient, cfg *config.Config) {
	log := logger.Get()

	serversResp, err := client.GetServers(ctx, connect.NewRequest(&gen.GetServersRequest{
		ActiveOnly: true,
	}))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get servers from backend")
		return
	}

	servers := serversResp.Msg.Servers
	log.Debug().Int("count", len(servers)).Msg("Got servers from backend")

	for _, server := range servers {
		log.Debug().Str("server", server.Name).Msg("Checking server for files")

		if err := checkRemoteFiles(ctx, client, server, cfg); err != nil {
			log.Error().
				Err(err).
				Str("server", server.Name).
				Msg("Failed to check remote files")
		}

		if err := checkLocalFiles(ctx, client, server); err != nil {
			log.Error().
				Err(err).
				Str("server", server.Name).
				Msg("Failed to check local files")
		}
	}
}

func checkRemoteFiles(ctx context.Context, client genconnect.BackendServiceClient, server *gen.Server, cfg *config.Config) error {
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

	if err := sshClient.Connect(domainServer); err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", server.Name, err)
	}
	defer sshClient.Close()

	remoteDonePath := "done/"
	files, err := sshClient.ListFiles(remoteDonePath)
	if err != nil {
		return fmt.Errorf("failed to list files in done/: %w", err)
	}

	log.Debug().
		Str("server", server.Name).
		Int("files", len(files)).
		Msg("Found files in ~/done/")

	for _, fileName := range files {
		remotePath := path.Join("done/", fileName)
		localPath := filepath.Join(dataDir, "done", server.Name, fileName)

		// Проверяем, существует ли уже задача для этого файла
		checkResp, err := client.CheckTaskExists(ctx, connect.NewRequest(&gen.CheckTaskExistsRequest{
			ServerId:   server.Id,
			Direction:  "download",
			RemotePath: remotePath,
		}))
		if err != nil {
			log.Warn().
				Err(err).
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Failed to check if task exists, skipping")
			continue
		}

		if checkResp.Msg.Exists {
			log.Debug().
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Task already exists, skipping")
			continue
		}

		fileSize, err := sshClient.GetFileSize(remotePath)
		if err != nil {
			log.Warn().
				Err(err).
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Failed to get file size, using 0")
			fileSize = 0
		}

		_, err = client.CreateTask(ctx, connect.NewRequest(&gen.CreateTaskRequest{
			ServerId:   server.Id,
			Direction:  "download",
			FileName:   fileName,
			RemotePath: remotePath,
			LocalPath:  localPath,
			FileSize:   fileSize,
		}))
		if err != nil {
			log.Error().
				Err(err).
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Failed to create download task")
		} else {
			log.Info().
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Download task created")
		}
	}

	return nil
}

func checkLocalFiles(ctx context.Context, client genconnect.BackendServiceClient, server *gen.Server) error {
	log := logger.Get()

	tasksDir := filepath.Join(dataDir, "tasks", server.Name)

	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		log.Debug().
			Str("server", server.Name).
			Str("path", tasksDir).
			Msg("Tasks directory does not exist")
		return nil
	}

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return fmt.Errorf("failed to read tasks directory: %w", err)
	}

	log.Debug().
		Str("server", server.Name).
		Int("files", len(entries)).
		Msg("Found files in local tasks directory")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		localPath := filepath.Join(tasksDir, fileName)
		remotePath := path.Join("tasks/", fileName)

		// Проверяем, существует ли уже задача для этого файла
		checkResp, err := client.CheckTaskExists(ctx, connect.NewRequest(&gen.CheckTaskExistsRequest{
			ServerId:   server.Id,
			Direction:  "upload",
			RemotePath: remotePath,
		}))
		if err != nil {
			log.Warn().
				Err(err).
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Failed to check if task exists, skipping")
			continue
		}

		if checkResp.Msg.Exists {
			log.Debug().
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Task already exists, skipping")
			continue
		}

		fileInfo, err := entry.Info()
		fileSize := int64(0)
		if err == nil {
			fileSize = fileInfo.Size()
		}

		_, err = client.CreateTask(ctx, connect.NewRequest(&gen.CreateTaskRequest{
			ServerId:   server.Id,
			Direction:  "upload",
			FileName:   fileName,
			RemotePath: remotePath,
			LocalPath:  localPath,
			FileSize:   fileSize,
		}))
		if err != nil {
			log.Error().
				Err(err).
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Failed to create upload task")
		} else {
			log.Info().
				Str("server", server.Name).
				Str("file", fileName).
				Msg("Upload task created")
		}
	}

	return nil
}
