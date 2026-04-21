package handler

import (
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterRoutes(
	e *echo.Echo,
	healthHandler *HealthHandler,
	serverHandler *ServerHandler,
	taskHandler *TaskHandler,
) {
	e.GET("/live", healthHandler.Liveness)
	e.GET("/healthz", healthHandler.Readiness)

	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	v1 := e.Group("/api/v1")

	servers := v1.Group("/servers")
	servers.POST("", serverHandler.CreateServer)
	servers.GET("", serverHandler.ListServers)
	servers.GET("/:id", serverHandler.GetServer)
	servers.DELETE("/:id", serverHandler.DeleteServer)
	servers.PUT("/:id", serverHandler.UpdateServer)

	tasks := v1.Group("/tasks")
	tasks.GET("", taskHandler.ListTasks)
	tasks.GET("/:id", taskHandler.GetTask)

	v1.GET("/workers/stats", serverHandler.GetWorkerStats)
}
