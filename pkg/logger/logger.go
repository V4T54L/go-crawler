package logger

import (
	"io"
	"log/slog"
)

// Init initializes the global slog logger.
func Init(writer io.Writer, level slog.Level) {
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize attribute keys for consistency if needed
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			if a.Key == slog.LevelKey {
				a.Key = "level"
			}
			if a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

