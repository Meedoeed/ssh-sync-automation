package cmd

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/gen"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/encryption"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/rpc"
	"github.com/Meedoeed/ssh-sync-automation/internal/runner"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/Meedoeed/ssh-sync-automation/internal/storage/postgres"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
)

var rootCmd = &cobra.Command{
	Use:   "ssh-sync-service",
	Short: "SSH Sync Automation Service",
	Long: `SSH Sync Automation Service — синхронизация файлов между серверами по SSH/SFTP.

Режимы запуска:
  ssh-sync-service              - Запуск всех компонентов (монолит через RPC)
  ssh-sync-service backend      - Запуск только backend RPC сервера
  ssh-sync-service scheduler    - Запуск планировщика задач
  ssh-sync-service worker       - Запуск worker для выполнения синхронизации`,
	Run: runMonolithRPC,
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

func runMonolithRPC(cmd *cobra.Command, args []string) {
	if err := godotenv.Load(); err != nil {
		println("No .env file found")
	}

	cfg := config.Load()
	validateEncryption(cfg)

	logger.Init(cfg.Log.Level, true)
	log := logger.Get()
	log.Info().Msg("Starting SSH-SYNC-AUTOMATION in MONOLITH mode (RPC-based)")

	encryptor := encryption.NewEncryptor(cfg.Encryption.Key)
	ctx := context.Background()

	db, err := postgres.NewDB(ctx, &cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	serverRepo := postgres.NewServerRepo(db.Pool, encryptor)
	taskRepo := postgres.NewTaskRepo(db.Pool)
	schedulerLeaderRepo := postgres.NewSchedulerLeaderRepo(db.Pool)
	probeTaskRepo := postgres.NewProbeTaskRepo(db.Pool)

	taskService := service.NewTaskService(taskRepo)
	rpcServer := rpc.NewBackendServer(taskRepo, serverRepo, schedulerLeaderRepo, probeTaskRepo, db.Pool)
	rpcPath, rpcHandler := genconnect.NewBackendServiceHandler(rpcServer)

	cleanupCtx, cleanupCancel := context.WithCancel(ctx)
	rpcServer.StartCleanupScheduler(cleanupCtx, taskService, 6*time.Hour, 7*24*time.Hour)
	defer cleanupCancel()

	rpcMux := http.NewServeMux()
	rpcMux.Handle(rpcPath, rpcHandler)

	go func() {
		log.Info().Msg("Starting RPC server on :8082")
		if err := http.ListenAndServe(":8082", rpcMux); err != nil {
			log.Fatal().Err(err).Msg("Failed to start RPC server")
		}
	}()

	go rpcServer.StartHeartbeatMonitor(ctx)

	rpcClient := genconnect.NewBackendServiceClient(
		&http.Client{Timeout: 30 * time.Second},
		"http://localhost:8082",
	)

	// Запуск шедулера
	schedulerCtx, schedulerCancel := context.WithCancel(ctx)
	go func() {
		log.Info().Msg("Starting internal scheduler")
		runner.RunSchedulerLoop(schedulerCtx, rpcClient, cfg, "./data")
	}()

	// Запуск воркера
	workerID := runner.GenerateWorkerID()
	dataDir := "./data"

	workerCtx, workerCancel := context.WithCancel(ctx)
	probeCtx, probeCancel := context.WithCancel(ctx)

	_, err = rpcClient.RegisterWorker(ctx, connect.NewRequest(&gen.RegisterWorkerRequest{
		WorkerId: workerID,
	}))
	if err != nil {
		log.Error().Err(err).Msg("Failed to register worker")
	} else {
		log.Info().Str("worker_id", workerID).Msg("Worker registered")
	}

	go func() {
		log.Info().Str("worker_id", workerID).Msg("Starting internal worker")
		go runner.RunHeartbeat(workerCtx, rpcClient, workerID)
		go runner.RunTaskLoop(workerCtx, rpcClient, workerID, cfg, dataDir)
		go runner.RunProbeTaskLoop(probeCtx, rpcClient, workerID, cfg, dataDir)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down monolith...")
	schedulerCancel()
	workerCancel()
	probeCancel()
	time.Sleep(3 * time.Second)

	log.Info().Msg("Monolith stopped")
}

func validateEncryption(cfg *config.Config) {
	if cfg.Encryption.Key == "" {
		logger.Get().Fatal().Msg("ENCRYPTION_KEY is required")
	}
	if len(cfg.Encryption.Key) != 32 {
		logger.Get().Fatal().Msg("ENCRYPTION_KEY must be 32 bytes for AES-256")
	}
}
