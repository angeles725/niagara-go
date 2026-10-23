package help

import (
	"bytes"
	"os"
	"path/filepath"
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
		stale := (&Env{BaseDir: root, Out: &out}).moduleIndexLine(install)
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
	if (&Env{BaseDir: root, Out: &out}).moduleIndexLine("") ||
		out.String() != "  module-index.json: not compared — no install is registered for this data root\n" {
		t.Errorf("unregistered: %q", out.String())
	}
}
