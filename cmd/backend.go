package cmd

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/encryption"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	myMiddleware "github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/middleware"
	"github.com/Meedoeed/ssh-sync-automation/internal/rpc"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/Meedoeed/ssh-sync-automation/internal/storage/postgres"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
)

var backendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Запуск backend RPC сервера",
	Long:  "Запускает RPC сервер для управления задачами и серверами",
	Run:   runBackend,
}

func runBackend(cmd *cobra.Command, args []string) {
	log.Println("Запуск в режиме BACKEND (только RPC сервер)")

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()
	validateEncryption(cfg)

	logger.Init(cfg.Log.Level, true)
	log := logger.Get()
	log.Info().Msg("Starting SSH-SYNC-AUTOMATION in BACKEND mode")

	encryptor := encryption.NewEncryptor(cfg.Encryption.Key)
	ctx := context.Background()

	db, err := postgres.NewDB(ctx, &cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	serverRepo := postgres.NewServerRepo(db.Pool, encryptor)
	taskRepo := postgres.NewTaskRepo(db.Pool)
	probeTaskRepo := postgres.NewProbeTaskRepo(db.Pool)
	schedulerLeaderRepo := postgres.NewSchedulerLeaderRepo(db.Pool)

	taskService := service.NewTaskService(taskRepo)

	rpcServer := rpc.NewBackendServer(taskRepo, serverRepo, schedulerLeaderRepo, probeTaskRepo, db.Pool)
	rpcPath, rpcHandler := genconnect.NewBackendServiceHandler(rpcServer)

	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	rpcServer.StartCleanupScheduler(cleanupCtx, taskService, 6*time.Hour, 7*24*time.Hour)
	defer cleanupCancel()

	rpcMux := http.NewServeMux()
	rpcMux.Handle(rpcPath, myMiddleware.CORSMiddleware(rpcHandler))

	go func() {
		log.Info().Msg("Starting RPC server on :8082")
		if err := http.ListenAndServe(":8082", rpcMux); err != nil {
			log.Fatal().Err(err).Msg("Failed to start RPC server")
		}
	}()

	go rpcServer.StartHeartbeatMonitor(context.Background())

	log.Info().Msg("Backend RPC server started. Press Ctrl+C to stop.")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Backend stopped")
}
