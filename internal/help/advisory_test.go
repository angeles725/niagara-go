package help

import (
	"path/filepath"
	"testing"
)

// A slot list item that is not an object makes Python raise an
// AttributeError (exit 1, traceback) wherever it reads the item; nh reports
// it as a malformed index (exit 2) at the same places. The --json class view
// never reads the items, so it prints them in both.
func TestMalformedSlotItem(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "versions", "registry.json"),
		`{"default": "4.14.0.162", "versions": {"4.14.0.162": {"label": "T", "install_dir": ".", "data_root": "."}}}`)
	write(t, filepath.Join(dir, "indexes", "class-index.json"),
		`{"BBad": {"package": "x", "type": "class"}, "BOk": {"package": "x", "type": "class"}}`)
	write(t, filepath.Join(dir, "indexes", "slots-index.json"),
		`{"BOk": {"package": "x", "properties": [{"name": "fine", "type": "int"}]},
		  "BBad": {"package": "x", "properties": ["notAnObject", {"name": "ok", "type": "int"}]}}`)
	write(t, filepath.Join(dir, "indexes", "inheritance.json"), `{}`)
	const want = "ERROR: malformed slots index: \"properties\"[0] of BBad is not an object\n"
	for _, c := range []struct {
		args []string
		out  string // what both print before reaching the item
	}{
		{[]string{"slots", "BBad"}, ""}, {[]string{"slots", "ok"}, ""},
		{[]string{"slots", "--by-type", "int"}, ""}, {[]string{"slots", "BBad", "--inherited"}, ""},
	} {
		out, errs, code := run(t, dir, append([]string{"--niagara-version", "4.14.0.162"}, c.args...)...)
		if code != 2 || out != c.out || errs != want {
			t.Errorf("%v: code %d stdout %q stderr %q", c.args, code, out, errs)
		}
	}
	// Items of another class, or of a section the query skips, are not read.
	for _, args := range [][]string{{"slots", "BOk"}, {"slots", "ok", "--type", "actions"}, {"slots", "BBad", "--json"}} {
		if _, errs, code := run(t, dir, append([]string{"--niagara-version", "4.14.0.162"}, args...)...); errs != "" || code == 2 {
			t.Errorf("%v: code %d stderr %q", args, code, errs)
		}
	}
}
