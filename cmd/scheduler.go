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

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "Запуск планировщика задач",
	Long:  "Запускает планировщик для создания задач синхронизации",
	Run:   runScheduler,
}

func runScheduler(cmd *cobra.Command, args []string) {
	log.Println("Запуск в режиме SCHEDULER (планировщик задач)")

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	cfg := config.Load()
	logger.Init(cfg.Log.Level, true)
	logger.Get().Info().Msg("Starting SSH-SYNC-AUTOMATION in SCHEDULER mode")

	logger.Get().Info().
		Dur("interval", cfg.Sync.Interval).
		Msg("Scheduler started with sync interval")

	// TODO: Релиз 3 — здесь будет:
	// - Подключение к backend через RPC клиент
	// - Периодический опрос активных серверов через backend API
	// - Создание задач через RPC вызовы
	// - Логика предотвращения дублирования задач

	logger.Get().Info().
		Msg("Scheduler running. Press Ctrl+C to stop.")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Get().Info().Msg("Scheduler stopped")
}

// simulateSchedulerWork — временная заглушка для демонстрации работы
func simulateSchedulerWork(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logger.Get().Debug().Msg("Scheduler tick: checking for new tasks")
			// TODO: Релиз 3 — реальная логика
		}
	}
}
