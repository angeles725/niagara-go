package drivers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// The expected values below were printed by module_nav_lib/drivers.py on the
// same inputs.

func TestDocModule(t *testing.T) {
	d := &docIndex{byCore: map[string]string{}}
	for _, kv := range [][2]string{{"modbus", "docModbus-doc"}, {"mod", "docMod-doc"}, {"opcua", "docOpcUa-doc"},
		{"opc", "docOpc-doc"}, {"lon", "docLon-doc"}, {"lonw", "docLonw-doc"}, {"ab", "docAb-doc"},
		{"bacnetx", "docBacnetX-doc"}, {"bacnety", "docBacnetY-doc"}} {
		d.cores = append(d.cores, kv[0])
		d.byCore[kv[0]] = kv[1]
	}
	for dir, want := range map[string]string{"modbusTcp": "docModbus-doc", "Modbus": "docModbus-doc",
		"opc": "docOpc-doc", "op": "", "lonworks": "docLonw-doc", "bac": "docBacnetX-doc",
		"bacnet": "docBacnetX-doc", "ab": "docAb-doc", "abc": "", "zzz": "", "ModbusTCP": "docModbus-doc"} {
		got, ok := d.docModule(dir)
		if got != want || ok != (want != "") {
			t.Errorf("docModule(%q) = %q %v, Python %q", dir, got, ok, want)
		}
	}
}

func TestDocIndex(t *testing.T) {
	v, err := index.Decode([]byte(`{"docModbus-doc": {"module": "docModbus", "type": "doc"},
		"docSource-doc": {"module": "docSource", "type": "doc"}, "x-doc": {"module": "docÉtat", "type": "doc"},
		"y-rt": {"module": "docY", "type": "rt"}, "z-doc": {"module": "", "type": "doc"},
		"docModbus2-doc": {"module": "docModbus", "type": "doc"}}`))
	if err != nil {
		t.Fatal(err)
	}
	d := newDocIndex(v.(*index.Object))
	if want := []string{"modbus", "état"}; !reflect.DeepEqual(d.cores, want) ||
		d.byCore["modbus"] != "docModbus2-doc" || d.byCore["état"] != "x-doc" {
		t.Errorf("cores %q byCore %v", d.cores, d.byCore)
	}
	want := map[string]string{"docModbus": "docModbus2-doc", "docSource": "docSource-doc", "docÉtat": "x-doc"}
	if !reflect.DeepEqual(d.byModule, want) {
		t.Errorf("byModule %v, Python %v", d.byModule, want)
	}
}

func TestUnescape(t *testing.T) {
	for in, want := range map[string]string{`module\://docUser/doc`: "module://docUser/doc",
		`a\\b`: `a\b`, `end\`: `end\`, `\é`: "é"} {
		if got := unescape(in); got != want {
			t.Errorf("unescape(%q) = %q, Python %q", in, got, want)
		}
	}
}

// TestHelpGuideBases: the network's own submodule first, the last definition
// of a file wins, language variants and other modules' lexicons are skipped,
// a Latin-1 file is decoded as such, properties escapes are removed.
func TestHelpGuideBases(t *testing.T) {
	root := t.TempDir()
	for rel, data := range map[string]string{
		"acme/acme-rt/extracted/acme.lexicon":    "help.guide.base=module://first/doc\nhelp.guide.base = module\\://last/doc \t\r\n",
		"acme/acme-rt/extracted/acme_fr.lexicon": "help.guide.base=module://fr/doc\n",
		"acme/acme-rt/extracted/acmeX.lexicon":   "x.help.guide.base=no\n  help.guide.base:module://last/doc\n",
		"acme/acme-rt/extracted/other.lexicon":   "help.guide.base=module://other/doc\n",
		"acme/acme-wb/extracted/acme.lexicon":    "help.guide.base=caf\xe9\\ au\\\\lait\n",
		"acme/acme-ux/notextracted":              "",
		"acme/README":                            "",
	} {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		dir, sub string
		want     []string
	}{
		{"acme", "acme-wb", []string{`café au\lait`, "module://last/doc"}},
		{"acme", "acme-rt", []string{"module://last/doc", `café au\lait`}},
		{"nope", "x", nil},
	} {
		if got := helpGuideBases(root, c.dir, c.sub); !reflect.DeepEqual(got, c.want) {
			t.Errorf("helpGuideBases(%s, %s) = %q, Python %q", c.dir, c.sub, got, c.want)
		}
	}
}
