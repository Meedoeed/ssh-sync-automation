package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
)

func main() {
	cfg := config.Load()
	logger.Init(cfg.Log.Level, true) // в разработке true, в проде false
	logger.Get().Info().Msg("Starting SSH-SYNC-AUTOMATION service")
	// Запуск http=сервера

	// TODO http-server

	// сразу сделал задел на graceful shutdown сервиса - пока что _=ctx - заглушка
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = ctx

	logger.Get().Info().Msg("Service stopped")
}
