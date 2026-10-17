package help

// Recall oracle (recall.py, `selftest --recall`): every item the raw sources
// declare must be found by the resolver of the command that looks it up.
// Rows report expected / found / missed with the first misses; a row with
// a miss fails the run (exit 1). find and fuzzy scan the whole corpus per
// query, so they run on a fixed, evenly spaced sample.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// sortStable is sorted() with a less function (a stable sort).
func sortStable[T any](s []T, less func(a, b T) bool) {
	sort.SliceStable(s, func(i, j int) bool { return less(s[i], s[j]) })
}

const (
	recallFirstMisses = 10 // FIRST_MISSES
	recallSample      = 12 // SAMPLE
)

// recallEnumRoots are ENUM_ROOTS (and enum_search's enum parents).
var recallEnumRoots = map[string]bool{"BFrozenEnum": true, "BDynamicEnum": true, "BEnum": true, "BBitString": true}

// recallRow is one row of the table.
type recallRow struct {
	command                 string
	expected, found, missed int
	first                   []string
	note                    string
}

// newRow is recall._row: count the items found and keep the first misses.
func newRow[T any](command string, truth []T, label func(T) string, found func(T) bool, note string) recallRow {
	r := recallRow{command: command, expected: len(truth), note: note}
	for _, x := range truth {
		if !found(x) {
			r.missed++
			if len(r.first) < recallFirstMisses {
				r.first = append(r.first, label(x))
			}
		}
	}
	r.found = r.expected - r.missed
	return r
}

// recallSampleOf is recall._sample: n items evenly spaced over items.
func recallSampleOf[T any](items []T, n int) []T {
	if len(items) <= n {
		return items
	}
	step := float64(len(items)) / float64(n)
	out := make([]T, n)
	for i := range n {
		out[i] = items[int(float64(i)*step)]
	}
	return out
}

var noGenerics = regexp.MustCompile(`&lt;.*?&gt;|<.*?>`)

func classQName(k classKey) string { return k.pkg + "." + k.name }

func memberLabel(m memberKey) string { return m.pkg + "." + m.class + "#" + m.member }

// checkAll is recall.check_all. The sampled find / fuzzy rows (samples)
// are not ported yet: both forms print the --no-samples table.
func (e *Env) checkAll(samples bool) ([]recallRow, float64, error) {
	truth := groundTruth(e.BaseDir)
	load := func(name string) (*index.Object, error) { return e.loadIndex(name, false) }
	var err error
	idx := map[string]*index.Object{}
	for _, n := range []string{"class-index.json", "jdk-class-index.json", "method-index.json",
		"slots-index.json", "source-index.json", "inheritance.json", "xref-index.json", "guides-index.json"} {
		if idx[n], err = load(n); err != nil {
			return nil, 0, err
		}
	}
	ci, jdkCI, mi, si := idx["class-index.json"], idx["jdk-class-index.json"], idx["method-index.json"], idx["slots-index.json"]
	src, inh, xref, gi := idx["source-index.json"], idx["inheritance.json"], idx["xref-index.json"], idx["guides-index.json"]

	var firstErr error
	hasClass := func(ix *index.Object, k classKey) bool {
		hits, err := classMatches(ix, classQName(k))
		if err != nil && firstErr == nil {
			firstErr = err
		}
		for _, h := range hits {
			if h.Package == k.pkg {
				return true
			}
		}
		return false
	}
	var rows []recallRow
	rows = append(rows, newRow("(.bajadoc parse)", truth.xmlFiles, func(s string) string { return s },
		func(rel string) bool { return !truth.xmlFailed[rel] }, "an unparseable file would hide its classes"))
	keys := truth.classes
	rows = append(rows, newRow("class", keys, classQName, func(k classKey) bool { return hasClass(ci, k) }, ""))
	rows = append(rows, newRow("search", keys, classQName, func(k classKey) bool {
		if !ci.Has(k.name) {
			return false
		}
		hits, err := classMatches(ci, k.name)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		for _, h := range hits {
			if h.Package == k.pkg {
				return true
			}
		}
		return false
	}, ""))
	pkgs := map[string]map[string]bool{}
	for _, key := range ci.Keys() {
		all, err := expand(key, ci.Object(key))
		if err != nil {
			return nil, 0, err
		}
		for _, c := range all {
			if pkgs[c.Package] == nil {
				pkgs[c.Package] = map[string]bool{}
			}
			pkgs[c.Package][key] = true
		}
	}
	rows = append(rows, newRow("package", keys, classQName, func(k classKey) bool { return pkgs[k.pkg][k.name] }, ""))
	rows = append(rows, newRow("api / related", keys, classQName, func(k classKey) bool {
		hits, err := classMatches(ci, classQName(k))
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return len(hits) > 0 && hits[0].Package == k.pkg
	}, "shared resolver resolve_entry"))
	ikeys := newInheritanceKeys(inh)
	rows = append(rows, newRow("hierarchy", keys, classQName, func(k classKey) bool { return ikeys.resolve(classQName(k)) != "" }, ""))

	// implementors lists classes; an interface's super-interfaces are its
	// hierarchy, not implementors.
	type implPair struct{ iface, name, pkg string }
	pairSet := map[implPair]bool{}
	var pairs []implPair
	for _, c := range truth.xml {
		if c.kind != "class" && c.kind != "enum" {
			continue
		}
		for _, iface := range c.implements {
			simple := strings.ReplaceAll(strings.SplitN(iface, "<", 2)[0], "$", ".")
			simple = simple[strings.LastIndex(simple, ".")+1:]
			if p := (implPair{simple, c.name, c.pkg}); !pairSet[p] {
				pairSet[p] = true
				pairs = append(pairs, p)
			}
		}
	}
	sortStable(pairs, func(a, b implPair) bool {
		if a.iface != b.iface {
			return a.iface < b.iface
		}
		if a.name != b.name {
			return a.name < b.name
		}
		return a.pkg < b.pkg
	})
	implCache := map[string]map[implementor]bool{}
	rows = append(rows, newRow("implementors", pairs,
		func(p implPair) string { return p.iface + " <- " + p.pkg + "." + p.name },
		func(p implPair) bool {
			if implCache[p.iface] == nil {
				set := map[implementor]bool{}
				for _, im := range implementorMatches(inh, p.iface) {
					set[im] = true
				}
				implCache[p.iface] = set
			}
			return implCache[p.iface][implementor{p.name, p.pkg}]
		}, ""))

	// extends keeps the first position and the last value of a repeated
	// (package, name); simple_ext keeps the first per simple name.
	var extOrder []classKey
	extends := map[classKey]string{}
	for _, c := range truth.xml {
		k := classKey{c.pkg, c.name}
		if _, ok := extends[k]; !ok {
			extOrder = append(extOrder, k)
		}
		extends[k] = c.extends
	}
	simpleExt := map[string]string{}
	for _, k := range extOrder {
		if _, ok := simpleExt[k.name]; !ok {
			simpleExt[k.name] = extends[k]
		}
	}
	isEnum := func(c bajadocClass) bool {
		seen := map[string]bool{}
		for cur := c.extends; cur != "" && !seen[cur]; {
			seen[cur] = true
			s := strings.SplitN(cur, "<", 2)[0]
			s = s[strings.LastIndex(s, ".")+1:]
			if recallEnumRoots[s] {
				return true
			}
			cur = simpleExt[s]
		}
		return false
	}
	enumSet := map[classKey]bool{}
	var enums []classKey
	for _, c := range truth.xml {
		if k := (classKey{c.pkg, c.name}); !enumSet[k] && isEnum(c) {
			enumSet[k] = true
			enums = append(enums, k)
		}
	}
	sortStable(enums, lessClassKey)
	rows = append(rows, newRow("enum", enums, classQName,
		func(k classKey) bool { return resolveEnum(src, inh, ikeys, classQName(k)) }, ""))

	modSet := map[string]bool{}
	var modules []string
	for _, c := range truth.xml {
		if c.module != "" && !modSet[c.module] {
			modSet[c.module] = true
			modules = append(modules, c.module)
		}
	}
	sortStable(modules, func(a, b string) bool { return a < b })
	rows = append(rows, newRow("module", modules, func(s string) string { return s },
		func(m string) bool { return len(moduleClassMatches(ci, m)) > 0 }, ""))

	jdkSet := map[classKey]bool{}
	var jdkKeys []classKey
	for _, c := range truth.jdk {
		if k := (classKey{c.pkg, c.name}); !jdkSet[k] {
			jdkSet[k] = true
			jdkKeys = append(jdkKeys, k)
		}
	}
	sortStable(jdkKeys, lessClassKey)
	rows = append(rows, newRow("class --jdk", jdkKeys, classQName, func(k classKey) bool { return hasClass(jdkCI, k) }, ""))

	miLower := lowerKeys(mi)
	memberCache := map[string][]*index.Object{}
	rows = append(rows, newRow("method", truth.methods, memberLabel, func(m memberKey) bool {
		entries, ok := memberCache[m.member]
		if !ok {
			entries = memberMatches(mi, miLower, m.member)
			memberCache[m.member] = entries
		}
		for _, en := range entries {
			if noGenerics.ReplaceAllString(orStr(en, "class", "", ""), "") == m.class &&
				strEq(get(en, "package", nil), m.pkg) {
				return true
			}
		}
		return false
	}, ""))

	sk := slotKeys(si)
	rows = append(rows, newRow("slots", truth.slots, memberLabel, func(m memberKey) bool {
		for _, k := range sk[lowerStr(m.class)] {
			data := si.Object(k)
			if !strEq(get(data, "package", nil), m.pkg) {
				continue
			}
			for _, sec := range []string{"properties", "actions", "topics"} {
				for _, s := range objList(data, sec) {
					if v, ok := get(s, "name", nil).(string); ok && v == m.member {
						return true
					}
				}
			}
		}
		return false
	}, ""))

	rows = append(rows, newRow("source", truth.java, func(s string) string { return s }, func(rel string) bool {
		simple := strings.TrimSuffix(pycompat.PosixBasename(rel), ".java")
		for _, sm := range sourceMatches(src, simple) {
			if corpusPath(sm.data) == rel {
				return true
			}
		}
		return false
	}, ""))
	rows = append(rows, newRow("xref", truth.java, func(s string) string { return s }, func(rel string) bool {
		simple := strings.TrimSuffix(pycompat.PosixBasename(rel), ".java")
		entry := getObj(xref, simple)
		return entry != nil && entry.Has("package")
	}, "the file's own entry (package), not only used_by"))

	gclean, gsrc := filepath.Join(e.BaseDir, "guides-clean"), filepath.Join(e.BaseDir, "guides")
	rows = append(rows, newRow("guide-search", truth.guides, func(s string) string { return s }, func(rel string) bool {
		return isRegularFile(filepath.Join(gclean, filepath.FromSlash(cleanTextName(rel, gsrc))))
	}, ""))
	topics := map[string]map[string]bool{}
	for _, f := range gi.Keys() {
		d := gi.Object(f)
		list := strList(d, "topics")
		if d.Has("all_topics") {
			list = strList(d, "all_topics")
		}
		set := map[string]bool{}
		for _, t := range list {
			set[t] = true
		}
		topics[f] = set
	}
	rows = append(rows, newRow("guide", truth.guides, func(s string) string { return s }, func(rel string) bool {
		folder, rest, _ := strings.Cut(rel, "/")
		stem, _ := pycompat.PosixSplitext(rest)
		return topics[folder][rest] || topics[folder][stem]
	}, "topic = page path in guides-index.json"))
	dclean, dsrc := filepath.Join(e.BaseDir, "devguide-clean"), filepath.Join(e.BaseDir, "devguide")
	rows = append(rows, newRow("devguide-search", truth.devguide, func(s string) string { return s }, func(rel string) bool {
		return isRegularFile(filepath.Join(dclean, filepath.FromSlash(cleanTextName(rel, dsrc))))
	}, ""))

	if firstErr != nil {
		return nil, 0, firstErr
	}
	rows = append(rows, recallRow{command: "callers", note: "excluded: keyed by call sites; a method " +
		"with no caller has no entry by definition (its class is covered by source/xref recall)"})
	return rows, truth.seconds, nil
}

// strEq is `value == s` for a decoded JSON value and a string.
func strEq(v any, s string) bool {
	x, ok := v.(string)
	return ok && x == s
}

// isRegularFile is os.path.isfile.
func isRegularFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// Recall is recall.cmd_recall: print the table; exit 1 when a row missed.
func (e *Env) Recall(samples bool) error {
	rows, seconds, err := e.checkAll(samples)
	if err != nil {
		return err
	}
	vid := identify(e.BaseDir)
	if vid == "" {
		vid = e.BaseDir
	}
	fmt.Fprintf(e.Out, "RECALL %s (ground truth from raw sources, parsed in %.0fs)\n", vid, seconds)
	fmt.Fprintf(e.Out, "  %s %s %s %s  note\n", ljust("command", 18), rjust("expected", 9),
		rjust("found", 9), rjust("missed", 8))
	bad := 0
	for _, r := range rows {
		fmt.Fprintf(e.Out, "  %s %s %s %s  %s\n", ljust(r.command, 18), rjust(thousands(r.expected), 9),
			rjust(thousands(r.found), 9), rjust(thousands(r.missed), 8), r.note)
		for _, m := range r.first {
			fmt.Fprintf(e.Out, "      miss: %s\n", m)
		}
		if r.missed > 0 {
			bad++
		}
	}
	if bad > 0 {
		fmt.Fprintf(e.Out, "RESULT: FAIL (%d command(s) with misses)\n", bad)
		return ExitCode(1)
	}
	fmt.Fprintln(e.Out, "RESULT: PASS (100% recall)")
	return ExitCode(0)
}
