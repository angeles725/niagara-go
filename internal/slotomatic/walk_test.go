package slotomatic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWalkSortedPrunesAndSkipsDirSymlinks(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"b/x", "a", ".hidden"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	os.WriteFile(filepath.Join(root, "b", "x", "F.java"), nil, 0o644)
	os.WriteFile(filepath.Join(root, "z.txt"), nil, 0o644)
	os.Symlink(filepath.Join(root, "b"), filepath.Join(root, "link"))
	var seen []string
	Walk(root+string(filepath.Separator), true, func(dir string, dirs *[]string, files []string) {
		seen = append(seen, strings.TrimPrefix(dir, root)+"|"+strings.Join(*dirs, ",")+"|"+strings.Join(files, ","))
		kept := (*dirs)[:0]
		for _, d := range *dirs {
			if !strings.HasPrefix(d, ".") {
				kept = append(kept, d)
			}
		}
		*dirs = kept
	})
	sep := string(filepath.Separator)
	want := []string{sep + "|.hidden,a,b,link|z.txt", sep + "a||", sep + "b|x|", sep + "b" + sep + "x||F.java"}
	if strings.Join(seen, "\n") != strings.Join(want, "\n") {
		t.Errorf("walk:\n%s\nwant:\n%s", strings.Join(seen, "\n"), strings.Join(want, "\n"))
	}
}

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
	os.Unsetenv("NIAGARA_ORGANIZED")
	repo := filepath.Join(dir, "repo")
	deep := filepath.Join(repo, "clients", "x")
	for _, d := range []string{filepath.Join(repo, ".git"), filepath.Join(repo, "organized"), deep, filepath.Join(dir, "organized")} {
		os.MkdirAll(d, 0o755)
	}
	t.Chdir(deep)
	if got := OrganizedRoot(); got != filepath.Join(repo, "organized") {
		t.Errorf("checkout discovery -> %q", got)
	}
}
