// internal/server/http.go
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

	// Middleware
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

// Start запускает HTTP сервер
func (s *HTTPServer) Start() error {
	addr := fmt.Sprintf(":%s", s.config.Server.Port)
	logger.Get().Info().Msgf("Starting HTTP server on %s", addr)
	return s.echo.Start(addr)
}

// Shutdown graceful shutdown HTTP сервера
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	logger.Get().Info().Msg("Shutting down HTTP server...")
	return s.echo.Shutdown(ctx)
}

// SetValidator устанавливает валидатор для Echo
func (s *HTTPServer) SetValidator(v echo.Validator) {
	s.echo.Validator = v
}

// GetEcho возвращает экземпляр Echo (для регистрации роутов)
func (s *HTTPServer) GetEcho() *echo.Echo {
	return s.echo
}

// GetServerService возвращает сервис серверов
func (s *HTTPServer) GetServerService() *service.ServerService {
	return s.serverService
}

// GetTaskService возвращает сервис задач
func (s *HTTPServer) GetTaskService() *service.TaskService {
	return s.taskService
}

// GetSyncService возвращает сервис синхронизации
func (s *HTTPServer) GetSyncService() *service.SyncService {
	return s.syncService
}

// GetDB возвращает подключение к БД
func (s *HTTPServer) GetDB() *postgres.DB {
	return s.db
}

// GetEncryptor возвращает шифровальщик
func (s *HTTPServer) GetEncryptor() *encryption.Encryptor {
	return s.encryptor
}
