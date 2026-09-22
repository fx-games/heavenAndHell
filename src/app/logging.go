package app

import (
	"log/slog"
	"os"
)

// setupLogging routes every log line (slog, and the log package through it) to stderr at HH_LOG's level:
// debug, info (the default), warn or error.
func setupLogging() {
	var level slog.Level
	_ = level.UnmarshalText([]byte(os.Getenv("HH_LOG"))) // empty or unknown leaves info
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}
