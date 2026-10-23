package help

import (
	"bytes"
	"strings"
	"testing"
)

func TestModuleXMLHint(t *testing.T) {
	for _, tt := range []struct{ query, want string }{
		{"kitControl-rt", "  module.xml lookups: nh module 'kitControl-rt'"},
		{"kitControl:BooleanLatch", "  module.xml lookups: nh type 'kitControl:BooleanLatch'"},
		{"BooleanLatch", "  module.xml lookups: nh type 'BooleanLatch', nh module 'BooleanLatch'"},
		{"setpoint reference", ""},
		{"doGet", ""},
		{"a-b-c", ""},
	} {
		if got := moduleXMLHint(tt.query); got != tt.want {
			t.Errorf("%q: got %q want %q", tt.query, got, tt.want)
		}
	}
}

// TestFindMissPointsAtModuleLookups: a find miss on a module-looking name
// ends with the module.xml lookups, since module.xml is not full-text indexed.
func TestFindMissPointsAtModuleLookups(t *testing.T) {
	var out bytes.Buffer
	e := &Env{BaseDir: t.TempDir(), Out: &out, Err: &out}
	if err := e.Find(FulltextParams{Query: "zzNoSuch-rt", Max: 3}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), "not proof of absence; try: find --or 'zzNoSuch-rt', class --jdk 'zzNoSuch-rt', "+
		"source-grep 'zzNoSuch-rt'\n  module.xml lookups: nh module 'zzNoSuch-rt'\n") {
		t.Errorf("got:\n%s", out.String())
	}
}
