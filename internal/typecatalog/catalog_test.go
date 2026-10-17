package typecatalog

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

func run(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := Main(args, cli.Stdio{Out: &out, Err: &errb})
	return out.String(), errb.String(), code
}

// build --out writes the same JSON as stdout, without the final newline, and prints the counts.
func TestBuildOutWritesTheCatalog(t *testing.T) {
	target := filepath.Join(t.TempDir(), "cat.json")
	out, errOut, code := run("build", "testdata/loose", "--out", target)
	if code != 0 || errOut != "" || out != "types: 3  properties: 5  actions: 0  topics: 0\n" {
		t.Fatalf("code %d out %q err %q", code, out, errOut)
	}
	written, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _, _ := run("build", "testdata/loose")
	if string(written)+"\n" != stdout {
		t.Error("--out content differs from the stdout catalog")
	}
	if _, errOut, code := run("build", "testdata/loose", "--out", filepath.Join(target, "x", "y")); code != cli.ExitCannotSearch || !strings.Contains(errOut, "cannot write") {
		t.Errorf("unwritable --out: code %d err %q", code, errOut)
	}
}

func TestLoadJSONKeepsFileOrderAndLastDuplicate(t *testing.T) {
	cat, err := loadJSON(strings.NewReader(`{"z.BZ": {"class": "BZ", "extends": "A", "properties": [], "actions": [], "topics": []},
		"a.BA": {"class": "BA", "extends": "old"}, "a.BA": {"class": "BA", "extends": "new",
		"properties": [{"name": "p", "flags": 1026, "flagLetters": "tf", "default": "x", "facets": "null"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cat.Keys, ",") != "z.BZ,a.BA" || cat.Get("a.BA").Extends != "new" || cat.Get("a.BA").Properties[0].Flags != 1026 {
		t.Errorf("keys %v rec %+v", cat.Keys, cat.Get("a.BA"))
	}
	if _, err := loadJSON(strings.NewReader(`[1]`)); err == nil {
		t.Error("a JSON list was accepted as a catalog")
	}
}

func TestShowUnreadableCatalogExits2(t *testing.T) {
	_, errOut, code := run("show", filepath.Join(t.TempDir(), "missing.json"), "X")
	if code != cli.ExitCannotSearch || !strings.HasPrefix(errOut, "n4-type-catalog: cannot read catalog ") {
		t.Errorf("code %d err %q", code, errOut)
	}
}

func TestLayoutModule(t *testing.T) {
	cases := map[string]string{
		"organized/control/control-rt/vineflower/javax/X.java": "control",
		`C:\o\Acme\acme-ux\cfr\X.java`:                         "acme",
		"organized/acme-extras/acme-extras-rt/x/X.java":        "acme-extras",
		"loose/a/b/X.java":                                     "",
	}
	for src, want := range cases {
		if got, _ := layoutModule(src); got != want {
			t.Errorf("%s -> %q, want %q", src, got, want)
		}
	}
}

func TestSelftestReportsSkipOrPass(t *testing.T) {
	t.Setenv("NIAGARA_ORGANIZED", "off")
	var errb bytes.Buffer
	if code := selftest(&errb); code != 0 || !strings.Contains(errb.String(), "OK (skipped=1)") ||
		!strings.Contains(errb.String(), "RealDataTest.test_bnumericwritable_in_docsource skipped") {
		t.Errorf("code %d\n%s", code, errb.String())
	}
}
