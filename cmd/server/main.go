package main

import (
	"context"
	"log"
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
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()

	if cfg.Encryption.Key == "" {
		log.Fatal("ENCRYPTION_KEY is required")
	}
	if len(cfg.Encryption.Key) != 32 {
		log.Fatal("ENCRYPTION_KEY must be 32 bytes for AES-256")
	}

	logger.Init(cfg.Log.Level, true)
	logger.Get().Info().Msg("Starting SSH-SYNC-AUTOMATION service")

	encryptor := encryption.NewEncryptor(cfg.Encryption.Key)
	logger.Get().Info().Msg("Encryption initialized")

	ctx := context.Background()
	db, err := postgres.NewDB(ctx, &cfg.Database)
	if err != nil {
		logger.Get().Fatal().Err(err).Msg("Failed to connect to database")
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
		logger.Get().Fatal().Err(err).Msg("Failed to start worker pool")
	}
	defer workerPool.StopAll()

	healthHandler := handler.NewHealthHandler(db)
	serverHandler := handler.NewServerHandler(serverService, workerPool)

	httpServer := server.NewHTTP(cfg, db, encryptor, serverService, taskService, syncService)

	httpServer.SetValidator(handler.NewCustomValidator())

	handler.RegisterRoutes(httpServer.GetEcho(), healthHandler, serverHandler)

	go func() {
		if err := httpServer.Start(); err != nil {
			logger.Get().Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Get().Error().Err(err).Msg("HTTP server shutdown error")
	}

	logger.Get().Info().Msg("Service stopped")
}
