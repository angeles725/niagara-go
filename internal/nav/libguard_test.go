package nav

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const navPath = "github.com/angeles725/niagara-go/internal/nav"

// libraryImportAllowed says whether a library package may import path: any
// package outside internal/nav, core, or another library package.
func libraryImportAllowed(path string) bool {
	if path != navPath && !strings.HasPrefix(path, navPath+"/") {
		return true
	}
	return path == navPath+"/core" || strings.HasPrefix(path, navPath+"/lib/")
}

// guarded are the packages whose selectors forbiddenSelectors names, by the
// name the selectors use.
var guarded = map[string]string{navPath + "/core": "core", "os": "os", "fmt": "fmt"}

// forbiddenSelectors are the runtime and printing calls a library never uses.
var forbiddenSelectors = map[string]bool{
	"core.Runtime": true, "core.New": true,
	"os.Stdout": true, "os.Stderr": true,
	"fmt.Print": true, "fmt.Println": true, "fmt.Printf": true,
}

// TestLibrariesAreImportClean enforces the rule of doc.go on every non-test
// file under lib/: no registry, nav or command package import (so importing
// a library registers no command), no runtime and no printing.
func TestLibrariesAreImportClean(t *testing.T) {
	fset := token.NewFileSet()
	files := 0
	err := filepath.WalkDir("lib", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		files++
		local := map[string]string{} // the file's name of each guarded package
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			if canon, ok := guarded[p]; ok {
				name := canon
				if imp.Name != nil {
					name = imp.Name.Name
				}
				local[name] = canon
			}
			if !libraryImportAllowed(p) {
				t.Errorf("%s imports %s: a library imports only core and other lib packages from internal/nav", path, p)
			}
			if p == "log" {
				t.Errorf("%s imports log: a library never prints", path)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && forbiddenSelectors[local[x.Name]+"."+sel.Sel.Name] {
					t.Errorf("%s: uses %s.%s (a library takes no runtime and never prints)",
						fset.Position(sel.Pos()), local[x.Name], sel.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("no library files found under lib/")
	}
}

// TestLibraryGuardRejects proves the guard's predicates catch what they must.
func TestLibraryGuardRejects(t *testing.T) {
	for _, p := range []string{navPath + "/registry", navPath, navPath + "/source", navPath + "/rankcmd"} {
		if libraryImportAllowed(p) {
			t.Errorf("libraryImportAllowed(%q) = true, want false", p)
		}
	}
	for _, p := range []string{navPath + "/core", navPath + "/lib/license", "fmt", "github.com/angeles725/niagara-go/internal/index"} {
		if !libraryImportAllowed(p) {
			t.Errorf("libraryImportAllowed(%q) = false, want true", p)
		}
	}
}
