package testkit

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func ReadText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func WriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func NoDotEntries(t *testing.T, root string) {
	t.Helper()
	for _, n := range listDir(t, root) {
		if strings.HasPrefix(n, ".") {
			t.Fatalf("stray temp entry %s left in %s", n, root)
		}
	}
}

var generatedUTC = regexp.MustCompile(`"generated_utc": "([^"]*)"`)

var isoSeconds = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\+00:00$`)

// TreeLines lists every path under root (sorted, root itself excluded):
// kind, permission bits, then a file's size and sha256, or a manifest's
// text with generated_utc normalized.
func TreeLines(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		line := fmt.Sprintf("%s %s %o", filepath.ToSlash(rel), info.Mode().Type(), info.Mode().Perm())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			if strings.HasSuffix(p, ".manifest.json") {
				m := generatedUTC.FindSubmatch(data)
				if m == nil || !isoSeconds.Match(m[1]) {
					t.Errorf("%s: generated_utc missing or not isoformat(timespec=seconds)", rel)
				}
				line += " " + string(generatedUTC.ReplaceAll(data, []byte(`"generated_utc": "<utc>"`)))
			} else {
				sum := sha256.Sum256(data)
				line += fmt.Sprintf(" %d %s", len(data), hex.EncodeToString(sum[:]))
			}
		}
		out = append(out, line)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	src, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	dst, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
}

// CopyTree copies the regular files under from into to.
func CopyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		copyFile(t, p, filepath.Join(to, rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// FileTexts maps every regular file under root to its content.
func FileTexts(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		data, err := os.ReadFile(p)
		rel, _ := filepath.Rel(root, p)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// WriteTree writes files (relative "/"-separated path -> content) under root.
func WriteTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		WriteFile(t, filepath.Join(root, filepath.FromSlash(rel)), []byte(text))
	}
}

// SymlinkOrSkip makes newname a symlink to oldname; on Windows without the
// symlink privilege the test is skipped.
func SymlinkOrSkip(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("no symlink privilege: %v", err)
		}
		t.Fatal(err)
	}
}
