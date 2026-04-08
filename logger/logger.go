package logger

import (
	"io"
	"log/slog"
	"os"
)

type Logger struct {
	Log *slog.Logger
}

func NewLogger(w io.Writer, verbose bool) *Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	return &Logger{Log: slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))}
}

// NewDefaultLogger creates a Logger that writes to stderr at info level.
func NewDefaultLogger() *Logger {
	return NewLogger(os.Stderr, false)
}

func (l *Logger) Info(msg string, args ...any) {
	l.Log.Info(msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.Log.Warn(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.Log.Error(msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.Log.Debug(msg, args...)
}
