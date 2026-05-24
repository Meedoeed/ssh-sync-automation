package cmd

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/handler"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/encryption"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/rpc"
	"github.com/Meedoeed/ssh-sync-automation/internal/server"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/Meedoeed/ssh-sync-automation/internal/storage/postgres"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var backendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Запуск backend API сервера",
	Long:  "Запускает HTTP API сервер для управления задачами и серверами",
	Run:   runBackend,
}

func runBackend(cmd *cobra.Command, args []string) {
	log.Println("Запуск в режиме BACKEND (только API сервер)")

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()
	validateEncryption(cfg)

	logger.Init(cfg.Log.Level, true)
	logger.Get().Info().Msg("Starting SSH-SYNC-AUTOMATION in BACKEND mode")

	encryptor := encryption.NewEncryptor(cfg.Encryption.Key)
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
	schedulerLeaderRepo := postgres.NewSchedulerLeaderRepo(db.Pool)

	rpcServer := rpc.NewBackendServer(taskRepo, serverRepo, schedulerLeaderRepo)
	rpcPath, rpcHandler := genconnect.NewBackendServiceHandler(rpcServer)

	rpcMux := http.NewServeMux()
	rpcMux.Handle(rpcPath, rpcHandler)

	go func() {
		logger.Get().Info().Msg("Starting RPC server on :8082")
		if err := http.ListenAndServe(":8082", rpcMux); err != nil {
			logger.Get().Fatal().Err(err).Msg("Failed to start RPC server")
		}
	}()

	go rpcServer.StartHeartbeatMonitor(context.Background())

	healthHandler := handler.NewHealthHandler(db)
	serverHandler := handler.NewServerHandler(serverService)
	taskHandler := handler.NewTaskHandler(taskService)

	httpServer := server.NewHTTP(cfg, db, encryptor, serverService, taskService, syncService)
	httpServer.SetValidator(handler.NewCustomValidator())
	handler.RegisterRoutes(httpServer.GetEcho(), healthHandler, serverHandler, taskHandler)

	go func() {
		if err := httpServer.Start(); err != nil {
			logger.Get().Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	logger.Get().Info().Msg("Backend API server started. Press Ctrl+C to stop.")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Get().Error().Err(err).Msg("HTTP server shutdown error")
	}

	logger.Get().Info().Msg("Backend stopped")
}
