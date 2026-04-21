package handler

import (
	"net/http"

	"github.com/Meedoeed/ssh-sync-automation/internal/domain"
	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/Meedoeed/ssh-sync-automation/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type TaskHandler struct {
	taskService *service.TaskService
}

func NewTaskHandler(taskService *service.TaskService) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
	}
}

type TaskResponse struct {
	ID               string  `json:"id"`
	ServerID         string  `json:"server_id"`
	ServerName       string  `json:"server_name,omitempty"`
	Direction        string  `json:"direction"`
	FileName         string  `json:"file_name"`
	RemotePath       string  `json:"remote_path"`
	LocalPath        string  `json:"local_path"`
	FileSize         int64   `json:"file_size"`
	BytesTransferred int64   `json:"bytes_transferred"`
	Progress         float64 `json:"progress"`
	Status           string  `json:"status"`
	AttemptCount     int     `json:"attempt_count"`
	MaxAttempts      int     `json:"max_attempts"`
	ErrorMessage     *string `json:"error_message,omitempty"`
	StartedAt        *string `json:"started_at,omitempty"`
	CompletedAt      *string `json:"completed_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

func (h *TaskHandler) ListTasks(c echo.Context) error {
	ctx := c.Request().Context()

	serverID := c.QueryParam("server_id")
	status := c.QueryParam("status")
	limit := 100

	var tasks []*domain.SyncTask
	var err error

	if serverID != "" {
		id, err := uuid.Parse(serverID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid server id",
			})
		}

		var statusPtr *domain.SyncStatus
		if status != "" {
			s := domain.SyncStatus(status)
			statusPtr = &s
		}

		tasks, err = h.taskService.ListServerTasks(ctx, id, statusPtr, limit)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to list server tasks")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to list tasks: " + err.Error(),
			})
		}
	} else {
		tasks, err = h.taskService.GetAllTasks(ctx, limit)
		if err != nil {
			logger.Get().Error().Err(err).Msg("Failed to list recent tasks")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to list tasks: " + err.Error(),
			})
		}
	}

	response := make([]TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		resp := TaskResponse{
			ID:               task.ID.String(),
			ServerID:         task.ServerID.String(),
			Direction:        string(task.Direction),
			FileName:         task.FileName,
			RemotePath:       task.RemotePath,
			LocalPath:        task.LocalPath,
			FileSize:         task.FileSize,
			BytesTransferred: task.BytesTransferred,
			Progress:         task.Progress(),
			Status:           string(task.Status),
			AttemptCount:     task.AttemptCount,
			MaxAttempts:      task.MaxAttempts,
			ErrorMessage:     task.ErrorMessage,
			CreatedAt:        task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:        task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		if task.StartedAt != nil {
			started := task.StartedAt.Format("2006-01-02T15:04:05Z07:00")
			resp.StartedAt = &started
		}
		if task.CompletedAt != nil {
			completed := task.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
			resp.CompletedAt = &completed
		}

		response = append(response, resp)
	}

	return c.JSON(http.StatusOK, response)
}

func (h *TaskHandler) GetTask(c echo.Context) error {
	id := c.Param("id")
	taskID, err := uuid.Parse(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid task id",
		})
	}

	ctx := c.Request().Context()
	task, err := h.taskService.GetTask(ctx, taskID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "task not found: " + err.Error(),
		})
	}
	if task == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "task not found",
		})
	}

	response := TaskResponse{
		ID:               task.ID.String(),
		ServerID:         task.ServerID.String(),
		Direction:        string(task.Direction),
		FileName:         task.FileName,
		RemotePath:       task.RemotePath,
		LocalPath:        task.LocalPath,
		FileSize:         task.FileSize,
		BytesTransferred: task.BytesTransferred,
		Progress:         task.Progress(),
		Status:           string(task.Status),
		AttemptCount:     task.AttemptCount,
		MaxAttempts:      task.MaxAttempts,
		ErrorMessage:     task.ErrorMessage,
		CreatedAt:        task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if task.StartedAt != nil {
		started := task.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		response.StartedAt = &started
	}
	if task.CompletedAt != nil {
		completed := task.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		response.CompletedAt = &completed
	}

	return c.JSON(http.StatusOK, response)
}
