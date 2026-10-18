package searchdoor

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The module_nav.py fake of scriptFakes: token, rank, grep, hierarchy, implementors,
// drivers, modules and license-feature(s); FAKE_MODULE_NAV_OLD=1 plays an older module_nav
// without rank, drivers and modules (exit 2) whose not-found class exits 0.

type fakeToken struct {
	class  string
	line   int
	module string
}

var fakeTokens = map[string][]fakeToken{
	"BStatusNumeric": {{"BStatusNumeric", 12, "baja"}, {"BStatusNumeric", 40, "docSource-doc"},
		{"BNumericPoint", 7, "control-rt"}, {"BNumericPoint", 9, "control-rt"},
		{"BNumericPoint", 11, "bacnet-rt"}, {"BNumericPoint", 15, "alarm-rt"},
		{"BNumericPoint", 18, "modbus-rt"}, {"BAlarmCounter", 3, "alarm-rt"}},
	"BDeviceNetwork": {{"BDeviceNetwork", 20, "driver-rt"}, {"BBacnetNetwork", 5, "bacnet-rt"}},
	"getFooBar":      {{"BFooUser", 31, "foo-rt"}},
}

var fakeModules = [][4]string{{"baja", "rt", "1,234", "567"}, {"bacnet-rt", "rt", "880", "301"},
	{"alarm-rt", "rt", "210", "88"}, {"control-rt", "rt", "95", "40"}}

func (f *fakeRun) miss(text string) {
	f.print("%s", text)
	f.exit(1)
}

func (f *fakeRun) fail(text string) {
	if !f.done {
		f.err.WriteString("module_nav: " + text + "\n")
	}
	f.exit(2)
}

// fakeClass is the fake's cls(): "B" + re.sub(r"\W", "", word).title().replace("_", "").
func fakeClass(word string) string {
	var b strings.Builder
	for _, r := range word {
		if pycompat.IsWordRune(r) {
			b.WriteRune(r)
		}
	}
	return "B" + strings.ReplaceAll(pycompat.Title(b.String()), "_", "")
}

// unescape is re.sub(r"\\(.)", r"\1", t).
func unescape(t string) string {
	var b strings.Builder
	rs := []rune(t)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '\\' && i+1 < len(rs) && rs[i+1] != '\n' {
			i++
		}
		b.WriteRune(rs[i])
	}
	return b.String()
}

func (f *fakeRun) moduleNav(args []string) {
	old := os.Getenv("FAKE_MODULE_NAV_OLD") == "1"
	cmd, rest := "", []string(nil)
	if len(args) > 0 {
		cmd, rest = args[0], args[1:]
	}
	switch cmd {
	case "token":
		f.navToken(rest[0])
	case "rank":
		f.navRank(rest, old)
	case "grep":
		f.navGrep(rest[0])
	case "hierarchy":
		f.navHierarchy(rest, old)
	case "implementors":
		iface := rest[0]
		if iface == "BNope" {
			f.miss(fmt.Sprintf("Interface %s not found.", pycompat.StrRepr(iface)))
			return
		}
		f.print("Implementors of %s: 3 classes", iface)
		f.print("CLASS            MODULE     PACKAGE")
		f.print("%s", strings.Repeat("-", 40))
		for _, r := range [][2]string{{"BPollScheduler", "driver-rt"}, {"BBacnetPoller", "bacnet-rt"}, {"BModbusPoller", "modbus-rt"}} {
			f.print("  %s  %s  com.demo.%s", r[0], r[1], strings.Split(r[1], "-")[0])
		}
	case "drivers":
		if old {
			f.fail("argument command: invalid choice: 'drivers'")
			return
		}
		if slices.Contains(rest, "--help") {
			f.print("usage: module_nav drivers [module]")
			f.exit(0)
			return
		}
		scope := "all modules"
		if len(rest) > 0 {
			scope = rest[0]
		}
		f.print("Drivers (%s):", scope)
		f.print("  bacnet-rt   BBacnetNetwork  BBacnetDevice  BBacnetPointDeviceExt")
		f.print("  modbus-rt   BModbusNetwork  BModbusDevice  BModbusPointDeviceExt")
	case "modules":
		if old {
			f.fail("argument command: invalid choice: 'modules'")
			return
		}
		f.print("NAME        TYPE  JAVA   CLASSES")
		for _, r := range fakeModules {
			f.print("  %s  %s  %s  %s", r[0], r[1], r[2], r[3])
		}
	case "license-features":
		module := opt(rest, "--module", "")
		if module != "bacnet-rt" && module != "baja" {
			f.miss(fmt.Sprintf("0 matches: module %s checks no license feature", pycompat.StrRepr(module)))
			return
		}
		f.print("License features required by %s: 2", module)
		f.print("FEATURE  CHECKS")
		f.print("  bacnet  4")
		f.print("  mstp    1")
	case "license-feature":
		feature := rest[0]
		if feature != "bacnet" && feature != "mstp" {
			f.miss(fmt.Sprintf("0 matches for license feature %s", pycompat.StrRepr(feature)))
			return
		}
		f.print("Call sites of license feature %s: 2", pycompat.StrRepr(feature))
		f.print("CLASS           MODULE     CHECK")
		f.print("%s", strings.Repeat("-", 40))
		f.print("  BBacnetNetwork  bacnet-rt  checkFeature")
		f.print("  BMstpNetwork    bacnet-rt  getFeature")
		f.print("Try: module_nav license-features --module bacnet-rt")
	default:
		f.fail(fmt.Sprintf("unknown command %s", pycompat.StrRepr(cmd)))
	}
	f.exit(0)
}

func (f *fakeRun) navToken(word string) {
	if word == "BSlowToken" {
		f.sleep()
	}
	rows := fakeTokens[word]
	if len(rows) == 0 {
		f.miss(fmt.Sprintf("0 matches for token %s in the token index", pycompat.StrRepr(word)))
		return
	}
	classes := map[string]bool{}
	for _, r := range rows {
		classes[r.class] = true
	}
	f.print("TOKEN: %s (%d occurrences in %d classes)", pycompat.StrRepr(word), len(rows), len(classes))
	for _, r := range rows {
		f.print("  %s  %d  %s", r.class, r.line, r.module)
	}
}

func (f *fakeRun) navRank(rest []string, old bool) {
	query := rest[slices.Index(rest, "--")+1]
	words := pycompat.Fields(pycompat.Lower(query))
	switch {
	case old:
		f.fail("argument command: invalid choice: 'rank'")
		return
	case slices.Contains(words, "rankdown"):
		f.fail("no class index (run module_nav index first)")
		return
	case anyZZ(words):
		f.miss(fmt.Sprintf("0 matches for rank %s (searched 120 classes)", pycompat.StrRepr(query)))
		return
	}
	var names []string
	for _, w := range words {
		for _, suffix := range []string{"Service", "Ext"} {
			names = append(names, fakeClass(w)+suffix)
		}
	}
	if slices.Contains(pycompat.Fields(query), "BStatusNumeric") {
		names = slices.Insert(names, min(1, len(names)), "BStatusNumeric")
	}
	f.print("Rank %s: %d matching classes (terms: %s)", pycompat.StrRepr(query), len(names), strings.Join(words, ", "))
	n, _ := strconv.Atoi(opt(rest, "-n", "20"))
	for i, name := range names[:min(n, len(names))] {
		f.print("%d\t%.3f\t%s\tdemo-rt\tcom/demo/%s.java", i+1, 9.5-float64(i+1)/4, name, name)
	}
}

func (f *fakeRun) navGrep(rx string) {
	var terms []string
	for _, t := range strings.Split(strings.TrimPrefix(rx, "(?i)"), "|") {
		terms = append(terms, unescape(t))
	}
	switch {
	case slices.Contains(terms, "grepdown"):
		f.fail("grep index unreadable")
		return
	case anyZZ(terms):
		f.miss(fmt.Sprintf("0 matches for /%s/ in 40 .java files", rx))
		return
	}
	f.print("Grep /%s/: matching lines in 40 .java files", rx)
	for _, module := range []string{"alarm-rt", "history-rt"} {
		f.print("  [%s]", module)
		for j, term := range terms {
			f.print("    com.demo.%sHelper:%d uses %s here", fakeClass(term), 10+j, term)
			f.print("    com.demo.BShared:%d shared %s and %s", 30+j, term, terms[0])
		}
	}
}

func (f *fakeRun) navHierarchy(rest []string, old bool) {
	root := rest[0]
	if root == "BSlowTree" {
		f.sleep()
	}
	if root == "BNope" {
		f.print("Loading class index ...")
		f.print("Class %s not found.", pycompat.StrRepr(root))
		if old {
			f.exit(0)
		} else {
			f.exit(1)
		}
		return
	}
	kids := []string{"BChildA", "BChildB"}
	if root == "BBig" {
		kids = nil
		for i := range 65 {
			kids = append(kids, fmt.Sprintf("BChild%02d", i))
		}
	}
	f.print("Hierarchy: %s (%d direct children, %d total descendants)", root, len(kids), len(kids)+5)
	f.print("Depth limit: %s", opt(rest, "--depth", "3"))
	f.print("%s (driver-rt)", root)
	for _, k := range kids {
		f.print("|-- %s (demo-rt)", k)
	}
	f.print("|   `-- BGrandChild (demo-rt)")
	f.print("    ... 4 descendants")
}
