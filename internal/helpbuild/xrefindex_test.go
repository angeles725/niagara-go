package helpbuild

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An import whose name ends with "." is where xref_builder raises
// (IndexError on parts[-1][0]): BuildXrefIndex returns the error, prints
// no "Built" line and writes no xref-index.json, whatever file comes
// before or after it.
func TestBuildXrefIndexImportEndingWithDot(t *testing.T) {
	for _, c := range []struct{ what, line string }{
		{"plain", "import c.;"},
		{"static", "import static c.D.;"},
		{"spaced", "import   c.d.  ;"},
	} {
		t.Run(c.what, func(t *testing.T) {
			base := t.TempDir()
			writeFile(t, filepath.Join(base, "source", "m-rt", "a", "A.java"), []byte("package a;\nimport b.B;\npublic class A {}\n"))
			writeFile(t, filepath.Join(base, "source", "m-rt", "b", "B.java"), []byte("package b;\n"+c.line+"\npublic class B {}\n"))
			writeFile(t, filepath.Join(base, "source", "m-rt", "c", "C.java"), []byte("package c;\npublic class C {}\n"))
			res, err := BuildXrefIndex(base)
			if !errors.Is(err, errEmptyImportName) {
				t.Fatalf("err %v, want %v", err, errEmptyImportName)
			}
			for _, l := range res.Log {
				if strings.HasPrefix(l, "Built") {
					t.Errorf("printed %q after the error", l)
				}
			}
			if _, err := os.Stat(filepath.Join(base, "indexes", "xref-index.json")); !os.IsNotExist(err) {
				t.Errorf("xref-index.json written (stat: %v)", err)
			}
		})
	}
}
