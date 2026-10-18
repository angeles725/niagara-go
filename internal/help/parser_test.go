package help

import (
	"reflect"
	"strings"
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
	if len(registry) != len(order) || len(order) != 17 {
		t.Errorf("registry %d commands, order %d, want 17", len(registry), len(order))
	}
	for n := range retired {
		if registry[n] != nil {
			t.Errorf("%s is both registered and retired", n)
		}
	}
	if len(retired)+len(order) != 58 {
		t.Errorf("%d kept + %d retired, want the 58 niagara_help.py commands", len(order), len(retired))
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
	if got := registry["source"].helpText(); got != "Look up or view Java source code" {
		t.Errorf("plain help %q", got)
	}
}

// TestRetiredCommands: a retired command, with any arguments (even --help
// or an unknown version), prints one line naming docs/retired-commands.md
// on stderr and exits 2; --help no longer lists it.
func TestRetiredCommands(t *testing.T) {
	dir := repo(t, twoVersions, ".")
	for _, args := range [][]string{
		{"hierarchy", "BComponent"}, {"--niagara-version", "1.0", "fuzzy", "alarm"},
		{"repl"}, {"export", "--help"}, {"inherited-slots", "BX", "--niagara-version=9.9"},
	} {
		name := args[0]
		if name == "--niagara-version" {
			name = args[2]
		}
		out, errs, code := run(t, dir, args...)
		want := "nh: '" + name + "' was retired in the Go port (no recorded use); see docs/retired-commands.md\n"
		if code != 2 || out != "" || errs != want {
			t.Errorf("%v: code %d stdout %q stderr %q", args, code, out, errs)
		}
	}
	out, _, code := run(t, dir, "--niagara-version", "1.0", "--help")
	if code != 0 || strings.Contains(out, "hierarchy") || strings.Contains(out, "fuzzy") ||
		!strings.Contains(out, "freshness") {
		t.Errorf("--help (code %d) still lists a retired command or lost a kept one:\n%s", code, out)
	}
}
