package check

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/course/key"
)

const goodStation = "../testdata/stations/good"

func runAll(t *testing.T, stationPath, install string) []Result {
	t.Helper()
	st, err := Open(stationPath, install, true)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var out []Result
	for i := range Spec {
		out = append(out, st.Run(&Spec[i]))
	}
	return out
}

// TestCheckSpecShape: unique IDs named after their lab, keys in the lab's own group, the
// fields each kind reads, 3-8 checks per lab with station state (17 for lab 11, whose nested
// Tcp Config and ping-health slots are one check each), none for lab 8.
func TestCheckSpecShape(t *testing.T) {
	seen := map[string]bool{}
	perLab := map[string]int{}
	typeRe := regexp.MustCompile(`^[a-zA-Z]+:[A-Z][A-Za-z]+$`)
	for _, ck := range Spec {
		perLab[ck.Lab]++
		if seen[ck.ID] || !strings.HasPrefix(ck.ID, "C"+ck.Lab+".") {
			t.Errorf("%s: duplicate or not named after lab %s", ck.ID, ck.Lab)
		}
		seen[ck.ID] = true
		group := "L" + ck.Lab + "."
		if len(ck.Keys) == 0 || ck.Desc == "" {
			t.Errorf("%s: no keys or description", ck.ID)
		}
		for k, note := range ck.Unobserved {
			if !slices.Contains(ck.Keys, k) || note == "" {
				t.Errorf("%s: unobserved note for %s is empty or %s is not one of its keys", ck.ID, k, k)
			}
		}
		for _, k := range ck.Keys {
			if !strings.HasPrefix(k, group) {
				t.Errorf("%s: key %s is outside %s", ck.ID, k, group)
			}
		}
		var ok bool
		switch ck.Kind {
		case kindType:
			ok = typeRe.MatchString(ck.Type) && (ck.Within == "" || typeRe.MatchString(ck.Within)) && ck.Op != "" && ck.N > 0
		case kindSlot:
			ok = typeRe.MatchString(ck.Type) && ck.Slot != "" && (ck.Set || ck.Want != "" || (ck.Distinct && ck.N > 0))
		case kindLink:
			ok = typeRe.MatchString(ck.Type) && typeRe.MatchString(ck.From) && ck.Slot != "" && ck.FromSlot != "" && ck.N > 0
		case kindFiles:
			ok = ck.Glob != "" && ck.N > 0
		case kindJar, kindJarSigned:
			ok = ck.Jar != ""
		case kindJarEntry:
			ok = ck.Jar != "" && ck.Glob != "" && ck.N > 0
		case kindJarText:
			ok = ck.Jar != "" && ck.Glob != "" && len(ck.Contains) > 0
		}
		if !ok {
			t.Errorf("%s: kind %q lacks a field it reads", ck.ID, ck.Kind)
		}
	}
	for _, lab := range []string{"1", "2", "3", "4", "6", "7", "9", "10", "11", "B"} {
		if n, most := perLab[lab], map[bool]int{true: 17, false: 8}[lab == "11"]; n < 3 || n > most {
			t.Errorf("lab %s has %d checks, want 3-%d", lab, n, most)
		}
	}
	if perLab["5"] == 0 || perLab["8"] != 0 {
		t.Errorf("lab 5 has %d checks (want jar checks), lab 8 has %d (want none)", perLab["5"], perLab["8"])
	}
}

// TestCheckKeysReal: every linked key item exists in the real answer key.
func TestCheckKeysReal(t *testing.T) {
	root := os.Getenv(key.RootEnv)
	if root == "" || !key.HasMarker(root) {
		t.Skipf("%s is not a niagara-research checkout: the check keys are not resolved", key.RootEnv)
	}
	c, problems := key.Load(root)
	if c == nil {
		t.Fatalf("%v", problems)
	}
	for _, ck := range Spec {
		for _, k := range ck.Keys {
			if c.Lookup(k) == nil {
				t.Errorf("%s: key %s is not an item of %s", ck.ID, k, key.LabsFile)
			}
		}
	}
	// A link to a predicted item must say what of the Prediction the check does not observe,
	// unless it was audited as observing all of it: the answer key would be marked verified from a
	// CONFIRMED verdict. The links are resolved as verify-predictions resolves them (itemLinks).
	links := itemLinks(c, Spec)
	for _, it := range c.Items {
		for _, l := range links[it] {
			if it.HasStatus("predicted") && len(l.Notes) == 0 && !fullyObserved[l.Check.ID+" "+it.ID] {
				t.Errorf("%s: links predicted %s (%s:%d): add an Unobserved note or audit it into fullyObserved",
					l.Check.ID, it.ID, it.File, it.Line)
			}
		}
	}
}

// fullyObserved lists the "check item" links to a predicted item of the real answer key that were
// audited as observing the whole Prediction (2026-10-17: none; every predicted item a check links
// also predicts a console line, a log, a view or a runtime status).
var fullyObserved = map[string]bool{}

// stationDiffers lists the checks the live TrainingStation fails because it really differs
// from the lab, with the evidence read from its config.bog (2026-10-17).
var stationDiffers = map[string]string{
	// Lab 2 folder holds Point1..Point4 linked to Add.in1..in4 and no Output point: step 11 of
	// Lab 3 (LI:576, link Add.Out -> Output.In10) was not done.
	"C3.4": "found 0, want at least 1",
}

// TestCheckTrainingStation runs every check against the live course station
// (COURSE_TRAINING_STATION) and, with COURSE_NIAGARA_INSTALL, the built jars.
func TestCheckTrainingStation(t *testing.T) {
	path := os.Getenv("COURSE_TRAINING_STATION")
	if path == "" {
		t.Skip("COURSE_TRAINING_STATION is not set: the live TrainingStation is not checked")
	}
	install := os.Getenv("COURSE_NIAGARA_INSTALL")
	got := map[string]string{}
	for _, r := range runAll(t, path, install) {
		switch {
		case r.Verdict == Skip && install == "" && strings.HasPrefix(r.Kind, "jar"):
		case r.Verdict == Fail && stationDiffers[r.ID] != "":
			got[r.ID] = r.Evidence
		case r.Verdict != Pass:
			t.Errorf("%s %s: %s", r.ID, r.Verdict, r.Evidence)
		}
	}
	want := map[string]string{}
	for id, ev := range stationDiffers {
		want[id] = ev
	}
	for id, ev := range got {
		if !strings.HasPrefix(ev, want[id]) {
			t.Errorf("%s: evidence %q, want it to start with %q", id, ev, want[id])
		}
		got[id] = want[id]
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("known station differences: got %v, want %v", got, want)
	}
}
