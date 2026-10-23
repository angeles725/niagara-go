package help

// The index facts of selftest: one stable fact per lookup kind, checked on the
// selected version's real indexes and text sources. Each holds on every
// registered Niagara version (4.3, 4.13, 4.14, 4.15), so a FAIL is a broken
// index or reader, not a version difference.

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// indexFact is one selftest fact: what it needs (a text source label or an
// index file), the lookup it runs, and the test of that lookup's output.
type indexFact struct {
	name, needs, claim string
	run                func(sub *Env) error
	holds              func(out string) bool
}

func contains(want string) func(string) bool {
	return func(out string) bool { return strings.Contains(out, want) }
}

// firstHit is the first "[label] title" row of label in a find report.
func firstHit(out, label string) string {
	for _, line := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "["+label+"] ") {
			return t
		}
	}
	return ""
}

func findFact(query string) func(sub *Env) error {
	return func(sub *Env) error { return sub.Find(FulltextParams{Query: query, Max: 3, Raised: true}) }
}

var indexFacts = []indexFact{
	{"lexicon", "lexicon-text", "find 'Alarm Popup Always On Top' ranks lexicon alarm-rt first among lexicon hits",
		findFact("Alarm Popup Always On Top"),
		func(out string) bool { return firstHit(out, "lexicon-text") == "[lexicon-text] alarm-rt" }},
	{"course", "course-text", "find 'course logistics' finds the course's Course Logistics deck",
		findFact("course logistics"), contains("[course-text] Course: 0 Course Logistics")},
	{"package", "package-text", "package 'javax.baja.sys' prints its package-text description",
		func(sub *Env) error { return sub.Package(PackageParams{Name: "javax.baja.sys"}) },
		contains("(package-text/javax.baja.sys.txt)")},
	{"class", "class-index.json", "class 'BComponent' is in javax.baja.sys",
		func(sub *Env) error { return sub.Class(ClassParams{Name: "BComponent"}) },
		contains("PACKAGE: javax.baja.sys\n")},
	{"slots", "slots-index.json", "slots 'BAlarmService' lists defaultAlarmClass: BAlarmClass",
		func(sub *Env) error { return sub.Slots(SlotsParams{Query: "BAlarmService"}) },
		contains("defaultAlarmClass: BAlarmClass")},
	{"method", "method-index.json", "method 'ackAlarm' lists javax.baja.alarm.BAlarmRecord.ackAlarm()",
		func(sub *Env) error { return sub.Method(MethodParams{Name: "ackAlarm"}) },
		contains("[methods] javax.baja.alarm.BAlarmRecord\n    void ackAlarm()\n")},
	{"module", "module-index.json", "module 'kitControl-rt' lists its dependency on baja",
		func(sub *Env) error { return sub.Module(ModuleParams{Name: "kitControl-rt"}) },
		func(out string) bool {
			return strings.HasPrefix(out, "MODULE: kitControl-rt\n") && strings.Contains(out, "\n  baja ")
		}},
}

// missingNeed is why fact f cannot be checked in this version ("" when it can):
// its text source is absent, or its index is absent or empty.
func (e *Env) missingNeed(f indexFact) (string, error) {
	if !strings.HasSuffix(f.needs, ".json") {
		if fts.DirOf(fts.SourceDirs(e.BaseDir), f.needs) == "" {
			return f.needs + "/ is absent in this version", nil
		}
		return "", nil
	}
	if !pycompat.IsFile(index.Path(e.BaseDir, f.needs)) {
		return "indexes/" + f.needs + " is absent in this version", nil
	}
	o, err := e.loadIndex(f.needs, false)
	if err != nil {
		return "", err
	}
	if o.Len() == 0 {
		return "indexes/" + f.needs + " is empty in this version", nil
	}
	return "", nil
}

// checkFacts runs every index fact: PASS and SKIP lines on stdout, the failures
// (a lookup that errs included) returned for selftest to print with its own;
// skipped counts the SKIPs. err is only an index that cannot be read.
func (e *Env) checkFacts() (fails []string, skipped int, err error) {
	for _, f := range indexFacts {
		why, err := e.missingNeed(f)
		if err != nil {
			return nil, 0, err
		}
		if why != "" {
			fmt.Fprintf(e.Out, "  SKIP  %s: %s\n", f.name, why)
			skipped++
			continue
		}
		out, err := e.captured(f.run)
		switch _, isExit := err.(ExitCode); {
		case err != nil && !isExit:
			fails = append(fails, fmt.Sprintf("%s: %s failed: %v", f.name, f.claim, err))
		case f.holds(out):
			fmt.Fprintf(e.Out, "  PASS  %s: %s\n", f.name, f.claim)
		default:
			fails = append(fails, fmt.Sprintf("%s: expected: %s", f.name, f.claim))
		}
	}
	return fails, skipped, nil
}
