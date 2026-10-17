package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestDeclScannersMatchPython compares the hand scanners with corpus.py's
// regexes on testdata/decl.json (gen_decl.py).
func TestDeclScannersMatchPython(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "decl.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Line       string    `json:"line"`
		Decl       *[]string `json:"decl"`
		Type       *string   `json:"type"`
		Annotation bool      `json:"annotation"`
		Package    *string   `json:"package"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		d, ok := MatchDecl(c.Line)
		switch {
		case c.Decl == nil && ok:
			t.Errorf("MatchDecl(%q) = %+v, Python: no match", c.Line, d)
		case c.Decl != nil:
			want := *c.Decl
			if !ok || d.Indent != want[0] || d.Modifiers != want[1] || d.Kind != want[2] || d.Name != want[3] {
				t.Errorf("MatchDecl(%q) = %+v %v, Python %q", c.Line, d, ok, want)
			}
		}
		name, ok := TypeDeclName(c.Line)
		if (c.Type == nil) == ok || (ok && name != *c.Type) {
			t.Errorf("TypeDeclName(%q) = %q %v, Python %v", c.Line, name, ok, c.Type)
		}
		if StartsWithAnnotation(c.Line) != c.Annotation {
			t.Errorf("StartsWithAnnotation(%q) = %v", c.Line, !c.Annotation)
		}
		pkg, ok := PackageName(c.Line)
		if (c.Package == nil) == ok || (ok && pkg != *c.Package) {
			t.Errorf("PackageName(%q) = %q %v, Python %v", c.Line, pkg, ok, c.Package)
		}
	}
}
