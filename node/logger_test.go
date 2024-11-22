package node

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_logger(t *testing.T) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	l := NewLogger(slog.NewTextHandler(w, nil))
	ctx := context.Background()
	l.Info(ctx, "test info", []LoggerField{{Key: "int", Value: 1}, {Key: "str", Value: "str"}})
	l.Info(ctx, "test info", nil)
	l.Warn(ctx, "test warn", []LoggerField{{"struct", struct{ Name string }{Name: "test"}}})
	l.Warn(ctx, "test warn", nil)
	l.Err(ctx, errors.New("test err"), []LoggerField{{"int64", int64(12)}, {"bool", true}})
	l.Err(ctx, errors.New("test err"), nil)
	w.Flush()

	assert.Contains(t, b.String(), `level=INFO msg="test info" int=1 str=str`)
	assert.Contains(t, b.String(), `level=INFO msg="test info"`)
	assert.Contains(t, b.String(), `level=WARN msg="test warn" struct={Name:test}`)
	assert.Contains(t, b.String(), `level=WARN msg="test warn"`)
	assert.Contains(t, b.String(), `level=ERROR msg="test err" int64=12 bool=true`)
	assert.Contains(t, b.String(), `level=ERROR msg="test err"`)
}
