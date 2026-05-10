package logger

import (
	"log/slog"
	"os"
)

// Init initialises the global slog logger.
// Set LOG_FORMAT=json for JSON output (e.g. production / log aggregators).
// Default is human-readable text format.
func Init() {
	format := os.Getenv("LOG_FORMAT")
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}
