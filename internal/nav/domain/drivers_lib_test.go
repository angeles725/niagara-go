package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/lib/license"
)

// fixtureDrivers runs Drivers on the drivers golden fixture without the nav
// runtime. The license features are given, as a caller that scanned them
// would: acme-rt has one, acmeTcp-rt two.
func fixtureDrivers(t *testing.T) ([]*DriverRow, string) {
	t.Helper()
	load := func(name string) *index.Object {
		v, err := index.LoadPython(filepath.Join("..", "testdata", "fixture", "drivers", "indexes", name))
		if err != nil {
			t.Fatal(err)
		}
		return v.(*index.Object)
	}
	str := func(s string) *string { return &s }
	features := FeaturesByModule([]license.Call{
		{Module: "acme-rt", Vendor: str("acme"), Feature: str("acmeDriver")},
		{Module: "acmeTcp-rt", Feature: str("acmeTcp")},
		{Module: "acmeTcp-rt", Vendor: str("acme"), Feature: str("acmeTcpPlus")},
		{Module: "nn-rt", Vendor: str("nn")}, // no feature: not a license feature
	})
	return Drivers(DriverInputs{ClassIndex: load("class-index.json"), Inheritance: load("inheritance.json"),
		Inventory: load("module-inventory.json"), Features: features,
		Organized: filepath.Join("..", "testdata", "fixture", "drivers-organized")})
}

func goldenStdout(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "golden", name, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestDriversTableMatchesGolden: the scope, header and rows of nav-drivers.
func TestDriversTableMatchesGolden(t *testing.T) {
	rows, scope := fixtureDrivers(t)
	golden := strings.Split(goldenStdout(t, "nav-drivers"), "\n")
	if want := strings.TrimPrefix(golden[1], "  DRIVER NETWORKS: "); scope != want {
		t.Errorf("scope = %q, golden %q", scope, want)
	}
	header := DriverTableHeader()
	got := append(header[:], make([]string, 0, len(rows))...)
	for _, r := range rows {
		got = append(got, FormatDriverRow(r))
	}
	if want := golden[3 : 3+len(got)]; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("table:\n%s\ngolden:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if golden[3+len(got)] != "" {
		t.Errorf("%d rows, the golden table goes on with %q", len(rows), golden[3+len(got)])
	}
}

// TestDriversInAndAnatomyMatchGolden: `drivers acme-rt` (nav-drivers-module)
// is the anatomy block of the one network DriversIn keeps.
func TestDriversInAndAnatomyMatchGolden(t *testing.T) {
	rows, _ := fixtureDrivers(t)
	in := DriversIn(rows, "acme-rt")
	if len(in) != 1 {
		t.Fatalf("DriversIn(acme-rt) = %d rows, golden 1", len(in))
	}
	if got, want := strings.Join(FormatAnatomy(in[0]), "\n")+"\n", goldenStdout(t, "nav-drivers-module"); got != want {
		t.Errorf("FormatAnatomy:\n%s\ngolden:\n%s", got, want)
	}
	if n := len(DriversIn(rows, "acme")); n != 1 { // the module directory
		t.Errorf("DriversIn(acme) = %d rows, want 1", n)
	}
	if n := len(DriversIn(rows, "nosuch-rt")); n != 0 {
		t.Errorf("DriversIn(nosuch-rt) = %d rows, want 0", n)
	}
}
