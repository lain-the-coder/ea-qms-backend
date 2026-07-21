package logging

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
)

// ── The fan-out handler ────────────────────────────────────────────
// slog has no built-in "send to two places" handler, so we make one.
// It holds a list of child handlers and forwards every call to each.
type multiHandler struct {
	handlers []slog.Handler
}

// Enabled: log this level if ANY child would.
func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle: hand the record to EVERY child (this is the fan-out).
func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(ctx, r.Clone()); err != nil {
			return err
		}
	}
	return nil
}

// WithAttrs: return a new multiHandler whose children each carry the attrs.
func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: next}
}

// WithGroup: same idea for groups.
func (m *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: next}
}

// ── The constructor ────────────────────────────────────────────────
func NewLogger(logDir string) (*slog.Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(
		filepath.Join(logDir, "app.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, err
	}

	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	textHandler := slog.NewTextHandler(os.Stdout, opts) // console, readable
	jsonHandler := slog.NewJSONHandler(file, opts)      // file, structured

	multi := &multiHandler{handlers: []slog.Handler{textHandler, jsonHandler}}

	return slog.New(multi), nil
}

type contextKey string

const loggerKey contextKey = "logger"

// setter
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	ctx = context.WithValue(ctx, loggerKey, logger)
	return ctx
}

// getter
func LoggerFrom(ctx context.Context) *slog.Logger {
	value := ctx.Value(loggerKey)
	logger, ok := value.(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return logger
}
