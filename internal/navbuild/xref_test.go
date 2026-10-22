package navbuild

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFindImports(t *testing.T) {
	text := "package p;\n\nimport a.B;\n  import static c.D.e ;\n\n \t\nimport f.*;\nimport static;\nimport g.h.; import i.J;\n" +
		"// import k.L;\nimportx m.N;\nimport static o.P.*;\nimport static.q;\nimport café.Été;\nimport r.S"
	want := []importMatch{{false, "a.B"}, {true, "c.D.e"}, {false, "f.*"}, {false, "static"}, {false, "g.h."},
		{true, "o.P.*"}, {false, "static.q"}, {false, "café.Été"}}
	if got := findImports(text); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestFirstLines(t *testing.T) {
	for _, tc := range []struct {
		content string
		n       int
		want    string
	}{
		{"a\nb\nc", 2, "a\nb"},
		{"a\nb\n", 2, "a\nb"},
		{"a\nb", 2, "a\nb"},
		{"a", 2, "a"},
		{"", 2, ""},
		{strings.Repeat("x\n", 3), 3, "x\nx\nx"},
	} {
		if got := firstLines(tc.content, tc.n); got != tc.want {
			t.Errorf("firstLines(%q, %d) = %q, want %q", tc.content, tc.n, got, tc.want)
		}
	}
}

// TestComputeXrefSkipsPathlessAndFailedSources pins build_xref's per-entry
// gate: an entry without a path is neither parsed nor failed but still
// resolves imports, and a missing file or a directory counts as failed.
func TestComputeXrefSkipsPathlessAndFailedSources(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "D.java"), []byte("package q;\nimport p.A;\nimport x.Y;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "C.java"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries := []classIndexEntry{
		{Name: "A", Package: "p", Module: "m1"},
		{Name: "B", Package: "p", Module: "m1", Path: "missing/B.java"},
		{Name: "C", Package: "p", Module: "m1", Path: "C.java"},
		{Name: "D", Package: "q", Module: "m2", Path: "D.java"},
	}
	x := ComputeXref(entries, dir)
	if x.FilesParsed != 1 || x.FilesFailed != 2 || x.ImportStatements != 2 || x.ResolvedImports != 1 {
		t.Errorf("parsed %d failed %d statements %d resolved %d, want 1 2 2 1",
			x.FilesParsed, x.FilesFailed, x.ImportStatements, x.ResolvedImports)
	}
	if got := x.ClassImports.vals["D"]; !reflect.DeepEqual(got, []string{"A"}) {
		t.Errorf("class_imports[D] = %v, want [A]", got)
	}
	if got := x.ModuleDeps.vals["m2"]; !reflect.DeepEqual(got, []string{"m1"}) {
		t.Errorf("module_deps[m2] = %v, want [m1]", got)
	}
	if got := x.ExternalImportedBy.vals["x.Y"]; !reflect.DeepEqual(got, []string{"D"}) {
		t.Errorf("external_imported_by[x.Y] = %v, want [D]", got)
	}
}
