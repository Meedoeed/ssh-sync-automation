package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/handler"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/encryption"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/server"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/Meedoeed/ssh-sync-automation/internal/storage/postgres"
	"github.com/Meedoeed/ssh-sync-automation/internal/worker"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ssh-sync-service",
	Short: "SSH Sync Automation Service",
	Long: `SSH Sync Automation Service — синхронизация файлов между серверами по SSH/SFTP.

Режимы запуска:
  ssh-sync-service              - Запуск всех компонентов (монолит)
  ssh-sync-service backend      - Запуск только backend API сервера
  ssh-sync-service scheduler    - Запуск планировщика задач
  ssh-sync-service worker       - Запуск worker для выполнения синхронизации`,
	Run: runMonolith,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Get().Fatal().Err(err).Msg("Command execution failed")
	}
}

func init() {
	rootCmd.AddCommand(backendCmd)
	rootCmd.AddCommand(schedulerCmd)
	rootCmd.AddCommand(workerCmd)
}

func runMonolith(cmd *cobra.Command, args []string) {
	if err := godotenv.Load(); err != nil {
		println("No .env file found")
	}

	cfg := config.Load()
	validateEncryption(cfg)

	logger.Init(cfg.Log.Level, true)
	log := logger.Get()
	log.Info().Msg("Starting SSH-SYNC-AUTOMATION service (monolith mode)")

	encryptor := encryption.NewEncryptor(cfg.Encryption.Key)
	ctx := context.Background()

	db, err := postgres.NewDB(ctx, &cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	serverRepo := postgres.NewServerRepo(db.Pool, encryptor)
	taskRepo := postgres.NewTaskRepo(db.Pool)
	statusRepo := postgres.NewServerStatusRepo(db.Pool)

	serverService := service.NewServerService(serverRepo, statusRepo)
	taskService := service.NewTaskService(taskRepo)
	syncService := service.NewSyncService(serverRepo, taskRepo, statusRepo, "./data")

	poolConfig := &worker.PoolConfig{
		Interval:    cfg.Sync.Interval,
		SyncCfg:     &cfg.Sync,
		ServerRepo:  serverRepo,
		StatusRepo:  statusRepo,
		SyncService: syncService,
	}
	workerPool := worker.NewPool(poolConfig)

	if err := workerPool.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to start worker pool")
	}
	defer workerPool.StopAll()

	healthHandler := handler.NewHealthHandler(db)
	serverHandler := handler.NewServerHandler(serverService, workerPool)
	taskHandler := handler.NewTaskHandler(taskService)

	httpServer := server.NewHTTP(cfg, db, encryptor, serverService, taskService, syncService)
	httpServer.SetValidator(handler.NewCustomValidator())
	handler.RegisterRoutes(httpServer.GetEcho(), healthHandler, serverHandler, taskHandler)

	go func() {
		if err := httpServer.Start(); err != nil {
			log.Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	waitForShutdown(httpServer)
	log.Info().Msg("Service stopped")
}

func waitForShutdown(httpServer *server.HTTPServer) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Get().Error().Err(err).Msg("HTTP server shutdown error")
	}
}

func validateEncryption(cfg *config.Config) {
	if cfg.Encryption.Key == "" {
		logger.Get().Fatal().Msg("ENCRYPTION_KEY is required")
	}
	if len(cfg.Encryption.Key) != 32 {
		logger.Get().Fatal().Msg("ENCRYPTION_KEY must be 32 bytes for AES-256")
	}
}
