package sizegate

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const maxLines = 500

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

func loadAllowlist(t *testing.T, root string) map[string]int {
	t.Helper()
	allowed := map[string]int{}
	data, err := os.ReadFile(filepath.Join(root, "internal/sizegate/allowlist.txt"))
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var path string
		var limit int
		if _, err := fmt.Sscan(line, &path, &limit); err != nil {
			t.Fatalf("allowlist line %q: want '<path> <max-lines>'", line)
		}
		allowed[path] = limit
	}
	return allowed
}

// gated reports whether the limit applies to the repo-relative path rel: every
// .go file outside testdata/ (Go files there are fixtures, not source) and
// every .py file anywhere, testdata/ included, because the remaining Python test
// helpers live there.
func gated(rel string) bool {
	switch {
	case strings.HasSuffix(rel, ".py"):
		return true
	case strings.HasSuffix(rel, ".go"):
		return !strings.HasPrefix(rel, "testdata/") && !strings.Contains(rel, "/testdata/")
	}
	return false
}

func TestGatedPaths(t *testing.T) {
	for rel, want := range map[string]bool{
		"main.go":                                 true,
		"internal/index/parse.go":                 true,
		"internal/golden/testdata/toy/main.go":    false,
		"testdata/x.go":                           false,
		"internal/nav/testdata/gen_golden.py":     true,
		"internal/helpbuild/testdata/parity/a.py": true,
		"tools/x.py":                              true,
		"internal/nav/testdata/golden/case/out":   false,
		"README.md":                               false,
	} {
		if got := gated(rel); got != want {
			t.Errorf("gated(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestSourceFilesStayUnderLimit(t *testing.T) {
	root := repoRoot(t)
	allowed := loadAllowlist(t, root)
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "bin") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if !gated(rel) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		n := bytes.Count(data, []byte("\n"))
		limit := maxLines
		if l, ok := allowed[rel]; ok {
			seen[rel] = true
			limit = l
			if n < l {
				t.Errorf("%s: %d lines, allowlist says %d: lower the allowlist entry", rel, n, l)
			}
		}
		if n > limit {
			t.Errorf("%s: %d lines > %d", rel, n, limit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path := range allowed {
		if !seen[path] {
			t.Errorf("allowlist entry %s matches no file: remove it", path)
		}
	}
}
