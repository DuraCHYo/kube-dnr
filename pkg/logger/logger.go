package logger

import (
	"context"
	"io"
	"log/slog"
)

// ctxKey - типизированный ключ для передачи логгера через context
// (строковые ключи контекста могут конфликтовать между пакетами)
type ctxKey struct{}

// NewContext сохраняет логгер в контексте
func NewContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext достает логгер из контекста.
// Если логгер не был установлен, возвращает slog.Default(),
// чтобы вызовы логирования не приводили к панике.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// New создает JSON-логгер с уровнем, зависящим от флага --verbose:
// Debug при verbose, иначе Info.
func New(w io.Writer, verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
