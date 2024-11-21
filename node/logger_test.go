package node

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

func Test_logger(t *testing.T) {
	l := NewLogger(slog.LevelDebug)
	ctx := context.Background()
	l.Info(ctx, "test info", map[string]any{"int": 1, "str": "str", "bool": true})
	l.Info(ctx, "test info", nil)
	l.Warn(ctx, "test warn", map[string]any{"int": 1, "str": "str", "bool": true})
	l.Warn(ctx, "test warn", nil)
	l.Err(ctx, errors.New("test err"), map[string]any{"int": 1, "str": "str", "bool": true})
	l.Err(ctx, errors.New("test err"), nil)
}
