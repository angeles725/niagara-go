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

// runAll runs every check of one station role (the labs: RoleTraining) on a station, with the
// install, the source tree and the course sources ("" = none).
func runAll(t *testing.T, stationPath, install, source, labsrc, role string) []Result {
	t.Helper()
	st, err := Open(stationPath, install, true)
	if err != nil {
		t.Fatal(err)
	}
	st.source, st.labsrc = source, labsrc
	defer st.Close()
	var out []Result
	for i := range Spec {
		if Role(Spec[i].Lab) == role {
			out = append(out, st.Run(&Spec[i]))
		}
	}
	return out
}

// mostChecks are the labs with more than 8 checks: lab 1 (C1.9-C1.12 read the dev home), lab 4 (C4.9 and C4.10 read the annotations of
// BLoopAction and BTstat), lab 10 (three file-text checks of one CSV, three of
// its history files, the job's lexicon) and
// lab 11 (its nested Tcp Config and ping-health slots are one check each).
var mostChecks = map[string]int{"1": 12, "4": 10, "10": 14, "11": 17}

// keyGroups are the ID prefixes a check of the lab may key: the lab's own items; for a review
// assignment also the needs-a-run checks X<n>; for the exam the problems P<n>.
func keyGroups(lab string) []string {
	switch {
	case strings.HasPrefix(lab, "RA"):
		return []string{lab + ".", "X"}
	case lab == "EXAM":
		return []string{"P"}
	}
	return []string{"L" + lab + "."}
}

// keyIn reports whether a key is the prefix followed by a number (X3, P2, L4.13, RA1.V10).
func keyIn(k, prefix string) bool {
	rest, ok := strings.CutPrefix(k, prefix)
	return ok && rest != "" && (strings.HasSuffix(prefix, ".") || strings.Trim(rest, "0123456789") == "")
}

// TestCheckSpecShape: unique IDs named after their lab, keys in the lab's own group, the
// fields each kind reads, 3-8 checks per lab with station state (more in mostChecks), none for
// lab 8.
func TestCheckSpecShape(t *testing.T) {
	seen := map[string]bool{}
	perLab := map[string]int{}
	typeRe := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*:[A-Z][A-Za-z0-9]+$`)
	for _, ck := range Spec {
		perLab[ck.Lab]++
		if seen[ck.ID] || !strings.HasPrefix(ck.ID, "C"+ck.Lab+".") {
			t.Errorf("%s: duplicate or not named after lab %s", ck.ID, ck.Lab)
		}
		seen[ck.ID] = true
		groups := keyGroups(ck.Lab)
		if len(ck.Keys) == 0 || ck.Desc == "" {
			t.Errorf("%s: no keys or description", ck.ID)
		}
		for k, note := range ck.Unobserved {
			if !slices.Contains(ck.Keys, k) || note == "" {
				t.Errorf("%s: unobserved note for %s is empty or %s is not one of its keys", ck.ID, k, k)
			}
		}
		for _, k := range ck.Keys {
			if !slices.ContainsFunc(groups, func(g string) bool { return keyIn(k, g) }) {
				t.Errorf("%s: key %s is outside %v", ck.ID, k, groups)
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
		case kindAudit, kindLog:
			_, err := histFields(ck.Kind, ck.Hist)
			ok = ck.Glob != "" && ck.Hist != nil && err == nil && ck.Op != "" && ck.N > 0
		case kindTrend:
			ok = ck.Glob != "" && ck.Hist != nil && ck.Hist.Max > ck.Hist.Min && ck.Op != "" && ck.N > 0
		case kindSourceText:
			_, err := regexp.Compile(ck.Match)
			ok = ck.Glob != "" && ck.Match != "" && err == nil && ck.Op != "" && ck.N > 0
		case kindAnnotation:
			ok = ck.Glob != "" && slices.Contains(annotationFacts, ck.Fact) && ck.Want != ""
		case kindFileText:
			_, err := regexp.Compile(ck.Match)
			ok = ck.Glob != "" && ck.Match != "" && err == nil && (ck.Line > 0 || (ck.Op != "" && ck.N > 0))
		}
		if !ok {
			t.Errorf("%s: kind %q lacks a field it reads", ck.ID, ck.Kind)
		}
	}
	for _, lab := range []string{"1", "2", "3", "4", "6", "7", "9", "10", "11", "B"} {
		if n, most := perLab[lab], max(8, mostChecks[lab]); n < 3 || n > most {
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
	// A link to a predicted item must say what of the prediction the check does not observe,
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
// audited as observing the whole prediction. 2026-10-17: only L10.24 (open one CSV), whose answer is
// the file content the three file-text checks read; every other predicted item a check links also
// predicts a console line, a log, a view or a runtime status.
// 2026-10-18: L1.14 (module-include.xml has the <type> of BFirstComponent), whose answer is the
// line C1.12 reads; the attribute order of the prediction is not significant in XML.
var fullyObserved = map[string]bool{"C10.8 L10.24": true, "C10.9 L10.24": true, "C10.10 L10.24": true, "C1.12 L1.14": true}

// liveStations are the real stations of each role and the checks each fails because it really
// differs from the course, with the evidence read from its config.bog and jars (2026-10-17).
var liveStations = []struct {
	role, env string
	differs   map[string]string
}{
	{RoleTraining, "COURSE_TRAINING_STATION", map[string]string{
		// Lab 2 folder holds Point1..Point4 linked to Add.in1..in4 and no Output point: step 11 of
		// Lab 3 (LI:576, link Add.Out -> Output.In10) was not done.
		"C3.4": "found 0, want at least 1",
	}},
	{RoleReview, "COURSE_REVIEW_STATION", map[string]string{
		// module.xml description "Cristian Angeles Niagara Reviw Assignments" (typo).
		"CRA1.2": "modules/Cristian_Angeles_Review-rt.jar!META-INF/module.xml lacks description=",
		// The components sit in Config/Cristian_Angeles_Review, not in the folder Review (RA1.V10).
		"CRA1.10": "found 0 in Review (2 in total), want at least 1; elsewhere: Cristian_Angeles_Review/ComponentWithId",
	}},
	{RoleCert, "COURSE_CERT_STATION", map[string]string{}},
}

// TestCheckLiveStations runs the checks of each role against its live station (the variable of
// the role) and, with COURSE_NIAGARA_INSTALL, the built jars; a role whose variable is unset skips.
func TestCheckLiveStations(t *testing.T) {
	for _, live := range liveStations {
		t.Run(live.role, func(t *testing.T) {
			path := os.Getenv(live.env)
			if path == "" {
				t.Skipf("%s is not set: the live %s station is not checked", live.env, live.role)
			}
			install, source, labsrc := os.Getenv("COURSE_NIAGARA_INSTALL"), os.Getenv("COURSE_NIAGARA_SOURCE"), os.Getenv("COURSE_LAB_SOURCES")
			got := map[string]string{}
			results := runAll(t, path, install, source, labsrc, live.role)
			if len(results) == 0 {
				// A role with no rows would pass without checking anything.
				t.Fatalf("no check runs on the %s station: the spec has no row of this role", live.role)
			}
			for _, r := range results {
				switch {
				case r.Verdict == Skip && install == "" && (strings.HasPrefix(r.Kind, "jar") || r.Root == rootInstall):
				case r.Verdict == Skip && source == "" && r.Kind == kindSourceText:
				case r.Verdict == Skip && labsrc == "" && (r.Kind == kindAnnotation || r.Root == rootLabsrc):
				case r.Verdict == Fail && live.differs[r.ID] != "":
					got[r.ID] = r.Evidence
				case r.Verdict != Pass:
					t.Errorf("%s %s: %s", r.ID, r.Verdict, r.Evidence)
				}
			}
			for id, ev := range got {
				if !strings.HasPrefix(ev, live.differs[id]) {
					t.Errorf("%s: evidence %q, want it to start with %q", id, ev, live.differs[id])
				}
				got[id] = live.differs[id]
			}
			want := live.differs
			if install == "" { // the jar differences are not checked
				want = map[string]string{}
				for id, ev := range live.differs {
					if ck := specRow(id); ck != nil && !strings.HasPrefix(ck.Kind, "jar") {
						want[id] = ev
					}
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("known station differences: got %v, want %v", got, want)
			}
		})
	}
}

func specRow(id string) *Check {
	for i := range Spec {
		if Spec[i].ID == id {
			return &Spec[i]
		}
	}
	return nil
}
