package help

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestFnmatchMatchesPython replays testdata/fnmatch.json, computed with
// Python 3.15 fnmatch.fnmatch: for each pattern, one "1"/"0" per name says
// whether it matches. The patterns reach every _translate branch (sets,
// negation, empty and trailing ranges, unclosed "[", escapes, non-ASCII).
func TestFnmatchMatchesPython(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "fnmatch.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Names   []string    `json:"names"`
		Matches [][2]string `json:"matches"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	if len(fx.Matches) == 0 {
		t.Fatal("no fnmatch rows")
	}
	for _, m := range fx.Matches {
		pat, bits := m[0], m[1]
		if len(bits) != len(fx.Names) {
			t.Fatalf("pattern %q: %d results for %d names", pat, len(bits), len(fx.Names))
		}
		for i, name := range fx.Names {
			if got, want := fnmatch(name, pat), bits[i] == '1'; got != want {
				t.Errorf("fnmatch(%q, %q) = %v, want %v", name, pat, got, want)
			}
		}
	}
}

func TestSuggestMatchesPython(t *testing.T) {
	names := []string{"BAlarmService", "BAlarmServices", "BAlarmSrvice", "BAlarmServise", "balarmservice", "BAlarm", "X"}
	want := []string{"BAlarmService", "balarmservice", "BAlarmSrvice", "BAlarmServices", "BAlarmServise"}
	if got := suggest(names, "BAlarmSevice", 5); !reflect.DeepEqual(got, want) {
		t.Errorf("suggest = %q, want %q", got, want)
	}
	if got := suggest(names, "BAlarmSevice", 2); !reflect.DeepEqual(got, want[:2]) {
		t.Errorf("suggest n=2 = %q", got)
	}
	sims := map[string]float64{"balarmservice": 0.9230769230769231, "balarmservices": 0.8571428571428572,
		"balarmsrvice": 0.9166666666666666, "balarm": 0, "x": 0}
	for name, want := range sims {
		if got := typoSimilarity("balarmsevice", name); got != want {
			t.Errorf("typoSimilarity(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestPythonFormatting(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 6310: "6,310", -1234567: "-1,234,567"} {
		if got := thousands(n); got != want {
			t.Errorf("thousands(%d) = %q, want %q", n, got, want)
		}
	}
	if got := ljust("é", 3) + "|" + rjust("é", 3) + "|" + headRunes("añbc", 2); got != "é  |  é|añ" {
		t.Errorf("padding by code points: %q", got)
	}
	if got := strip("\x1c\t x \x1f"); got != "x" {
		t.Errorf("strip: %q", got)
	}
}
