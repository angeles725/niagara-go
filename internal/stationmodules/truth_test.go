package stationmodules

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJar writes a jar holding META-INF/module.xml = moduleXML ("" for a jar without it).
func writeJar(t *testing.T, path, moduleXML string) {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	name, body := "META-INF/module.xml", moduleXML
	if moduleXML == "" {
		name, body = "readme.txt", "no module.xml"
	}
	w, err := z.Create(name)
	if err == nil {
		_, err = w.Write([]byte(body))
	}
	if err == nil {
		err = z.Close()
	}
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err == nil {
		err = os.WriteFile(path, buf.Bytes(), 0o644)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte(readFile(t, src)), 0o644); err != nil {
		t.Fatal(err)
	}
}

const fooRtXML = `<module name="foo-rt" moduleName="foo" runtimeProfile="rt" vendor="Tridium" vendorVersion="4.14.0.162">` +
	`<types><type name="Known" class="x.BKnown"/></types></module>`

// versionMismatchInstall is an install whose foo-rt (4.14.0.162) is installed and staged
// byte-identical in sw/, with a station that uses foo:Known and foo:Newer (not in that version).
func versionMismatchInstall(t *testing.T) (root, station string) {
	t.Helper()
	root = t.TempDir()
	jar := filepath.Join(root, "modules", "foo-rt.jar")
	writeJar(t, jar, fooRtXML)
	copyFile(t, jar, filepath.Join(root, "sw", "4.14.0.162", "foo-rt.jar"))
	station = filepath.Join(t.TempDir(), "station.xml")
	xml := `<?xml version='1.0'?><bajaObjectGraph version='4.0'><p h='1' m='f=foo' t='f:Known'>` +
		`<p n='N' h='2' t='f:Newer'/></p></bajaObjectGraph>`
	if err := os.WriteFile(station, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, station
}

// TestInstalledPartWithoutTheTypeIsAVersionMismatch: a type the installed part does not
// provide means the station needs another version of the module, not that the part is
// missing; nothing is planned (the staged jar is the one already installed).
func TestInstalledPartWithoutTheTypeIsAVersionMismatch(t *testing.T) {
	root, station := versionMismatchInstall(t)
	code, out, errText := run("check", station, "--install", root)
	if code != 1 || errText != "" {
		t.Fatalf("exit %d stderr %q\n%s", code, errText, out)
	}
	if strings.Contains(out, "✗ foo-rt") || strings.Contains(out, "MISSING module-parts") || strings.Contains(out, "cp ") {
		t.Errorf("an installed part is reported missing or a copy is planned:\n%s", out)
	}
	want := "\nTYPES NOT IN THE INSTALLED VERSION: 1\n" +
		"  ✗ foo:Newer  (installed foo-rt v4.14.0.162 does not provide it; the station needs a different foo version)\n"
	if !strings.Contains(out, want) {
		t.Errorf("no version-mismatch report, want %q in:\n%s", want, out)
	}
	code, out, _ = run("check", station, "--install", root, "--json")
	if code != 1 || !strings.Contains(out, `"missing": {}`) ||
		!strings.Contains(out, `"foo:Newer": "installed foo-rt v4.14.0.162 does not provide it; the station needs a different foo version"`) {
		t.Errorf("json exit %d\n%s", code, out)
	}
}

// TestNoCopyIsPlannedOntoAnIdenticalJar: a missing part whose modules/ jar is byte-identical
// to the staged one gets no cp (it would copy the file onto itself), with or without --fix.
func TestNoCopyIsPlannedOntoAnIdenticalJar(t *testing.T) {
	root := t.TempDir()
	writeJar(t, filepath.Join(root, "modules", "foo-rt.jar"),
		`<module name="foo-rt" moduleName="foo" runtimeProfile="rt" vendor="Tridium" vendorVersion="4.14.0.162">`+
			`<dependencies><dependency name="bar-rt"/></dependencies></module>`)
	bar := filepath.Join(root, "modules", "bar-rt.jar")
	writeJar(t, bar, "") // there, but not a readable module-part
	copyFile(t, bar, filepath.Join(root, "sw", "4.14.0.162", "bar-rt.jar"))
	for _, args := range [][]string{{"doctor", "--install", root}, {"doctor", "--install", root, "--fix"}} {
		code, out, errText := run(args...)
		if code != 1 || errText != "" {
			t.Fatalf("%v: exit %d stderr %q\n%s", args, code, errText, out)
		}
		if strings.Contains(out, "cp ") || strings.Contains(out, "COPIED") {
			t.Errorf("%v: copy planned onto an identical jar:\n%s", args, out)
		}
		if want := "      → identical jar already at " + bar + " (copying it changes nothing)\n"; !strings.Contains(out, want) {
			t.Errorf("%v: want %q in:\n%s", args, want, out)
		}
	}
	_, out, _ := run("doctor", "--install", root, "--json")
	if strings.Contains(out, `"cp"`) || !strings.Contains(out, `"action": "identical"`) {
		t.Errorf("json plans a copy onto an identical jar:\n%s", out)
	}
}
