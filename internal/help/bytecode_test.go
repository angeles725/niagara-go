package help

import (
	"bytes"
	"strings"
	"testing"
)

// runBytecodeClass runs `class` on the bytecode fixture root.
func runBytecodeClass(t *testing.T, p ClassParams) (string, int) {
	t.Helper()
	var out bytes.Buffer
	e := &Env{BaseDir: bytecodeRoot(t), Out: &out}
	if err := e.Class(p); err != nil {
		t.Fatal(err)
	}
	return out.String(), e.misses
}

// TestClassFallsBackToBytecode: a class with neither javadoc nor source is
// shown from its class file, every line tagged [bytecode]; a slot known only
// by its field says so.
func TestClassFallsBackToBytecode(t *testing.T) {
	got, misses := runBytecodeClass(t, ClassParams{Name: "BLatchView"})
	want := `CLASS: BLatchView [bytecode]
PACKAGE: com.tridium.kitControl.wb [bytecode]
TYPES: kitControl:LatchView (kitControl-wb.jar) [bytecode]
No javadoc (.bajadoc) or Java source for this class; its class file is indexed:
BYTECODE: bytecode-index.json (kitControl-wb.jar!com/tridium/kitControl/wb/BLatchView.class)
ACCESS: public [bytecode]
SUPER: javax.baja.workbench.view.BWbComponentView (not in bytecode-index.json) [bytecode]
INTERFACES: javax.baja.workbench.BIView [bytecode]
SLOTS (1 declared, 0 inherited) [bytecode]:
  topic    fired  (field only: no slot annotation)
METHODS (1 public, declared) [bytecode]: <init>
`
	if got != want || misses != 0 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

// TestClassBytecodeInheritsSlots: --bytecode shows a class with javadoc from
// its class file too; the supers the index holds are walked, an override
// hides the inherited slot it re-declares, and the chain names where it
// leaves the index.
func TestClassBytecodeInheritsSlots(t *testing.T) {
	got, misses := runBytecodeClass(t, ClassParams{Name: "com.tridium.kitControl.util.BBooleanLatch", Bytecode: true})
	want := `CLASS: BBooleanLatch [bytecode]
PACKAGE: com.tridium.kitControl.util [bytecode]
TYPES: kitControl:BooleanLatch (kitControl-rt.jar) [bytecode]
BYTECODE: bytecode-index.json (kitControl-rt.jar!com/tridium/kitControl/util/BBooleanLatch.class)
ACCESS: public [bytecode]
SUPER: com.tridium.kitControl.util.BLatch > javax.baja.sys.BComponent > javax.baja.sys.BComplex (not in bytecode-index.json) [bytecode]
INTERFACES: (none) [bytecode]
SLOTS (3 declared, 2 inherited) [bytecode]:
  property facets: BFacets = BFacets.makeBoolean()  (override)
  property out: BStatusBoolean = new BStatusBoolean()  flags rtso
  property in: BStatusBoolean = new BStatusBoolean()  flags s
  inherited from BLatch:
    property clock: BStatusBoolean = new BStatusBoolean()  flags tso
    action   latch()
METHODS (3 public, declared) [bytecode]: <init>, getOut, getType
`
	if got != want || misses != 0 {
		t.Errorf("misses %d, got:\n%s\nwant:\n%s", misses, got, want)
	}
}

// TestClassBytecodeMiss: a name no index holds is a class miss whose scope
// names bytecode-index.json; --bytecode misses in its own scope.
func TestClassBytecodeMiss(t *testing.T) {
	for _, tt := range []struct {
		p    ClassParams
		want string
	}{
		{ClassParams{Name: "BNowhere"}, "bytecode-index.json: 4 classes (1 unreadable: kitControl-ux.jar!com/tridium/kitControl/ux/BLatchTypeExt.class)"},
		{ClassParams{Name: "BNowhere", Bytecode: true}, "excluded from this scope: classes no module.xml registers as a type"},
	} {
		got, misses := runBytecodeClass(t, tt.p)
		if misses != 1 || !strings.Contains(got, tt.want) {
			t.Errorf("%+v: misses %d, lacks %q:\n%s", tt.p, misses, tt.want, got)
		}
	}
}
