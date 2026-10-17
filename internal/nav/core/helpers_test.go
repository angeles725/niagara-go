package core

import (
	"os"
	"testing"
	"time"

	"github.com/angeles725/niagara-go/internal/index"
)

func NewTestMeta(t *testing.T, text string) *index.Object {
	t.Helper()
	v, err := index.Decode([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return v.(*index.Object)
}

func mustMtime(t *testing.T, p string) float64 {
	t.Helper()
	m, ok := mtime(p)
	if !ok {
		t.Fatalf("no mtime for %s", p)
	}
	return m
}

func setMtime(t *testing.T, p string, secs float64) {
	t.Helper()
	tm := time.Unix(0, int64(secs*1e9))
	if err := os.Chtimes(p, tm, tm); err != nil {
		t.Fatal(err)
	}
}
