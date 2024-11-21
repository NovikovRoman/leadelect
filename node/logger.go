package node

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

type Logger interface {
	Info(ctx context.Context, msg string, args map[string]any)
	Warn(ctx context.Context, msg string, args map[string]any)
	Err(ctx context.Context, err error, args map[string]any)
}

type logDefault struct {
	logger *slog.Logger
}

func NewLogger(level slog.Level) Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	return &logDefault{
		logger: slog.New(slog.NewTextHandler(os.Stdout, opts)),
	}
}

func (l *logDefault) Info(ctx context.Context, msg string, args map[string]any) {
	l.logger.InfoContext(ctx, msg, toSlogAttr(args)...)
}

func (l *logDefault) Warn(ctx context.Context, msg string, args map[string]any) {
	l.logger.WarnContext(ctx, msg, toSlogAttr(args)...)
}

func (l *logDefault) Err(ctx context.Context, err error, args map[string]any) {
	l.logger.ErrorContext(ctx, err.Error(), toSlogAttr(args)...)
}

func toSlogAttr(args map[string]any) (a []any) {
	if args == nil {
		return
	}

	a = make([]any, 0, len(args)*2)
	for k, v := range args {
		a = append(a, slog.Attr{
			Key:   k,
			Value: slog.StringValue(fmt.Sprintf("%+v", v)),
		})
	}
	return
}
