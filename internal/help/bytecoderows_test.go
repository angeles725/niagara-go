package help

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// bytecodeRoot is a data root holding the bytecode-index.json fixture and a
// class index with BBooleanLatch only, so one class has javadoc.
func bytecodeRoot(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "indexes", "bytecode-index.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := moduleRoot(t)
	write(t, filepath.Join(root, "indexes", "bytecode-index.json"), string(data))
	write(t, filepath.Join(root, "indexes", "class-index.json"), `{"BBooleanLatch": {"path": "com/tridium/kitControl/util/BBooleanLatch.html", `+
		`"package": "com.tridium.kitControl.util", "type": "class", "module": "kitControl-rt"}}`)
	return root
}

// TestBytecodeSlotsWalkSupers: the supers of a class are walked as far as
// bytecode-index.json holds them; its declared slots come first, then each
// super's that no nearer class re-declares (an override hides the inherited
// slot), each line naming kind, type, default, flag letters and override.
func TestBytecodeSlotsWalkSupers(t *testing.T) {
	var out bytes.Buffer
	e := &Env{BaseDir: bytecodeRoot(t), Out: &out}
	rows, _, err := e.bytecodeRows()
	if err != nil {
		t.Fatal(err)
	}
	latch := rows["com.tridium.kitControl.util.BBooleanLatch"]
	want := "com.tridium.kitControl.util.BLatch > javax.baja.sys.BComponent > javax.baja.sys.BComplex (not in bytecode-index.json)"
	if got := superChain(rows, latch); got != want {
		t.Errorf("superChain = %s", got)
	}
	if got := superChain(rows, rows["javax.baja.sys.BComponent"]); got != "javax.baja.sys.BComplex (not in bytecode-index.json)" {
		t.Errorf("superChain(BComponent) = %s", got)
	}
	e.printBytecodeSlots(rows, latch)
	wantSlots := `SLOTS (3 declared, 2 inherited) [bytecode]:
  property facets: BFacets = BFacets.makeBoolean()  (override)
  property out: BStatusBoolean = new BStatusBoolean()  flags rtso
  property in: BStatusBoolean = new BStatusBoolean()  flags s
  inherited from BLatch:
    property clock: BStatusBoolean = new BStatusBoolean()  flags tso
    action   latch()
`
	if out.String() != wantSlots {
		t.Errorf("slots:\n%s", out.String())
	}
}

// TestBytecodeSlotLine: an action names its parameter, return type and
// default argument; a slot known only by its field shows its kind and name.
func TestBytecodeSlotLine(t *testing.T) {
	var out bytes.Buffer
	e := &Env{BaseDir: bytecodeRoot(t), Out: &out}
	write(t, filepath.Join(e.BaseDir, "indexes", "bytecode-index.json"), `{"classes": [{"class": "a.BNet", "slots": [
	 {"name": "submit", "kind": "action", "type": "BParam", "return_type": "BOrd", "default": "new BParam()", "flag_letters": "h", "via": "annotation"},
	 {"name": "ping", "kind": "action", "via": "field"}]}]}`)
	rows, _, err := e.bytecodeRows()
	if err != nil {
		t.Fatal(err)
	}
	e.printBytecodeSlots(rows, rows["a.BNet"])
	want := "SLOTS (2 declared, 0 inherited) [bytecode]:\n" +
		"  action   submit(BParam) -> BOrd  default argument new BParam()  flags h\n" +
		"  action   ping  (field only: no slot annotation)\n"
	if out.String() != want {
		t.Errorf("slots:\n%s", out.String())
	}
}
