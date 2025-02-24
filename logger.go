package dive

import (
	"context"
	"log/slog"
)

// Logger defines the interface for logging within agents.
// It supports structured logging and is designed to be compatible
// with common logging libraries like zerolog and slog.
type Logger interface {
	// Debug logs a message at debug level with optional key-value pairs
	Debug(msg string, keysAndValues ...any)

	// Info logs a message at info level with optional key-value pairs
	Info(msg string, keysAndValues ...any)

	// Warn logs a message at warn level with optional key-value pairs
	Warn(msg string, keysAndValues ...any)

	// Error logs a message at error level with optional key-value pairs
	Error(msg string, keysAndValues ...any)

	// With returns a new Logger instance with the given key-value pairs added to the context
	With(keysAndValues ...any) Logger
}

type contextKey string

const (
	loggerKey contextKey = "agents.logger"
)

func WithLogger(ctx context.Context, logger Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey, logger)
}

func LoggerFromCtx(ctx context.Context) Logger {
	if ctx == nil {
		return NewSlogLogger(slog.Default())
	}
	logger, ok := ctx.Value(loggerKey).(Logger)
	if !ok {
		return NewSlogLogger(slog.Default())
	}
	return logger
}
