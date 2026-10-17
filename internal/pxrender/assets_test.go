package pxrender

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// touch creates every path (files, or directories when they end in "/") under root.
func touch(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, p)
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(p), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// module:// candidates come in Python's order: the station copy, the wb/rt/ux artifacts, then
// every other artifact in sorted order, never a hidden one; resolve takes the first that exists.
func TestModuleCandidateOrder(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	touch(t, root, "shared/px/m/i.png", "org/m/m-wb/extracted/i.png", "org/m/m-ux/extracted/i.png",
		"org/m/m-zz/extracted/i.png", "org/m/m-aa/extracted/i.png", "org/m/.hidden/extracted/i.png")
	for _, organized := range []string{"org", "./org/", "org//", "org/./"} {
		a := newAssets("shared", organized)
		var got []string
		for _, c := range a.moduleCandidates("m", "i.png") {
			got = append(got, filepath.Clean(c))
		}
		want := []string{"shared/px/m/i.png", "org/m/m-wb/extracted/i.png", "org/m/m-rt/extracted/i.png",
			"org/m/m-ux/extracted/i.png", "org/m/m-aa/extracted/i.png", "org/m/m-ux/extracted/i.png",
			"org/m/m-wb/extracted/i.png", "org/m/m-zz/extracted/i.png"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("organized %q: candidates\n got %q\nwant %q", organized, got, want)
		}
	}
	a := newAssets("shared", "org")
	if got := a.resolve("module://m/i.png"); got != "shared/px/m/i.png" {
		t.Errorf("resolve = %q, want the station copy first", got)
	}
	if err := os.Remove("shared/px/m/i.png"); err != nil {
		t.Fatal(err)
	}
	if got := a.resolve("module://m/i.png"); got != "org/m/m-wb/extracted/i.png" {
		t.Errorf("resolve = %q, want the wb artifact next", got)
	}
	if got := newAssets("shared", "").moduleCandidates("m", "i.png"); len(got) != 1 {
		t.Errorf("without --organized only the station copy is a candidate, got %q", got)
	}
}

func TestFindShared(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "st/shared/Imagenes/", "st/shared/px/a/b/c/view.px", "loose/px/view.px",
		"deep/Imagenes/", "deep/1/2/3/4/5/6/7/8/view.px", "near/Imagenes/", "near/1/2/3/4/5/6/7/view.px")
	cases := map[string]string{
		"st/shared/px/a/b/c/view.px":   "st/shared",          // an ancestor holds Imagenes/
		"loose/px/view.px":             "loose",              // none does: the parent of the .px directory
		"deep/1/2/3/4/5/6/7/8/view.px": "deep/1/2/3/4/5/6/7", // Imagenes/ is 9 levels up: not searched
		"near/1/2/3/4/5/6/7/view.px":   "near",               // 8 levels up: the last one searched
	}
	for px, want := range cases {
		if got := findShared(filepath.Join(root, px), ""); got != filepath.Join(root, want) {
			t.Errorf("findShared(%s) = %q, want %q", px, got, filepath.Join(root, want))
		}
	}
	if got := findShared("x.px", "given"); got != "given" {
		t.Errorf("an override wins, got %q", got)
	}
}

// A ref that resolves to a directory is reported and skipped instead of aborting the render
// (Python crashes with IsADirectoryError).
func TestDirectoryRefIsSkipped(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "shared/Imagenes/a.png")
	a := newAssets(filepath.Join(root, "shared"), "")
	uri, err := a.dataURI("file:^Imagenes")
	if err != nil || uri != "" {
		t.Fatalf("uri %q err %v", uri, err)
	}
	if uri, err = a.dataURI("file:^Imagenes/a.png"); err != nil || !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("a real image after the directory: uri %q err %v", uri, err)
	}
	if got := a.missingSorted(); !reflect.DeepEqual(got, []string{"file:^Imagenes"}) {
		t.Errorf("missing = %q", got)
	}
	want := []string{"file:^Imagenes: " + filepath.Join(root, "shared", "Imagenes") + " is a directory, not an image (skipped)"}
	if !reflect.DeepEqual(a.notImages, want) {
		t.Errorf("notImages = %q, want %q", a.notImages, want)
	}
}
