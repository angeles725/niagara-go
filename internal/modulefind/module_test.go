package modulefind

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestModuleScan pins what the scanner extracts from the fixture tree (the values the
// Python golden module-find.json prints for slots, actions, extends, ords and writers).
func TestModuleScan(t *testing.T) {
	m := newModule("testdata/srcA")
	if m.files != 4 || len(m.unreadable) != 0 {
		t.Fatalf("files %d, unreadable %v", m.files, m.unreadable)
	}
	var slots []string
	for _, cls := range m.slots.sortedKeys() {
		s, _ := m.slots.get(cls)
		for _, name := range s.keys {
			slots = append(slots, cls+"."+name+"<"+s.m[name].typ+">"+s.m[name].flags)
		}
	}
	want := []string{"BColdRoom.setpoint<baja:StatusNumeric>Flags.SUMMARY | Flags.OPERATOR",
		"BColdRoom.mode<baja:String>Flags.OPERATOR", "BRoomPanel.temp<BStatusNumeric>Flags.OPERATOR",
		"BRoomPanel.label<baja:String>Flags.READONLY | Flags.SUMMARY",
		"BRoomPanel.cache<baja:Integer>Flags.TRANSIENT | Flags.HIDDEN", "BRoomPanel.untyped<>"}
	if !reflect.DeepEqual(slots, want) {
		t.Errorf("slots = %q", slots)
	}
	if sp, _ := m.slots.get("BColdRoom"); sp.m["setpoint"].min == nil || *sp.m["setpoint"].min != -40.0 {
		t.Errorf("MIN facet not parsed")
	}
	acts, _ := m.actions.get("BColdRoom")
	if got := strings.Join(acts.keys, ",") + "=" + acts.m["reset"]; got != "applySetpoint,defrost,reset=h" {
		t.Errorf("actions = %s", got)
	}
	if got := strings.Join(m.resolveChain("BRoomPanel"), " -> "); got != "BRoomPanel -> BColdRoom -> BComponent" {
		t.Errorf("chain = %s", got)
	}
	if got := len(m.ords); got != 5 || m.ords[0] != [2]string{"BColdRoom", "station:|slot:/Drivers"} {
		t.Errorf("ords = %q", m.ords)
	}
	if got := sortedSet(m.writers["mode"]); !reflect.DeepEqual(got, []string{`BColdRoom: .set("mode", ...)`, "Helper: setMode(...)"}) {
		t.Errorf("writers[mode] = %q", got)
	}
	if !m.dynWriters["BDashServlet"]["BDashServlet: comp.set(prop, ...)  [slot resolved at runtime]"] {
		t.Errorf("dynamic writer missing: %v", m.dynWriters)
	}
}

func TestPyFloat(t *testing.T) {
	for in, ok := range map[string]bool{"-40.0": true, "1.": true, ".5": true, "1.2.3": false, ".": false} {
		if _, got := pyFloat(in); got != ok {
			t.Errorf("pyFloat(%q) ok = %v", in, got)
		}
	}
}

// TestWalkJavaJoinsForTheHost: java_lines builds each path with os.path.join over os.walk,
// so on Windows a nested file is "root\\pkg\\X.java" ("root" spelled as given).
func TestWalkJavaJoinsForTheHost(t *testing.T) {
	var got []string
	walkJava("testdata/srcA", func(path, _ string) { got = append(got, path) })
	if len(got) == 0 {
		t.Fatal("no .java files under testdata/srcA")
	}
	sep := "/"
	if runtime.GOOS == "windows" {
		sep = `\`
	}
	for _, p := range got {
		rest := strings.TrimPrefix(p, "testdata/srcA"+sep)
		if rest == p || strings.Contains(rest, map[string]string{"/": `\`, `\`: "/"}[sep]) {
			t.Errorf("path %q is not os.path.join(%q, ...) for %s", p, "testdata/srcA", runtime.GOOS)
		}
	}
}
