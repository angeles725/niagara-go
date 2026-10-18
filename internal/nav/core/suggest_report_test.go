package core

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func newRuntime(t *testing.T, base string) (*Runtime, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	return NewAt("", base, &out, &errb), &out, &errb
}

// The expected strings below were printed by module_nav_lib (suggest.py,
// class_search.class_scope) on the same data.

func TestClassScope(t *testing.T) {
	for _, c := range []struct{ data, want string }{
		{`{"_meta": {"modules": 3, "timestamp": "T1"}, "classes": {"BFoo": [{"module": "a-rt"},
		  {"module": "b-rt"}], "BFooBar": [{"module": "a-rt"}], "Baz": [{"module": "c"}]}}`,
			"4 class entries (3 names) from 3 modules, index built T1"},
		{`{"classes": {"A": [{"module": "x"}, {"module": null}, {"module": "x"}]}}`,
			"3 class entries (1 names) from 2 modules, index built at an unknown time"},
		{`{"classes": {}}`, "0 class entries (0 names) from 0 modules, index built at an unknown time"},
		{`{}`, "0 class entries (0 names) from 0 modules, index built at an unknown time"},
	} {
		if got := ClassScope(NewTestMeta(t, c.data)); got != c.want {
			t.Errorf("ClassScope(%s) = %q, want %q", c.data, got, c.want)
		}
	}
}

func TestClassMissAndReport(t *testing.T) {
	r, out, _ := newRuntime(t, t.TempDir())
	data := NewTestMeta(t, `{"_meta": {"modules": 3, "timestamp": "T1"}, "classes": {"BFoo": [{"module": "a-rt"},
		{"module": "b-rt"}], "BFooBar": [{"module": "a-rt"}], "Baz": [{"module": "c"}]}}`)
	got := r.ClassMiss("BFo", data, "", "search '*BFo*'")
	want := "  0 matches for class 'BFo' in 4 class entries (3 names) from 3 modules, index built T1.\n" +
		"  Try: search '*BFo*'\n  Did you mean: BFoo, BFooBar\n"
	if out.String() != want || !reflect.DeepEqual(got, []string{"BFoo", "BFooBar"}) || r.ExitCode() != 1 {
		t.Errorf("ClassMiss: %q %q exit %d", out.String(), got, r.ExitCode())
	}
	out.Reset()
	if got := r.ClassMiss("X", NewTestMeta(t, `{}`), "", ""); got != nil ||
		out.String() != "  0 matches for class 'X' in 0 class entries (0 names) from 0 modules, index built at an unknown time.\n" {
		t.Errorf("ClassMiss without classes: %q %q", out.String(), got)
	}
	out.Reset()
	if got := r.Report("thing 'zz'", "scope", "zz", []string{"aa"}, ""); got != nil ||
		out.String() != "  0 matches for thing 'zz' in scope.\n" {
		t.Errorf("Report: %q %q", out.String(), got)
	}
}

func TestCloseNamesNonPositiveLimit(t *testing.T) {
	for _, limit := range []int{0, -3} {
		if got := CloseNames("BFoo", []string{"BFoo", "bfoo", "BFooBar"}, limit); len(got) != 0 {
			t.Errorf("CloseNames(limit %d) = %q, Python []", limit, got)
		}
	}
}

func TestModuleMiss(t *testing.T) {
	base := t.TempDir()
	os.MkdirAll(filepath.Join(base, "indexes"), 0o755)
	inv := `{"_meta": {"timestamp": "T2"}, "modules": {"alarm-rt": {}, "alarm-ux": {}, "baja": {}}}`
	if err := os.WriteFile(filepath.Join(base, "indexes", "module-inventory.json"), []byte(inv), 0o644); err != nil {
		t.Fatal(err)
	}
	r, out, _ := newRuntime(t, base)
	got := r.ModuleMiss("alarm", "modules")
	want := "  0 matches for module 'alarm' in 3 submodules in module-inventory.json, index built T2.\n" +
		"  Try: modules\n  Did you mean: alarm-rt, alarm-ux\n"
	if out.String() != want || !reflect.DeepEqual(got, []string{"alarm-rt", "alarm-ux"}) {
		t.Errorf("ModuleMiss: %q %q", out.String(), got)
	}
	empty := t.TempDir()
	os.MkdirAll(filepath.Join(empty, "indexes"), 0o755)
	os.WriteFile(filepath.Join(empty, "indexes", "module-inventory.json"), []byte(`{}`), 0o644)
	r, out, errb := newRuntime(t, empty)
	if got := r.ModuleMiss("x", "modules"); got != nil || out.Len() != 0 || errb.Len() != 0 || r.ExitCode() != 0 {
		t.Errorf("empty inventory: %q %q %q exit %d", got, out.String(), errb.String(), r.ExitCode())
	}
}

func TestResolveOrganizedAndSourceRoot(t *testing.T) {
	corpus := t.TempDir()
	t.Setenv("NAV_CORPUS_BASE", corpus)
	r, _, errb := newRuntime(t, t.TempDir())
	if p, tried := r.ResolveOrganized(); p != corpus || tried != "" {
		t.Errorf("ResolveOrganized() = %q, %q", p, tried)
	}
	if got := r.SourceRoot(nil); got != corpus || errb.Len() != 0 {
		t.Errorf("SourceRoot = %q, stderr %q", got, errb.String())
	}
	missing := filepath.Join(corpus, "nope")
	t.Setenv("NAV_CORPUS_BASE", missing)
	if p, tried := r.ResolveOrganized(); p != "" || tried != missing {
		t.Errorf("ResolveOrganized() = %q, %q", p, tried)
	}
	for _, c := range []struct{ meta, hint string }{
		{`{}`, "Set NAV_CORPUS_BASE to the organized/ directory."},
		{`{"source": "/does/not/exist"}`, "Set NAV_CORPUS_BASE to the organized/ directory."},
		{`{"source_root": ` + strconv.Quote(corpus) + `}`, "The index was built from " + corpus +
			", which exists: export NAV_CORPUS_BASE=" + corpus},
	} {
		r, _, errb := newRuntime(t, t.TempDir())
		want := "ERROR: corpus not found: " + missing + "\n  " + c.hint + "\n"
		if got := r.SourceRoot(NewTestMeta(t, c.meta)); got != "" || errb.String() != want || r.ExitCode() != 2 {
			t.Errorf("SourceRoot(%s) = %q, stderr %q, want %q", c.meta, got, errb.String(), want)
		}
	}
}
