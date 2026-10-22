package obixnav

import (
	"bytes"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Expected values are Python 3 repr()/int() output.
func TestPyStrRepr(t *testing.T) {
	cases := map[string]string{"abc": `'abc'`, "it's": `"it's"`, `a"b`: `'a"b'`, `both ' "`: `'both \' "'`,
		"tab\there": `'tab\there'`, "é": `'é'`, "\x00\x7f": `'\x00\x7f'`, "\u2028": `'\u2028'`,
		"\U0001F600": "'\U0001F600'", `back\slash`: `'back\\slash'`}
	for in, want := range cases {
		if got := pycompat.StrRepr(in); got != want {
			t.Errorf("pycompat.StrRepr(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestPyInt(t *testing.T) {
	for in, want := range map[string]int{" 1 ": 1, "+1": 1, "1_0": 10, "-3": -3} {
		if got, err := pyInt(in); err != nil || got != want {
			t.Errorf("pyInt(%q) = %d, %v", in, got, err)
		}
	}
	for _, in := range []string{"x", "1__0", "", "0x1"} {
		if _, err := pyInt(in); err == nil {
			t.Errorf("pyInt(%q) accepted", in)
		}
	}
}

func TestRtSecs(t *testing.T) {
	str := func(s string) *string { return &s }
	for in, want := range map[string]float64{"PT8H0S": 28800, "PT30M0S": 1800, "PT5S": 5, "PT1.5M": 90, "PTH": 0} {
		if got, ok, err := rtSecs(str(in)); err != nil || !ok || got != want {
			t.Errorf("rtSecs(%q) = %v, %v, %v; want %v", in, got, ok, err, want)
		}
	}
	if _, ok, _ := rtSecs(str("8h")); ok {
		t.Error("a non-PT value must not parse")
	}
	if _, _, err := rtSecs(str("PT1.2.3S")); err == nil {
		t.Error("PT1.2.3S must be a ValueError")
	}
}

// A missing pass file is an input error (exit 2), not "searched, no match" (exit 1).
func TestMissingPassFileIsExit2(t *testing.T) {
	var out, errb bytes.Buffer
	code := Run(Options{PassFile: "testdata/inputs/absent", Command: "cond"}, cli.Stdio{Out: &out, Err: &errb})
	if code != cli.ExitCannotSearch || !strings.HasPrefix(errb.String(), "No pass file: ") {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
}

// The room key is read once: an int from --room (map, wiring, config), a string from the dump
// positional; neither leaks into the other field.
func TestParseRoomKey(t *testing.T) {
	cases := []struct {
		args []string
		room int
		dump string
	}{
		{[]string{"map", "--room", "3"}, 3, ""},
		{[]string{"wiring"}, 0, ""},
		{[]string{"config", "--room", "2"}, 2, ""},
		{[]string{"dump", "4"}, 0, "4"},
		{[]string{"dump", "x"}, 0, "x"},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		o, code, done := parse(c.args, cli.Stdio{Out: &out, Err: &errb})
		if done || o.Room != c.room || o.DumpRoom != c.dump {
			t.Errorf("%q: Room %d DumpRoom %q (done %v code %d %s), want %d %q",
				c.args, o.Room, o.DumpRoom, done, code, errb.String(), c.room, c.dump)
		}
	}
}

// --help describes every subcommand, and each subcommand's --help says what it does; parsing
// help never connects to a station.
func TestHelpDescribesEverySubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"--help"}, cli.Stdio{Out: &out, Err: &errb}); code != 0 {
		t.Fatalf("--help exit %d: %s", code, errb.String())
	}
	for _, name := range []string{"map", "cond", "wiring", "config", "read", "dump"} {
		found := false
		for _, line := range strings.Split(out.String(), "\n") {
			if f := strings.Fields(line); len(f) > 1 && f[0] == name {
				found = true
			}
		}
		if !found {
			t.Errorf("--help has no description line for %q:\n%s", name, out.String())
		}
		var sout, serr bytes.Buffer
		if code := Main([]string{name, "--help"}, cli.Stdio{Out: &sout, Err: &serr}); code != 0 {
			t.Fatalf("%s --help exit %d: %s", name, code, serr.String())
		}
		if parts := strings.SplitN(sout.String(), "\n\n", 3); len(parts) < 3 || strings.HasPrefix(parts[1], "positional") || strings.HasPrefix(parts[1], "options") {
			t.Errorf("%s --help has no description paragraph:\n%s", name, sout.String())
		}
	}
}
