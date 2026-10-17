package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIterJavaOrderAndSubmodules(t *testing.T) {
	org := t.TempDir()
	touch(t, filepath.Join(org, "b", "b-rt", "vineflower", "z", "Z.java"))
	touch(t, filepath.Join(org, "b", "b-rt", "vineflower", "A.java"))
	touch(t, filepath.Join(org, "b", "b-rt", "vineflower", "a", "B.java"))
	touch(t, filepath.Join(org, "b", "b-rt", "vineflower", "notes.txt"))
	touch(t, filepath.Join(org, "a", "a", "a.jar"))
	touch(t, filepath.Join(org, "a", "docs", "readme"))
	if err := os.Symlink(filepath.Join(org, "b", "b-rt", "vineflower", "z"),
		filepath.Join(org, "b", "b-rt", "vineflower", "link")); err != nil {
		t.Fatal(err)
	}
	var skipped []string
	subs := IterSubmodules(org, &skipped)
	if len(subs) != 2 || subs[0].Name != "a" || subs[1].Name != "b-rt" || !reflect.DeepEqual(skipped, []string{"a/docs"}) {
		t.Errorf("submodules %+v skipped %q", subs, skipped)
	}
	var rels []string
	for _, f := range IterJava(org) {
		rels = append(rels, f.Rel)
	}
	want := []string{"b/b-rt/vineflower/A.java", "b/b-rt/vineflower/a/B.java", "b/b-rt/vineflower/z/Z.java"}
	if !reflect.DeepEqual(rels, want) {
		t.Errorf("IterJava %q, want %q (sorted, top-down, symlinked dir not entered)", rels, want)
	}
	for _, c := range [][3]string{{"a", "a", "standalone"}, {"a", "a-rt", "rt"}, {"a", "a-xx", "other"}, {"a", "a-doc", "doc"}} {
		if got := SubmoduleKind(c[0], c[1]); got != c[2] {
			t.Errorf("SubmoduleKind(%s, %s) = %s", c[0], c[1], got)
		}
	}
	if SubmoduleType("a", "a") != "" || SubmoduleType("a", "a-xx") != "" {
		t.Error("SubmoduleType of standalone / unknown suffix")
	}
}

func TestWalkJavaFiltersSubmodulesAndStopsEarly(t *testing.T) {
	org := t.TempDir()
	for _, rel := range []string{"a/a-rt/vineflower/A.java", "a/a-rt/vineflower/p/B.java",
		"a/a-wb/vineflower/C.java", "b/b-rt/vineflower/D.java"} {
		touch(t, filepath.Join(org, filepath.FromSlash(rel)))
	}
	walk := func(keep func(Submodule) bool, stopAfter int) []string {
		var got []string
		WalkJava(org, keep, func(f JavaFile) bool {
			got = append(got, f.Sub+":"+f.Rel)
			return len(got) < stopAfter
		})
		return got
	}
	all := walk(nil, 99)
	var iter []string
	for _, f := range IterJava(org) {
		iter = append(iter, f.Sub+":"+f.Rel)
	}
	if !reflect.DeepEqual(all, iter) || len(all) != 4 {
		t.Errorf("WalkJava %q, IterJava %q", all, iter)
	}
	if got := walk(nil, 2); !reflect.DeepEqual(got, all[:2]) {
		t.Errorf("stop after 2: %q (the walk must end inside a nested directory too)", got)
	}
	rt := walk(func(s Submodule) bool { return SubmoduleKind(s.Module, s.Name) == "rt" }, 99)
	if want := []string{all[0], all[1], all[3]}; !reflect.DeepEqual(rt, want) {
		t.Errorf("rt submodules %q, want %q", rt, want)
	}
}

func TestTopLevelFiles(t *testing.T) {
	classes := NewTestMeta(t, `{
		"A": [{"path": "p/A.java", "outer_class": null}, {"path": "q/A.java", "outer_class": "None"}],
		"A$In": [{"path": "p/A.java", "outer_class": "A"}],
		"B": [{"path": "p/A.java", "outer_class": ""}, {"path": "", "outer_class": null}],
		"C": [{"path": "p/C.java", "inner": true}, {"path": "p/C2.java"}]}`)
	var got []string
	for _, e := range TopLevelFiles(classes) {
		p, _ := e.Entry.String("path")
		got = append(got, e.Name+"="+p)
	}
	want := []string{"A=p/A.java", "A=q/A.java", "C=p/C2.java"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPyFormatting(t *testing.T) {
	if Commas(1234567) != "1,234,567" || Commas(-1000) != "-1,000" || Commas(999) != "999" {
		t.Error("Commas")
	}
	if Fixed(0.125, 2) != "0.12" || Fixed(17.95, 1) != "17.9" || Fixed(2.5, 0) != "2" {
		t.Error("Fixed rounds like Python format")
	}
	if RJust("é", 3) != "  é" || LJust("é", 3) != "é  " || Cut("aéb", 2) != "aé" || Cut("ab", 5) != "ab" {
		t.Error("code point widths")
	}
	v := NewTestMeta(t, `{"a": [1, 2.50, null, true, "x'"], "b": 1e400, "c": -0}`)
	if got := PyRepr(v); got != `{'a': [1, 2.5, None, True, "x'"], 'b': inf, 'c': 0}` {
		t.Errorf("PyRepr %s", got)
	}
}
