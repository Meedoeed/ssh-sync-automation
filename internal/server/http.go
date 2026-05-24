package server

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/Meedoeed/ssh-sync-automation/internal/config"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/encryption"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	myMiddleware "github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/middleware"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/Meedoeed/ssh-sync-automation/internal/storage/postgres"
)

type HTTPServer struct {
	echo          *echo.Echo
	config        *config.Config
	db            *postgres.DB
	encryptor     *encryption.Encryptor
	serverService *service.ServerService
	taskService   *service.TaskService
	syncService   *service.SyncService
}

func NewHTTP(
	cfg *config.Config,
	db *postgres.DB,
	encryptor *encryption.Encryptor,
	serverService *service.ServerService,
	taskService *service.TaskService,
	syncService *service.SyncService,
) *HTTPServer {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(myMiddleware.EchoLogger())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	return &HTTPServer{
		echo:          e,
		config:        cfg,
		db:            db,
		encryptor:     encryptor,
		serverService: serverService,
		taskService:   taskService,
		syncService:   syncService,
	}
}

func (s *HTTPServer) Start() error {
	addr := fmt.Sprintf(":%s", s.config.Server.Port)
	logger.Get().Info().Msgf("Starting HTTP server on %s", addr)
	return s.echo.Start(addr)
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	logger.Get().Info().Msg("Shutting down HTTP server...")
	return s.echo.Shutdown(ctx)
}

func (s *HTTPServer) SetValidator(v echo.Validator) {
	s.echo.Validator = v
}

func (s *HTTPServer) GetEcho() *echo.Echo {
	return s.echo
}
