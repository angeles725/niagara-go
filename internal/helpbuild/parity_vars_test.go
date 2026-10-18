package helpbuild

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// realParityVar is the umbrella switch of every test that reads a real
// install or the real corpus (minutes of work, so off by default, -short or
// not); the finer HELPBUILD_PARITY_* variables of a test also turn it on.
const realParityVar = "HELPBUILD_PARITY_REAL"

// paritySwitches are the real-data parity variables that are switches; the
// other HELPBUILD_PARITY_* variables hold a path or a path list.
var paritySwitches = map[string]bool{realParityVar: true, "HELPBUILD_PARITY_FULL": true, "HELPBUILD_PARITY_415": true}

// parityVar reports whether the real-data parity variable name is on: a
// switch when set to "1", a value variable when not empty. A switch set to
// anything else is an error naming the variable and the accepted value.
func parityVar(name string) (bool, error) {
	v := os.Getenv(name)
	if !paritySwitches[name] {
		return v != "", nil
	}
	switch v {
	case "":
		return false, nil
	case "1":
		return true, nil
	}
	return false, fmt.Errorf("%s=%q: the accepted value is 1 (unset turns it off)", name, v)
}

// parityOn is parityVar failing t on a malformed switch.
func parityOn(t *testing.T, name string) bool {
	t.Helper()
	on, err := parityVar(name)
	if err != nil {
		t.Fatal(err)
	}
	return on
}

// needRealParity skips t, naming the variables, unless
// HELPBUILD_PARITY_REAL=1 or one of the finer variables is on; every one of
// them is parsed first, so a malformed value fails instead of skipping.
func needRealParity(t *testing.T, finer ...string) {
	t.Helper()
	on := false
	names := realParityVar + "=1"
	for _, v := range append([]string{realParityVar}, finer...) {
		on = parityOn(t, v) || on
		if v != realParityVar {
			names += " or " + v
		}
	}
	if !on {
		t.Skipf("real-data parity runs only on request: set %s", names)
	}
}

// guidesRealVersions are the versions TestParityGuidesReal builds:
// 4.3.58.18 with HELPBUILD_PARITY_REAL=1, 4.15.6.20 with
// HELPBUILD_PARITY_415=1; none is an error, never an empty pass.
func guidesRealVersions() ([]string, error) {
	var vids []string
	for _, c := range [][2]string{{realParityVar, "4.3.58.18"}, {"HELPBUILD_PARITY_415", "4.15.6.20"}} {
		on, err := parityVar(c[0])
		if err != nil {
			return nil, err
		}
		if on {
			vids = append(vids, c[1])
		}
	}
	if len(vids) == 0 {
		return nil, fmt.Errorf("no version to compare: set %s=1 (4.3.58.18) or HELPBUILD_PARITY_415=1 (4.15.6.20)", realParityVar)
	}
	return vids, nil
}

// A switch counts only when set to "1"; any other value fails naming the
// variable and the accepted value, so a typo never turns a test into a
// vacuous pass. A value variable (a path or a list) counts when non-empty.
func TestRealParityVars(t *testing.T) {
	for _, c := range []struct {
		name, value string
		on          bool
		err         string
	}{
		{realParityVar, "", false, ""},
		{realParityVar, "1", true, ""},
		{realParityVar, "true", false, realParityVar + `="true": the accepted value is 1`},
		{"HELPBUILD_PARITY_415", "0", false, `HELPBUILD_PARITY_415="0": the accepted value is 1`},
		{"HELPBUILD_PARITY_FULL", "yes", false, `HELPBUILD_PARITY_FULL="yes": the accepted value is 1`},
		{"HELPBUILD_PARITY_JARS", "a.jar", true, ""},
		{"HELPBUILD_PARITY_COURSE", "", false, ""},
	} {
		t.Setenv(c.name, c.value)
		on, err := parityVar(c.name)
		if on != c.on || (err == nil) != (c.err == "") || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s=%q: on %v, err %v; want on %v, err %q", c.name, c.value, on, err, c.on, c.err)
		}
	}
}

// TestParityGuidesReal never runs with no version to compare.
func TestGuidesRealVersions(t *testing.T) {
	for _, c := range []struct {
		real, v415 string
		want       string
		err        string
	}{
		{"1", "", "4.3.58.18", ""},
		{"", "1", "4.15.6.20", ""},
		{"1", "1", "4.3.58.18 4.15.6.20", ""},
		{"", "true", "", `HELPBUILD_PARITY_415="true"`},
		{"", "", "", "no version to compare"},
	} {
		t.Setenv(realParityVar, c.real)
		t.Setenv("HELPBUILD_PARITY_415", c.v415)
		vids, err := guidesRealVersions()
		if got := strings.Join(vids, " "); got != c.want || (err == nil) != (c.err == "") || err != nil && !strings.Contains(err.Error(), c.err) {
			t.Errorf("REAL=%q 415=%q: %q, %v; want %q, %q", c.real, c.v415, got, err, c.want, c.err)
		}
	}
}

// An explicit HELPBUILD_PARITY_JARS is used whole, empty entries dropped;
// a list with no jar is an error.
func TestRealJarsList(t *testing.T) {
	t.Setenv("HELPBUILD_PARITY_JARS", "")
	if jars, explicit, err := realJars(); explicit || err != nil || len(jars) != 6 {
		t.Errorf("default: %q %v %v", jars, explicit, err)
	}
	sep := string(os.PathListSeparator)
	t.Setenv("HELPBUILD_PARITY_JARS", "a.jar"+sep+sep+"b.jar"+sep)
	if jars, explicit, err := realJars(); !explicit || err != nil || strings.Join(jars, " ") != "a.jar b.jar" {
		t.Errorf("explicit: %q %v %v", jars, explicit, err)
	}
	t.Setenv("HELPBUILD_PARITY_JARS", sep+sep)
	if _, _, err := realJars(); err == nil || !strings.Contains(err.Error(), "HELPBUILD_PARITY_JARS") {
		t.Errorf("empty list: %v", err)
	}
}
