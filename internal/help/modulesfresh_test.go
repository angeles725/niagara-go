package help

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestModuleIndexFreshness: module-index.json records the jar count and bytes
// of the install's modules/ it read; freshness compares them with the install
// now.
func TestModuleIndexFreshness(t *testing.T) {
	install := t.TempDir()
	mods := filepath.Join(install, "modules")
	write(t, filepath.Join(mods, "a-rt.jar"), "12345")
	write(t, filepath.Join(mods, "b-rt.JAR"), "1")
	write(t, filepath.Join(mods, "notes.txt"), "not a jar")
	root := t.TempDir()
	line := func() (string, bool) {
		var out bytes.Buffer
		stale := (&Env{BaseDir: root, Out: &out}).installIndexLine(moduleIndexName, install)
		return strings.ReplaceAll(out.String(), install, "<INSTALL>"), stale
	}
	if got, stale := line(); got != "  module-index.json: absent (rebuild-all builds it from the install's modules/)\n" || stale {
		t.Errorf("absent: %q %v", got, stale)
	}
	write(t, filepath.Join(root, "indexes", "module-index.json"), `{"jars": 2, "jar_bytes": 6, "modules": {}}`)
	if got, stale := line(); got != "  module-index.json: matches <INSTALL>/modules (2 jars, 6 bytes)\n" || stale {
		t.Errorf("in sync: %q %v", got, stale)
	}
	write(t, filepath.Join(mods, "c-rt.jar"), "12")
	want := "  module-index.json: STALE — built from 2 jars (6 bytes); <INSTALL>/modules now holds 3 jars (8 bytes)\n"
	if got, stale := line(); got != want || !stale {
		t.Errorf("stale: %q %v", got, stale)
	}
	if err := os.RemoveAll(mods); err != nil {
		t.Fatal(err)
	}
	want = "  module-index.json: not compared — <INSTALL>/modules is not reachable\n"
	if got, stale := line(); got != want || stale {
		t.Errorf("unmounted: %q %v", got, stale)
	}
	var out bytes.Buffer
	if (&Env{BaseDir: root, Out: &out}).installIndexLine(moduleIndexName, "") ||
		out.String() != "  module-index.json: not compared — no install is registered for this data root\n" {
		t.Errorf("unregistered: %q", out.String())
	}
}

// TestInstallIndexFreshness: palette-index.json, tag-index.json,
// lexicon-index.json and bytecode-index.json are compared with the install the same way, each under
// its own name, and freshness checks every one.
func TestInstallIndexFreshness(t *testing.T) {
	for _, name := range []string{paletteIndexName, tagIndexName, lexiconIndexName, bytecodeIndexName} {
		install, root := t.TempDir(), t.TempDir()
		write(t, filepath.Join(install, "modules", "a-rt.jar"), "123")
		write(t, filepath.Join(root, "indexes", name), `{"jars": 1, "jar_bytes": 3, "files": []}`)
		var out bytes.Buffer
		if (&Env{BaseDir: root, Out: &out}).installIndexLine(name, install) ||
			out.String() != "  "+name+": matches "+filepath.Join(install, "modules")+" (1 jars, 3 bytes)\n" {
			t.Errorf("%s in sync: %q", name, out.String())
		}
		write(t, filepath.Join(install, "modules", "a-rt.jar"), "1234")
		out.Reset()
		if !(&Env{BaseDir: root, Out: &out}).installIndexLine(name, install) ||
			!strings.HasPrefix(out.String(), "  "+name+": STALE — built from 1 jars (3 bytes); ") {
			t.Errorf("%s stale: %q", name, out.String())
		}
		if !slices.Contains(installIndexes, name) {
			t.Errorf("freshness does not check %s: %q", name, installIndexes)
		}
	}
}
