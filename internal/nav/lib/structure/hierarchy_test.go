package structure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// navIndexes is the decoded inheritance.json and class index "classes" table
// of the nav golden fixture, read without the nav runtime.
func navIndexes(t *testing.T) (inh, classes *index.Object) {
	t.Helper()
	load := func(name string) *index.Object {
		v, err := index.LoadPython(filepath.Join("..", "..", "testdata", "fixture", "nav", "indexes", name))
		if err != nil {
			t.Fatal(err)
		}
		return v.(*index.Object)
	}
	return load("inheritance.json"), load("class-index.json").Object("classes")
}

// golden is the stdout lines of a module-nav golden case.
func golden(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", name, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(b), "\n")
}

// expectLines compares formatted rows with the golden lines from start.
func expectLines(t *testing.T, what string, got, golden []string, start int) {
	t.Helper()
	if start+len(got) > len(golden) {
		t.Fatalf("%s: %d rows overrun the golden", what, len(got))
	}
	for i, line := range got {
		if line != golden[start+i] {
			t.Errorf("%s row %d = %q, golden %q", what, i, line, golden[start+i])
		}
	}
}

// TestSubclassesMatchesGolden: nav-hierarchy-depth1 (cut rows included).
func TestSubclassesMatchesGolden(t *testing.T) {
	inh, classes := navIndexes(t)
	h, ok := Subclasses(inh, classes, "BComponent", 1)
	if !ok {
		t.Fatal("BComponent has subclasses in the fixture")
	}
	if h.Children != 8 || h.Descendants != 40 || !h.Located || h.Module != "baja (+doc source)" ||
		h.Package != "javax.baja.sys" {
		t.Errorf("header = %+v, golden 8 children, 40 descendants, baja (+doc source), javax.baja.sys", h)
	}
	var lines []string
	for _, r := range h.Rows {
		lines = append(lines, FormatTreeRow(r))
	}
	g := golden(t, "nav-hierarchy-depth1")
	expectLines(t, "Subclasses", lines, g, 5)
	if g[5+len(lines)] != "" {
		t.Errorf("Subclasses: %d rows, the golden tree goes on with %q", len(lines), g[5+len(lines)])
	}
	if _, ok := Subclasses(inh, classes, "NoSuchClass", 1); ok {
		t.Error("Subclasses of an unknown class: ok")
	}
}

// TestChainMatchesGolden: nav-hierarchy-chain (located and "?" rows).
func TestChainMatchesGolden(t *testing.T) {
	inh, classes := navIndexes(t)
	rows, ok := Chain(inh, classes, "BAlarmService")
	if !ok || len(rows) != 6 {
		t.Fatalf("Chain = %d rows, %v; golden 6", len(rows), ok)
	}
	expectLines(t, "Chain", FormatChain(rows), golden(t, "nav-hierarchy-chain"), 3)
	if _, ok := Chain(inh, classes, "NoSuchClass"); ok {
		t.Error("Chain of an unknown class: ok")
	}
}

// TestImplementorsMatchesGolden: nav-implementors and the case-insensitive
// resolution of nav-implementors-case.
func TestImplementorsMatchesGolden(t *testing.T) {
	inh, classes := navIndexes(t)
	name, rows, ok := Implementors(inh, classes, "BIAlarmSource")
	if !ok || name != "BIAlarmSource" || len(rows) != 3 {
		t.Fatalf("Implementors = %q, %d rows, %v; golden BIAlarmSource, 3", name, len(rows), ok)
	}
	expectLines(t, "Implementors", []string{FormatImplementor(rows[0]), FormatImplementor(rows[1])},
		golden(t, "nav-implementors"), 5)
	name, rows, ok = Implementors(inh, classes, "biservice")
	if !ok || name != "BIService" || len(rows) != 1 {
		t.Fatalf("Implementors(biservice) = %q, %d rows, %v; golden BIService, 1", name, len(rows), ok)
	}
	expectLines(t, "Implementors(biservice)", []string{FormatImplementor(rows[0])},
		golden(t, "nav-implementors-case"), 5)
	if _, _, ok := Implementors(inh, classes, "BNoSuchInterface"); ok {
		t.Error("Implementors of an unknown interface: ok")
	}
}
