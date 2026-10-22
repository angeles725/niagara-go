package help

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `package <name>` prints the package description (package-text/) above
// its classes; a package without one lists its classes only.
func TestPackageShowsDescription(t *testing.T) {
	base := t.TempDir()
	write := func(rel, data string) {
		p := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("indexes/class-index.json", `{"BAlarmClass": {"package": "javax.baja.alarm", "type": "class"},`+
		` "BOther": {"package": "p.plain", "type": "class"}}`)
	write("package-text/javax.baja.alarm.txt", "TITLE: javax.baja.alarm package\nPACKAGE: javax.baja.alarm\n"+
		"MODULE: alarm-rt\n\nDESCRIPTION:\nEvery alarm has an alarm class.\n")
	var out, errb strings.Builder
	e := &Env{Out: &out, Err: &errb, BaseDir: base}
	if err := e.Package(PackageParams{Name: "javax.baja.alarm"}); err != nil {
		t.Fatal(err)
	}
	want := "javax.baja.alarm (1 classes):\n\n  DESCRIPTION:\n  Every alarm has an alarm class.\n" +
		"  (package-text/javax.baja.alarm.txt)\n\n  BAlarmClass (class)\n\n"
	if out.String() != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out.String(), want)
	}
	out.Reset()
	if err := e.Package(PackageParams{Name: "p.plain"}); err != nil {
		t.Fatal(err)
	}
	if want := "p.plain (1 classes):\n\n  BOther (class)\n\n"; out.String() != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", out.String(), want)
	}
}
