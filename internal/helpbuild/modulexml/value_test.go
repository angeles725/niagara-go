package modulexml

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestValueIsTheIndexRecord: the module-index.json record keeps every list in
// document order and an agent's optional attributes only when set.
func TestValueIsTheIndexRecord(t *testing.T) {
	m, err := Parse([]byte(kitControlUX))
	if err != nil {
		t.Fatal(err)
	}
	got := pyjson.Dumps(m.Value(), pyjson.Defaults(-1))
	want := `{"name": "kitControl-ux", "module": "kitControl", "runtime_profile": "ux", "vendor": "Tridium", ` +
		`"version": "4.15.6.20", "description": "Kit Control UX", ` +
		`"dependencies": [{"name": "baja", "vendor": "Tridium", "version": "4.15.6"}, ` +
		`{"name": "kitControl-rt", "vendor": "Tridium", "version": "4.15.6"}], ` +
		`"types": [{"name": "KitControlJsBuild", "class": "com.tridium.kitControl.ux.BKitControlJsBuild"}, ` +
		`{"name": "LatchTypeExt", "class": "com.tridium.kitControl.ux.BLatchTypeExt"}], ` +
		`"agents": [{"type": "LatchTypeExt", "class": "com.tridium.kitControl.ux.BLatchTypeExt", ` +
		`"on": ["kitControl:BooleanLatch", "kitControl:NumericLatch"], "app": "webShell", "required_permissions": "r"}], ` +
		`"java_permissions": [{"profile": "station", "class": "java.lang.RuntimePermission", "name": "setIO", "action": ""}, ` +
		`{"profile": "station", "class": "java.io.FilePermission", "name": "${/}x", "action": "read"}], ` +
		`"defs": [{"name": "kit.icon", "value": "module://kitControl/icon.png"}], ` +
		`"adapters": [{"type": "LatchTypeExt", "from": "baja:RelTime", "to": "control:TriggerMode"}]}`
	if got != want {
		t.Errorf("value:\n got %s\nwant %s", got, want)
	}
}
