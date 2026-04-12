package handler

import (
	"net/http"

	"github.com/Meedoeed/ssh-sync-automation/internal/storage/postgres"
	"github.com/labstack/echo/v4"
)

type HealthHandler struct {
	db *postgres.DB
}

func NewHealthHandler(db *postgres.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Liveness(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "alive"})
}

func (h *HealthHandler) Readiness(c echo.Context) error {
	if h.db == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"reason": "database not initialized",
		})
	}

	if err := h.db.Ping(c.Request().Context()); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
			"reason": "database unavailable: " + err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
}
