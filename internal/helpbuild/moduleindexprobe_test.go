package helpbuild

import (
	"os"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// TestModuleIndexDryProbe builds module-index.json from a real install into a
// temporary data root (the install is only read) and prints the element counts,
// which the docs compare with a grep audit of the same install. Set
// NH_MODULE_INDEX_PROBE to the install folder (the one holding modules/) to run
// it, with -v to see the counts.
func TestModuleIndexDryProbe(t *testing.T) {
	install := os.Getenv("NH_MODULE_INDEX_PROBE")
	if install == "" {
		t.Skip("NH_MODULE_INDEX_PROBE is not set (an install folder to probe read-only)")
	}
	root := t.TempDir()
	res, err := buildModuleIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range append(res.Log, res.Warnings...) {
		t.Log(l)
	}
	idx, err := index.LoadObject(index.Path(root, moduleIndexName))
	if err != nil {
		t.Fatal(err)
	}
	mods, _ := idx.Get("modules")
	keys := []string{"dependencies", "types", "agents", "java_permissions", "defs", "adapters"}
	sums := map[string]int{}
	on := 0
	for _, name := range mods.(*index.Object).Keys() {
		rec := mods.(*index.Object).Object(name)
		for _, k := range keys {
			v, _ := rec.Get(k)
			sums[k] += len(v.([]any))
		}
		agents, _ := rec.Get("agents")
		for _, a := range agents.([]any) {
			o, _ := a.(*index.Object).Get("on")
			on += len(o.([]any))
		}
	}
	errs, _ := idx.Get("errors")
	t.Logf("module.xml read: %d modules + %d errors", mods.(*index.Object).Len(), len(errs.([]any)))
	// Templates and schemas: indexed + unreadable rows (a row with a path), in how many jars.
	for _, k := range []string{"permission_templates", "schemas"} {
		n, jars := 0, 0
		for _, name := range mods.(*index.Object).Keys() {
			if v, ok := mods.(*index.Object).Object(name).Get(k); ok {
				n, jars = n+len(v.([]any)), jars+1
			}
		}
		t.Logf("%-20s %4d in %d jars", k, n, jars)
	}
	for _, e := range errs.([]any) {
		if p, ok := e.(*index.Object).String("path"); ok {
			t.Logf("unreadable member: %s", p)
		}
	}
	for _, k := range keys {
		t.Logf("%-17s %6d", k, sums[k])
	}
	t.Logf("%-17s %6d", "agent on-types", on)
}
