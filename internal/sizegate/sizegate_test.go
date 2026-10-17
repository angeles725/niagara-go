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
			if path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "bin") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
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
