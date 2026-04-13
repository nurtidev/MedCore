package logger

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
)

type ctxKey struct{}

// New создаёт zerolog логгер с заданным уровнем и форматом.
func New(level, format string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var base zerolog.Logger
	if format == "console" {
		base = zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		base = zerolog.New(os.Stdout)
	}

	return base.
		Level(lvl).
		With().
		Timestamp().
		Logger()
}

// WithContext кладёт логгер в context.
func WithContext(ctx context.Context, log zerolog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}

// FromContext достаёт логгер из context.
// Если логгера нет — возвращает nop логгер.
func FromContext(ctx context.Context) zerolog.Logger {
	if log, ok := ctx.Value(ctxKey{}).(zerolog.Logger); ok {
		return log
	}
	return zerolog.Nop()
}

// WithRequestID добавляет correlation_id в логгер и кладёт обратно в context.
func WithRequestID(ctx context.Context, requestID string) (context.Context, zerolog.Logger) {
	log := FromContext(ctx).With().Str("request_id", requestID).Logger()
	return WithContext(ctx, log), log
}
