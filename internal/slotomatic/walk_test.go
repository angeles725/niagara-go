package slotomatic

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrganizedRoot(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"", " OFF ", "0"} {
		t.Setenv("NIAGARA_ORGANIZED", v)
		if got := OrganizedRoot(); got != "" {
			t.Errorf("%q -> %q", v, got)
		}
	}
	t.Setenv("NIAGARA_ORGANIZED", filepath.Join(dir, "missing"))
	if got := OrganizedRoot(); got != "" {
		t.Errorf("missing dir -> %q", got)
	}
	t.Setenv("NIAGARA_ORGANIZED", dir)
	if got := OrganizedRoot(); got != dir {
		t.Errorf("env dir -> %q", got)
	}
	// Python strips only for the off test; the path itself is used as given (" dir " is no dir).
	t.Setenv("NIAGARA_ORGANIZED", " "+dir+" ")
	if got := OrganizedRoot(); got != "" {
		t.Errorf("padded dir -> %q, want none (os.path.isdir of the untrimmed value)", got)
	}
	os.Unsetenv("NIAGARA_ORGANIZED")
	repo := filepath.Join(dir, "repo")
	deep := filepath.Join(repo, "clients", "x")
	for _, d := range []string{filepath.Join(repo, ".git"), filepath.Join(repo, "organized"), deep, filepath.Join(dir, "organized")} {
		must(t, os.MkdirAll(d, 0o755))
	}
	t.Chdir(deep)
	if got := OrganizedRoot(); got != filepath.Join(repo, "organized") {
		t.Errorf("checkout discovery -> %q", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Join is os.path.join: a rooted name restarts the path instead of being glued to dir.
func TestJoinIsHostPathJoin(t *testing.T) {
	for _, c := range []struct{ dir, name, want string }{
		{"a", "/b", "/b"},
		{"", "b", "b"},
		{"a/", "b", "a/b"},
		{"a", "b", "a" + string(filepath.Separator) + "b"},
	} {
		if got := Join(c.dir, c.name); got != c.want {
			t.Errorf("Join(%q, %q) = %q, want %q", c.dir, c.name, got, c.want)
		}
	}
}
