package node

import (
	"context"
	"log/slog"
	"os"
)

type Logger interface {
	Info(ctx context.Context, msg string, fields []LoggerField)
	Warn(ctx context.Context, msg string, fields []LoggerField)
	Err(ctx context.Context, err error, fields []LoggerField)
}

type LoggerField struct {
	Key   string
	Value any
}

type logDefault struct {
	logger *slog.Logger
}

func NewLogger(handler slog.Handler) Logger {
	if handler == nil {
		opts := &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return &logDefault{
		logger: slog.New(handler),
	}
}

func (l *logDefault) Info(ctx context.Context, msg string, fields []LoggerField) {
	l.logger.InfoContext(ctx, msg, toSlogAttr(fields)...)
}

func (l *logDefault) Warn(ctx context.Context, msg string, fields []LoggerField) {
	l.logger.WarnContext(ctx, msg, toSlogAttr(fields)...)
}

func (l *logDefault) Err(ctx context.Context, err error, fields []LoggerField) {
	l.logger.ErrorContext(ctx, err.Error(), toSlogAttr(fields)...)
}

func toSlogAttr(fields []LoggerField) (a []any) {
	if fields == nil {
		return
	}

	a = make([]any, 0, len(fields)*2)
	for _, item := range fields {
		a = append(a, slog.Attr{
			Key:   item.Key,
			Value: slog.AnyValue(item.Value),
		})
	}
	return
}
