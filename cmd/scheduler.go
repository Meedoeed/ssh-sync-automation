package cmd

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/leader"
	"github.com/Meedoeed/ssh-sync-automation/internal/runner"
	"github.com/Meedoeed/ssh-sync-automation/proto/genconnect"
)

var (
	schedulerBackendAddr string
	schedulerInterval    time.Duration
	dataDir              string
	schedulerID          string
)

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Запуск планировщика задач",
	Long: `Запускает планировщик для создания задач синхронизации.
	
Планировщик поддерживает leader election — можно запустить несколько инстансов,
но задачи будет выполнять только лидер. Остальные находятся в режиме ожидания.`,
	Run: runScheduler,
}

func init() {
	schedulerCmd.Flags().StringVar(&schedulerBackendAddr, "backend", "http://localhost:8082", "Адрес backend RPC сервера")
	schedulerCmd.Flags().DurationVar(&schedulerInterval, "interval", 30*time.Second, "Интервал между циклами планировщика")
	schedulerCmd.Flags().StringVar(&dataDir, "data-dir", "./data", "Директория для хранения данных")
	schedulerCmd.Flags().StringVar(&schedulerID, "id", "", "Уникальный ID шедулера (генерируется автоматически если не указан)")
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

	if schedulerID == "" {
		schedulerID = generateSchedulerID()
		log.Info().Str("generated_id", schedulerID).Msg("Auto-generated scheduler ID")
	}

	log.Info().
		Dur("interval", schedulerInterval).
		Str("backend_addr", schedulerBackendAddr).
		Str("data_dir", dataDir).
		Str("scheduler_id", schedulerID).
		Msg("Scheduler started")

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	rpcClient := genconnect.NewBackendServiceClient(httpClient, schedulerBackendAddr)

	election := leader.NewRPCElection(schedulerID, rpcClient, 30*time.Second)

	electionCtx, electionCancel := context.WithCancel(context.Background())
	go election.Start(electionCtx)

	for !election.IsLeader() {
		log.Debug().Msg("Waiting to become leader...")
		time.Sleep(2 * time.Second)
	}

	log.Info().Msg("Became leader, starting scheduler loop")

	ctx, cancel := context.WithCancel(context.Background())

	go runner.RunSchedulerLoop(ctx, rpcClient, cfg, dataDir)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down scheduler...")
	electionCancel()
	cancel()
	time.Sleep(2 * time.Second)
	log.Info().Msg("Scheduler stopped")
}

func generateSchedulerID() string {
	return "scheduler-" + time.Now().Format("20060102150405") + "-" + uuid.New().String()[:8]
}
