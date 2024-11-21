package node

import (
	"path/filepath"
	"testing"
)

func Test_absPath(t *testing.T) {
	abs, _ := filepath.Abs("./test.txt")

	tests := []struct {
		path string
		want string
	}{
		{
			path: "./test.txt",
			want: abs,
		},
		{
			path: "/test/test.txt",
			want: "/test/test.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := absPath(tt.path); got != tt.want {
				t.Errorf("absPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
