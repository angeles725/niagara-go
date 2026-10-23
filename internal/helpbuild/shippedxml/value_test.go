package shippedxml

import (
	"testing"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

var compact = pyjson.DumpOpts{Indent: -1, ItemSep: ", ", KeySep: ": "}

func TestPermissionsValue(t *testing.T) {
	p, err := ParsePermissions("rc/module-permissions.xml", []byte(groupsTemplate))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"path": "rc/module-permissions.xml", "java_permissions": [], "groups": [` +
		`{"type": "all", "requests": []}, {"type": "station", "requests": [` +
		`{"name": "NETWORK_COMMUNICATION", "purpose_key": "Outside access for Driver", "parameters": [` +
		`{"name": "hosts", "value": "127.0.0.1"}, {"name": "ports", "value": "*"}], "commented": true}, ` +
		`{"name": "FILE_ACCESS", "purpose_key": "Logs", "parameters": [], "commented": false}]}], "velocity": []}`
	if got := pyjson.Dumps(p.Value(), compact); got != want {
		t.Errorf("value:\n%s\nwant:\n%s", got, want)
	}
	j, _ := ParsePermissions("t/module-permissions.xml", []byte(javaTemplate))
	if got := pyjson.Dumps(j.Value(), compact); got != `{"path": "t/module-permissions.xml", "java_permissions": [`+
		`{"profile": "station", "class": "java.io.FilePermission", "name": "${niagara.user.home}${/}logging", "action": "read,write"}, `+
		`{"profile": "all", "class": "java.util.logging.LoggingPermission", "name": "control", "action": ""}], "groups": [], "velocity": []}` {
		t.Errorf("java value: %s", got)
	}
}

func TestSchemaValue(t *testing.T) {
	s, _ := ParseSchema("rsc/obix.xsd", []byte(obixXSD))
	want := `{"path": "rsc/obix.xsd", "kind": "xsd", "root": "schema", "target_namespace": "http://obix.org/ns/schema/1.0", "elements": 2}`
	if got := pyjson.Dumps(s.Value(), compact); got != want {
		t.Errorf("value: %s", got)
	}
}
