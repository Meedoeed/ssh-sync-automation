package middleware

import (
	"time"

	"github.com/Meedoeed/ssh-sync-automation/internal/infrastructure/logger"
	"github.com/labstack/echo/v4"
)

func EchoLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			logger.Get().Info().
				Str("method", c.Request().Method).
				Str("uri", c.Request().RequestURI).
				Int("status", c.Response().Status).
				Dur("latency", latency).
				Str("ip", c.RealIP()).
				Str("request_id", c.Response().Header().Get(echo.HeaderXRequestID)).
				Msg("HTTP request")

			return err
		}
	}
}
