package indexes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// The fixture's bajadoc-clean/ member sections: modifiers and generic
// return types, a space before "(", Unicode names and spaces, field
// descriptions after " - ", lines without a name, a section ended by an
// unindented line or an empty one, members before any class line.
func TestBuildMethodIndexFixture(t *testing.T) {
	checkS8Golden(t, "method", "method-index.json")
}

// method_indexer: _METHOD_NAME (`[^(]*?(\w+)\s*\(`), the constructor
// pattern (`(\w+)\s*\(` at the start) and the field rule (the last word
// before " - "), with Python's Unicode \w and \s; the expected names are
// CPython 3.15's.
func TestMemberNames(t *testing.T) {
	for _, c := range []struct{ sig, method, ctor, field string }{
		{"void doGet(WebOp c)", "doGet", "", "c)"},
		{"static <T extends BObject> T  make (Class<T> c)", "make", "", "c)"},
		{"BComponent()", "BComponent", "BComponent", "BComponent()"},
		{"BComponent \t(int x)", "BComponent", "BComponent", "x)"},
		{"new BComponent()", "BComponent", "", "BComponent()"},
		{"int x.y(int z)", "y", "", "z)"},
		{"void a(b(c)", "a", "", "a(b(c)"},
		{"(weird)", "", "", "(weird)"},
		{"void noParen", "", "", "noParen"},
		{"void ünïcode_näme(int x)", "ünïcode_näme", "", "x)"},
		{"void sp\u00a0ace\u00a0(int x)", "ace", "", "x)"},
		{"void f\u3000(x)", "f", "", "(x)"},
		{"void <T>(x)", "", "", "<T>(x)"},
		{"static Property servletName - the name - really", "", "", "servletName"},
		{"int\u2003wide", "", "", "wide"},
		{"\u2167x(y)", "\u2167x", "\u2167x", "\u2167x(y)"},
		{"x\u0301y(z)", "y", "", "x\u0301y(z)"}, // U+0301 (Mn) is not \w
		{" - lead", "", "", ""},
	} {
		if got := methodName(c.sig); got != c.method {
			t.Errorf("method %q: %q, want %q", c.sig, got, c.method)
		}
		if got := constructorName(c.sig); got != c.ctor {
			t.Errorf("constructor %q: %q, want %q", c.sig, got, c.ctor)
		}
		if got := fieldName(c.sig); got != c.field {
			t.Errorf("field %q: %q, want %q", c.sig, got, c.field)
		}
	}
}

// A page that is not UTF-8 stops the builder (Python does not catch the
// UnicodeDecodeError) and the old index stays.
func TestBuildMethodIndexNotUTF8(t *testing.T) {
	base := t.TempDir()
	testkit.WriteFile(t, filepath.Join(base, "bajadoc-clean", "a", "Bad.txt"), []byte("CLASS: Bad\n\nMETHODS:\n  void f()\n\xff"))
	testkit.WriteFile(t, filepath.Join(base, "indexes", "method-index.json"), []byte("old"))
	if _, err := BuildMethodIndex(base); err == nil {
		t.Fatal("no error for a page that is not UTF-8")
	}
	if data, _ := os.ReadFile(filepath.Join(base, "indexes", "method-index.json")); string(data) != "old" {
		t.Errorf("index replaced: %q", data)
	}
}
