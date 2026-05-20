package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	workerID   string
	workerMode string
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
	workerCmd.Flags().StringVar(&workerMode, "mode", "polling", "Режим работы: polling (опрос задач) или push")
}

func runWorker(cmd *cobra.Command, args []string) {
	log.Println("Запуск в режиме WORKER (исполнитель задач)")

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()
	logger.Init(cfg.Log.Level, true)
	logger.Get().Info().Msg("Starting SSH-SYNC-AUTOMATION in WORKER mode")

	if workerID == "" {
		workerID = generateWorkerID()
		logger.Get().Info().Str("generated_id", workerID).Msg("Auto-generated worker ID")
	}

	logger.Get().Info().
		Str("worker_id", workerID).
		Str("mode", workerMode).
		Msg("Worker started")

	// TODO: Релиз 2 — здесь будет:
	// - Подключение к backend через RPC клиент
	// - Heartbeat отправка
	// - Polling задач или long-polling
	// - Выполнение задач синхронизации
	// - Отправка прогресса через RPC

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go simulateWorkerWork(ctx, workerID)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	cancel()
	logger.Get().Info().Msg("Worker stopped")
}

func generateWorkerID() string {
	return "worker-" + time.Now().Format("20060102150405")
}

func simulateWorkerWork(ctx context.Context, id string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Get().Debug().Str("worker_id", id).Msg("Worker stopping...")
			return
		case <-ticker.C:
			logger.Get().Debug().
				Str("worker_id", id).
				Msg("Worker heartbeat (waiting for tasks from backend)")
			// TODO: Релиз 2 — запрос задачи через RPC
		}
	}
}
