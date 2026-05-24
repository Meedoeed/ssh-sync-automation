package handler

import (
	"net/http"
	"strings"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ServerHandler struct {
	serverService *service.ServerService
}

func NewServerHandler(serverService *service.ServerService) *ServerHandler {
	return &ServerHandler{
		serverService: serverService,
	}
}

type CreateServerRequest struct {
	Name       string  `json:"name"`
	Host       string  `json:"host"`
	Port       int     `json:"port"`
	Username   string  `json:"username"`
	AuthType   string  `json:"auth_type"`
	Password   *string `json:"password,omitempty"`
	PrivateKey *string `json:"private_key,omitempty"`
	IsActive   bool    `json:"is_active"`
}

type ServerResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Host      string  `json:"host"`
	Port      int     `json:"port"`
	Username  string  `json:"username"`
	AuthType  string  `json:"auth_type"`
	IsActive  bool    `json:"is_active"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
	LastSeen  *string `json:"last_seen,omitempty"`
}

type UpdateServerRequest struct {
	Name       string  `json:"name,omitempty"`
	Host       string  `json:"host,omitempty"`
	Port       int     `json:"port,omitempty"`
	Username   string  `json:"username,omitempty"`
	AuthType   string  `json:"auth_type,omitempty"`
	Password   *string `json:"password,omitempty"`
	PrivateKey *string `json:"private_key,omitempty"`
	IsActive   *bool   `json:"is_active,omitempty"`
}

func (h *ServerHandler) CreateServer(c echo.Context) error {
	var req CreateServerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
		})
	}

	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "name is required",
		})
	}
	if req.Host == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "host is required",
		})
	}
	if req.Port < 1 || req.Port > 65535 {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "port must be between 1 and 65535",
		})
	}
	if req.Username == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "username is required",
		})
	}
	if req.AuthType != "password" && req.AuthType != "key" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "auth_type must be 'password' or 'key'",
		})
	}
	if req.AuthType == "password" && (req.Password == nil || strings.TrimSpace(*req.Password) == "") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "password is required for password authentication",
		})
	}
	if req.AuthType == "key" && (req.PrivateKey == nil || strings.TrimSpace(*req.PrivateKey) == "") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "private_key is required for key authentication",
		})
	}

	server := &domain.Server{
		Name:       req.Name,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		AuthType:   req.AuthType,
		Password:   req.Password,
		PrivateKey: req.PrivateKey,
		IsActive:   req.IsActive,
	}

	if err := h.serverService.CreateServer(c.Request().Context(), server); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create server: " + err.Error(),
		})
	}

	logger.Get().Info().
		Str("server_name", server.Name).
		Msg("Server created successfully")

	response := ServerResponse{
		ID:        server.ID.String(),
		Name:      server.Name,
		Host:      server.Host,
		Port:      server.Port,
		Username:  server.Username,
		AuthType:  server.AuthType,
		IsActive:  server.IsActive,
		CreatedAt: server.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: server.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if server.LastSeen != nil {
		lastSeen := server.LastSeen.Format("2006-01-02T15:04:05Z07:00")
		response.LastSeen = &lastSeen
	}

	return c.JSON(http.StatusCreated, response)
}

func (h *ServerHandler) ListServers(c echo.Context) error {
	ctx := c.Request().Context()
	activeOnly := c.QueryParam("active_only") == "true"

	servers, err := h.serverService.ListServers(ctx, activeOnly)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list servers: " + err.Error(),
		})
	}

	response := make([]ServerResponse, 0, len(servers))
	for _, server := range servers {
		resp := ServerResponse{
			ID:        server.ID.String(),
			Name:      server.Name,
			Host:      server.Host,
			Port:      server.Port,
			Username:  server.Username,
			AuthType:  server.AuthType,
			IsActive:  server.IsActive,
			CreatedAt: server.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: server.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if server.LastSeen != nil {
			lastSeen := server.LastSeen.Format("2006-01-02T15:04:05Z07:00")
			resp.LastSeen = &lastSeen
		}
		response = append(response, resp)
	}

	return c.JSON(http.StatusOK, response)
}

func (h *ServerHandler) GetServer(c echo.Context) error {
	id := c.Param("id")
	serverID, err := uuid.Parse(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid server id",
		})
	}

	server, err := h.serverService.GetServer(c.Request().Context(), serverID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "server not found",
		})
	}

	response := ServerResponse{
		ID:        server.ID.String(),
		Name:      server.Name,
		Host:      server.Host,
		Port:      server.Port,
		Username:  server.Username,
		AuthType:  server.AuthType,
		IsActive:  server.IsActive,
		CreatedAt: server.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: server.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if server.LastSeen != nil {
		lastSeen := server.LastSeen.Format("2006-01-02T15:04:05Z07:00")
		response.LastSeen = &lastSeen
	}

	return c.JSON(http.StatusOK, response)
}

func (h *ServerHandler) DeleteServer(c echo.Context) error {
	id := c.Param("id")
	serverID, err := uuid.Parse(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid server id",
		})
	}

	ctx := c.Request().Context()

	server, err := h.serverService.GetServer(ctx, serverID)
	if err != nil {
		logger.Get().Warn().
			Err(err).
			Str("server_id", id).
			Msg("Server not found for deletion")
	} else if server == nil {
		logger.Get().Warn().
			Str("server_id", id).
			Msg("Server is nil")
	}

	// Удаляем сервер из БД
	if err := h.serverService.DeleteServer(ctx, serverID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to delete server: " + err.Error(),
		})
	}

	serverName := ""
	if server != nil {
		serverName = server.Name
	}

	logger.Get().Info().
		Str("server_id", id).
		Str("server_name", serverName).
		Msg("Server deleted successfully")

	return c.JSON(http.StatusOK, map[string]string{
		"status":      "deleted",
		"server_id":   id,
		"server_name": serverName,
	})
}

func (h *ServerHandler) UpdateServer(c echo.Context) error {
	id := c.Param("id")
	serverID, err := uuid.Parse(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid server id",
		})
	}

	ctx := c.Request().Context()

	existingServer, err := h.serverService.GetServer(ctx, serverID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "server not found",
		})
	}
	if existingServer == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "server not found",
		})
	}

	var req UpdateServerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body: " + err.Error(),
		})
	}

	if req.Name != "" {
		existingServer.Name = req.Name
	}
	if req.Host != "" {
		existingServer.Host = req.Host
	}
	if req.Port != 0 && req.Port >= 1 && req.Port <= 65535 {
		existingServer.Port = req.Port
	}
	if req.Username != "" {
		existingServer.Username = req.Username
	}
	if req.AuthType != "" {
		if req.AuthType != "password" && req.AuthType != "key" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "auth_type must be 'password' or 'key'",
			})
		}
		existingServer.AuthType = req.AuthType
	}
	if req.Password != nil {
		existingServer.Password = req.Password
	}
	if req.PrivateKey != nil {
		existingServer.PrivateKey = req.PrivateKey
	}
	if req.IsActive != nil {
		existingServer.IsActive = *req.IsActive
	}

	if existingServer.AuthType == "password" && (existingServer.Password == nil || strings.TrimSpace(*existingServer.Password) == "") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "password is required for password authentication",
		})
	}
	if existingServer.AuthType == "key" && (existingServer.PrivateKey == nil || strings.TrimSpace(*existingServer.PrivateKey) == "") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "private_key is required for key authentication",
		})
	}

	if err := h.serverService.UpdateServer(ctx, existingServer); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to update server: " + err.Error(),
		})
	}

	logger.Get().Info().
		Str("server_id", serverID.String()).
		Str("server_name", existingServer.Name).
		Msg("Server updated successfully")

	response := ServerResponse{
		ID:        existingServer.ID.String(),
		Name:      existingServer.Name,
		Host:      existingServer.Host,
		Port:      existingServer.Port,
		Username:  existingServer.Username,
		AuthType:  existingServer.AuthType,
		IsActive:  existingServer.IsActive,
		CreatedAt: existingServer.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: existingServer.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if existingServer.LastSeen != nil {
		lastSeen := existingServer.LastSeen.Format("2006-01-02T15:04:05Z07:00")
		response.LastSeen = &lastSeen
	}

	return c.JSON(http.StatusOK, response)
}
