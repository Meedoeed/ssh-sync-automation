package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/server"
)

func main() {
	cfg := config.Load()
	logger.Init(cfg.Log.Level, true) // в разработке true, в проде false
	logger.Get().Info().Msg("Starting SSH-SYNC-AUTOMATION service")

	httpServer := server.NewHTTP(cfg)

	go func() {
		if err := httpServer.Start(); err != nil {
			logger.Get().Fatal().Err(err).Msg("Failed to start HTTP server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Get().Error().Err(err).Msg("HTTP server shutdown error")
	}

	logger.Get().Info().Msg("Service stopped")
}
