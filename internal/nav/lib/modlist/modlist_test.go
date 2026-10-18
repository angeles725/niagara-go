package modlist

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// TestModulesMatchesGolden calls Modules on the nav fixture inventory without
// the nav runtime and compares the results line, header and rows with the
// `modules` goldens (filters, --top and a "-" type and "v?" bytecode).
func TestModulesMatchesGolden(t *testing.T) {
	v, err := index.LoadPython(filepath.Join("..", "..", "testdata", "fixture", "nav", "indexes", "module-inventory.json"))
	if err != nil {
		t.Fatal(err)
	}
	inv := v.(*index.Object)
	for _, tc := range []struct {
		golden string
		filter ModuleFilter
	}{
		{"nav-modules-type", ModuleFilter{Type: "rt"}},
		{"nav-modules-zkm-top", ModuleFilter{ZKM: true, Top: 2}},
		{"nav-modules-bytecode", ModuleFilter{HasCode: true, ByBytecode: true, Bytecode: 52}},
	} {
		t.Run(tc.golden, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", tc.golden, "stdout"))
			if err != nil {
				t.Fatal(err)
			}
			golden := strings.Split(string(b), "\n")
			rows := Modules(inv, tc.filter)
			header := ModuleTableHeader()
			got := []string{"  Modules" + tc.filter.Label() + ": " + strconv.Itoa(len(rows)) + " results", header[0], header[1]}
			for _, r := range rows {
				got = append(got, FormatModuleRow(r))
			}
			if want := golden[1 : 1+len(got)]; strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("got:\n%s\ngolden:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
			if golden[1+len(got)] != "" {
				t.Errorf("%d rows, the golden table goes on with %q", len(rows), golden[1+len(got)])
			}
		})
	}
}

// TestFormatModuleRowOfACallerRow: a row the caller builds formats its
// bytecode from the number.
func TestFormatModuleRowOfACallerRow(t *testing.T) {
	r := ModuleRow{Name: "x-rt", Type: "rt", JavaFiles: 1234, ClassCount: 5, Bytecode: 52}
	want := "  x-rt                                       rt   1,234       5       v52"
	if got := FormatModuleRow(r); got != want {
		t.Errorf("FormatModuleRow = %q, want %q", got, want)
	}
}
