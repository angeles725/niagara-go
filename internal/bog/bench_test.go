package bog

import (
	"os"
	"testing"
)

// BenchmarkReadText and BenchmarkParse time one real bog named by BOG_BENCH_FILE (outside the
// repository): go test -run '^$' -bench . -benchmem ./internal/bog/
func benchFile(b *testing.B) string {
	path := os.Getenv("BOG_BENCH_FILE")
	if path == "" {
		b.Skip("BOG_BENCH_FILE not set")
	}
	return path
}

func BenchmarkReadText(b *testing.B) {
	path := benchFile(b)
	for b.Loop() {
		if _, err := ReadText(path); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	text, err := ReadText(benchFile(b))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		Parse("bench", text)
	}
}

func BenchmarkLoad(b *testing.B) {
	path := benchFile(b)
	for b.Loop() {
		if _, err := Load(path); err != nil {
			b.Fatal(err)
		}
	}
}
