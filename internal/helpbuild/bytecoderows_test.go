package helpbuild

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/classfile"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestBytecodeSlotRow: a slot row carries its flags as javac's decimal number
// and as Slot Sheet letters (none when the flags element is omitted), and is
// counted by how the slot was found.
func TestBytecodeSlotRow(t *testing.T) {
	x := &bytecodeIndex{}
	for _, c := range []struct {
		slot classfile.Slot
		want string
	}{
		{classfile.Slot{Kind: "property", Name: "out", Type: "BStatusBoolean", Default: "new BStatusBoolean()", Flags: "5", Via: "annotation"},
			`{"name": "out", "kind": "property", "type": "BStatusBoolean", "return_type": "", "default": "new BStatusBoolean()", "flags": "5", "flag_letters": "rh", "via": "annotation"}`},
		{classfile.Slot{Kind: "action", Name: "reset", Type: "BBoolean", ReturnType: "BValue", Via: "annotation"},
			`{"name": "reset", "kind": "action", "type": "BBoolean", "return_type": "BValue", "default": "", "flags": "", "flag_letters": "", "via": "annotation"}`},
		{classfile.Slot{Kind: "topic", Name: "fired", Via: "field"},
			`{"name": "fired", "kind": "topic", "type": "", "return_type": "", "default": "", "flags": "", "flag_letters": "", "via": "field"}`},
	} {
		if got := pyjson.Dumps(x.slotRow(c.slot), pyjson.Defaults(-1)); got != c.want {
			t.Errorf("slotRow(%s) = %s", c.slot.Name, got)
		}
	}
	if x.byAnnotation != 2 || x.byField != 1 {
		t.Errorf("by annotation %d, by field %d", x.byAnnotation, x.byField)
	}
}
