package help

import (
	"reflect"
	"testing"
)

func TestOrderNamesEveryCommandOnce(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range order {
		if seen[n] {
			t.Errorf("%s listed twice in order", n)
		}
		seen[n] = true
		if registry[n] == nil {
			t.Errorf("%s in order but not registered", n)
		}
	}
	if len(registry) != len(order) || len(order) != 58 {
		t.Errorf("registry %d commands, order %d, want 58", len(registry), len(order))
	}
	if got := commandNames(); !reflect.DeepEqual(got, order) {
		t.Errorf("commandNames differs from order")
	}
}

func TestAliasHelp(t *testing.T) {
	c := command{help: "Show all slots", aliasOf: "slots --inherited"}
	if got := c.helpText(); got != "Alias of 'slots --inherited': show all slots" {
		t.Errorf("alias help %q", got)
	}
	if got := registry["export"].helpText(); got != "Export full class report to markdown file" {
		t.Errorf("plain help %q", got)
	}
}

func TestNotPortedStub(t *testing.T) {
	dir := repo(t, twoVersions, ".")
	out, errs, code := run(t, dir, "--niagara-version", "1.0", "repl")
	if code != 2 || out != "" || errs != "nh: 'repl' is not ported yet\n" {
		t.Errorf("code %d out %q err %q", code, out, errs)
	}
}
