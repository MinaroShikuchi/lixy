package middlewares

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var logger *slog.Logger

func GetLogger() *slog.Logger {
	return logger
}

// setupLogger configures structured logging
func SetupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger = slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

type LoggerHandlerFunc func(http.ResponseWriter, *http.Request, *slog.Logger)

// loggingMiddleware logs HTTP requests
func LoggingMiddleware(next LoggerHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Log request info
		logger.Info("Request started",
			"method", r.Method,
			"path", r.URL.Path,
			"remoteAddr", r.RemoteAddr,
			"userAgent", r.UserAgent())

		// Call the handler
		next(w, r, logger)

		// Log request completion
		logger.Info("Request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start))
	}
}
