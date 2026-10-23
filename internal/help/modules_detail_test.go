package help

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleDetailLists(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    ModuleParams
		want string
	}{
		{"types", ModuleParams{Name: "kitControl-rt", Detail: "types"}, `TYPES of kitControl-rt (2):
  kitControl:BooleanLatch  com.tridium.kitControl.util.BBooleanLatch
  kitControl:NumericLatch  com.tridium.kitControl.util.BNumericLatch
`},
		{"agents", ModuleParams{Name: "kitControl-wb", Detail: "agents"}, `AGENTS of kitControl-wb (1):
  kitControl:LatchView  com.tridium.kitControl.wb.BLatchView
    on: kitControl:BooleanLatch
    app webShell; requires permissions r
`},
		{"permissions", ModuleParams{Name: "kitControl-rt", Detail: "permissions"}, `JAVA PERMISSIONS of kitControl-rt (2):
  station  java.lang.RuntimePermission "setIO"
  all      java.util.PropertyPermission "*" read
`},
		{"defs", ModuleParams{Name: "baja", Detail: "defs"}, `DEFS of baja (1):
  baja.home = module://baja
`},
		{"empty list", ModuleParams{Name: "baja", Detail: "agents"}, "AGENTS of baja (0):\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, misses := runModule(t, tt.p)
			if got != tt.want || misses != 0 {
				t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, tt.want)
			}
		})
	}
}

// TestModuleDetailNeedsOneModule: a list flag on an ambiguous name lists the
// candidates and exits 1 rather than picking one.
func TestModuleDetailNeedsOneModule(t *testing.T) {
	got, misses := runModule(t, ModuleParams{Name: "kitControl", Detail: "types"})
	if !strings.HasPrefix(got, "3 modules matching 'kitControl':\n") ||
		!strings.HasSuffix(got, "\n--types lists one module: name one of these.\n") || misses != 1 {
		t.Errorf("misses %d, got:\n%s", misses, got)
	}
}

func TestModuleJSON(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    ModuleParams
		want string
	}{
		{"summary", ModuleParams{Name: "control-rt", JSON: true}, `{
  "module": {
    "name": "control-rt",
    "module": "control",
    "runtime_profile": "rt",
    "vendor": "Tridium",
    "version": "4.15.6.20",
    "description": "Control points",
    "dependencies": [
      {
        "name": "baja",
        "vendor": "Tridium",
        "version": "4.15.6"
      }
    ],
    "types": [
      {
        "name": "BooleanPoint",
        "class": "javax.baja.control.BBooleanPoint"
      }
    ],
    "agents": [],
    "java_permissions": [],
    "defs": [],
    "adapters": [
      {
        "type": "BooleanPoint",
        "from": "baja:RelTime",
        "to": "control:TriggerMode"
      }
    ],
    "jar": "control-rt.jar"
  },
  "reverse_dependencies": [
    {
      "name": "kitControl-rt",
      "version": "4.15.6"
    }
  ]
}
`},
		{"list", ModuleParams{Name: "baja", Detail: "defs", JSON: true}, `{
  "module": "baja",
  "defs": [
    {
      "name": "baja.home",
      "value": "module://baja"
    }
  ]
}
`},
		{"several", ModuleParams{Name: "kitControl", JSON: true}, `{
  "query": "kitControl",
  "matches": [
    "kitControl-rt",
    "kitControl-ux",
    "kitControl-wb"
  ]
}
`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, misses := runModule(t, tt.p)
			if got != tt.want || misses != 0 {
				t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, tt.want)
			}
		})
	}
}

// TestModuleCLI: the list flags exclude one another, --json combines with
// them, and an ambiguous list request exits 1.
func TestModuleCLI(t *testing.T) {
	dir := repo(t, `{"default": "1.0.0.1", "versions": {"1.0.0.1": {"install_dir": ".", "data_root": "."}}}`, ".")
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "module-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "indexes", "module-index.json"), string(data))
	for _, tt := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"module", "baja", "--defs"}, 0, "DEFS of baja (1):\n"},
		{[]string{"module", "baja", "--defs", "--json"}, 0, "{\n  \"module\": \"baja\",\n  \"defs\": ["},
		{[]string{"module", "kitControl", "--types"}, 1, "3 modules matching 'kitControl':"},
		{[]string{"module", "baja", "--types", "--defs"}, 2, ""},
	} {
		out, errs, code := run(t, dir, tt.args...)
		if code != tt.code || !strings.HasPrefix(out, tt.out) {
			t.Errorf("%v: code %d stdout %q stderr %q", tt.args, code, out, errs)
		}
	}
}
