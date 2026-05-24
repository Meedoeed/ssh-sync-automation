package cmd

import (
	"context"
	"log"
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
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/runner"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
)

var (
	workerID    string
	backendAddr string
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Запуск worker для выполнения синхронизации",
	Long:  "Запускает worker для выполнения задач синхронизации",
	Run:   runWorker,
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
		workerID = runner.GenerateWorkerID()
		log.Info().Str("generated_id", workerID).Msg("Auto-generated worker ID")
	}

	log.Info().
		Str("worker_id", workerID).
		Str("backend_addr", backendAddr).
		Msg("Worker starting")

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	rpcClient := genconnect.NewBackendServiceClient(httpClient, backendAddr)

	ctx := context.Background()

	registerResp, err := rpcClient.RegisterWorker(ctx, connect.NewRequest(&gen.RegisterWorkerRequest{
		WorkerId: workerID,
	}))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to register worker")
	}
	log.Info().Bool("success", registerResp.Msg.Success).Str("message", registerResp.Msg.Message).Msg("Worker registered")

	heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
	go runner.RunHeartbeat(heartbeatCtx, rpcClient, workerID)

	taskCtx, taskCancel := context.WithCancel(ctx)
	go runner.RunTaskLoop(taskCtx, rpcClient, workerID, cfg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down worker...")
	heartbeatCancel()
	taskCancel()
	time.Sleep(2 * time.Second)
	log.Info().Msg("Worker stopped")
}
