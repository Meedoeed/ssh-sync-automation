package handler

import (
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterRoutes(e *echo.Echo, healthHandler *HealthHandler) {
	e.GET("/live", healthHandler.Liveness)
	e.GET("/healthz", healthHandler.Readiness)
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
}
