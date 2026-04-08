package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Liveness(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "alive"})
}

func (h *HealthHandler) Readiness(c echo.Context) error {
	// TODO после реализации БД
	// проверять статус БД
	return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
}
