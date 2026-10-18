package check

import (
	"archive/zip"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/course/key"
)

// Verdicts. A check that could not run (no --install, an unreadable jar) is SKIP, never PASS.
const (
	Pass = "PASS"
	Fail = "FAIL"
	Skip = "SKIP"
)

// Result is one check run against a station.
type Result struct {
	*Check
	Verdict  string
	Evidence string
	// Absent: the check is SKIP because a tree it reads (--install, --source, --labsrc,
	// --solutions) was not given, not because one was unreadable; verify does not let such a check
	// lower what the others decide (Decide).
	Absent bool
}

// Station is the input of a check run: the parsed bog, its directory and the optional install.
type Station struct {
	dir     string // the station directory (holds config.bog and shared/)
	Name    string // the root component's stationName, else the directory name
	b       *bog.Bog
	nested  bool   // read with bog.Options.NestedSlots: Comp.Nested holds the dotted slots
	install string // "" when --install was not given
	source  string // the source tree of the source-text kind; "" when --source was not given
	labsrc  string // the lab module sources of the annotation kind; "" when --labsrc was not given
	// solutions is the corpus solutions tree of the rows with Root solutions; "" when --solutions
	// was not given.
	solutions string
	jars      map[string]*jarFile
}

type jarFile struct {
	zr  *zip.ReadCloser
	err error
}

// Open reads a station directory or a bog file inside one; nested also reads the values
// stored below composite slots (Comp.Nested). The error is the bog reader's message (exit 2).
func Open(path, install string, nested bool) (*Station, error) {
	file, dir := path, filepath.Dir(path)
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		file, dir = filepath.Join(path, "config.bog"), path
	}
	b, err := bog.LoadOptions(file, bog.Options{NestedSlots: nested})
	if err != nil {
		return nil, err
	}
	s := &Station{dir: dir, Name: filepath.Base(dir), b: b, nested: nested, install: install, jars: map[string]*jarFile{}}
	for _, h := range b.Roots {
		if c, _ := b.Comps.Get(h); c != nil {
			if sl, ok := c.Slots.Get("stationName"); ok && sl.Value != nil && *sl.Value != "" {
				s.Name = *sl.Value
			}
		}
	}
	return s, nil
}

// NeedsNested reports whether a check reads a dotted (nested) slot, so the bog must be read
// with bog.Options.NestedSlots.
func NeedsNested(checks []*Check) bool {
	for _, ck := range checks {
		if (ck.Kind == kindSlot || ck.Kind == kindListing) && ck.dotted() {
			return true
		}
	}
	return false
}

// dotted reports whether the slot or one of the Stored slots is nested.
func (ck *Check) dotted() bool {
	for _, sl := range append([]string{ck.Slot}, ck.Stored...) {
		if strings.Contains(sl, ".") {
			return true
		}
	}
	return false
}

// Scope is "station <name> (<n> components, <m> links)", for a summary line.
func (s *Station) Scope() string {
	return fmt.Sprintf("station %s (%s, %s)", s.Name, key.Plural(s.b.Comps.Len(), "component"),
		key.Plural(len(s.b.Links), "link"))
}

// Close releases the jars the checks opened.
func (s *Station) Close() {
	for _, j := range s.jars {
		if j.zr != nil {
			j.zr.Close()
		}
	}
}

// isType reports whether c is "module:Type"; the module comes from the bog's final prefix
// map, so a prefix declared after its first use still resolves.
func (s *Station) isType(c *bog.Comp, spec string) bool {
	module, typ, _ := strings.Cut(spec, ":")
	m, _ := s.b.Prefixes.Get(c.Pfx)
	return m == module && c.SimpleType() == typ
}

func (s *Station) within(c *bog.Comp, spec string) bool {
	for p := c.ParentH; p != nil; {
		pc, ok := s.b.Comps.Get(*p)
		if !ok {
			return false
		}
		if s.isType(pc, spec) {
			return true
		}
		p = pc.ParentH
	}
	return false
}

// comps returns the components of type spec in document order.
func (s *Station) comps(spec string) []*bog.Comp {
	var out []*bog.Comp
	for _, c := range s.b.Comps.All() {
		if s.isType(c, spec) {
			out = append(out, c)
		}
	}
	return out
}

func countOK(op string, got, n int) bool {
	if op == opEq {
		return got == n
	}
	return got >= n
}

func want(op string, n int) string {
	if op == opEq {
		return fmt.Sprintf("exactly %d", n)
	}
	return fmt.Sprintf("at least %d", n)
}

// pathList prints up to three component paths.
func pathList(cs []*bog.Comp) string {
	var names []string
	for i, c := range cs {
		if i == 3 {
			names = append(names, fmt.Sprintf("… (+%d)", len(cs)-3))
			break
		}
		names = append(names, c.Path)
	}
	return strings.Join(names, ", ")
}

// Run runs one check on the station.
func (s *Station) Run(ck *Check) Result {
	r := Result{Check: ck}
	if err := ck.validate(); err != nil {
		r.Verdict, r.Evidence = Skip, "bad row: "+err.Error()
		return r
	}
	switch ck.Kind {
	case kindType:
		r.Verdict, r.Evidence = s.checkType(ck)
	case kindSlot:
		r.Verdict, r.Evidence = s.checkSlot(ck)
	case kindLink:
		r.Verdict, r.Evidence = s.checkLink(ck)
	case kindFiles:
		r.Verdict, r.Evidence = s.checkFiles(ck)
	case kindFileText:
		r.Verdict, r.Evidence = s.checkFileText(ck)
	case kindAudit, kindLog, kindTrend:
		r.Verdict, r.Evidence = s.checkHistory(ck)
	case kindSourceText:
		r.Verdict, r.Evidence = s.checkSourceText(ck)
	case kindAnnotation:
		r.Verdict, r.Evidence = s.checkAnnotation(ck)
	case kindJarCert:
		r.Verdict, r.Evidence = s.checkJarCert(ck)
	case kindListing:
		r.Verdict, r.Evidence = s.checkListing(ck)
	default:
		r.Verdict, r.Evidence = s.checkJar(ck)
	}
	r.Absent = r.Verdict == Skip && s.treeNotGiven(ck)
	return r
}

// treeNotGiven reports whether a tree the check reads was not given to the run: the install for
// the jar kinds, the source tree for source-text, the root of an annotation, files, file-text or
// jar-cert pem row.
func (s *Station) treeNotGiven(ck *Check) bool {
	if strings.HasPrefix(ck.Kind, "jar") && s.install == "" {
		return true
	}
	if ck.Kind == kindSourceText {
		return s.source == ""
	}
	root := ck.Root
	if ck.Kind == kindAnnotation && root == rootStation {
		root = rootLabsrc
	}
	switch root {
	case rootInstall:
		return s.install == ""
	case rootLabsrc:
		return s.labsrc == ""
	case rootSolutions:
		return s.solutions == ""
	}
	return false
}

func verdict(ok bool) string {
	if ok {
		return Pass
	}
	return Fail
}

// below reports whether a component path is inside the folder path under.
func below(path, under string) bool { return strings.HasPrefix(path, under+"/") }

// selected returns the components of the check's type that pass its Within and Under filters,
// all the components of the type, and the filter as evidence (" under T", " in F", or "").
func (s *Station) selected(ck *Check) (cs, all []*bog.Comp, where string) {
	all = s.comps(ck.Type)
	if ck.Within == "" && ck.Under == "" {
		return all, all, ""
	}
	for _, c := range all {
		if (ck.Within == "" || s.within(c, ck.Within)) && (ck.Under == "" || below(c.Path, ck.Under)) {
			cs = append(cs, c)
		}
	}
	if ck.Within != "" {
		where = " under " + ck.Within
	}
	if ck.Under != "" {
		where += " in " + ck.Under
	}
	if len(all) != len(cs) {
		where += fmt.Sprintf(" (%d in total)", len(all))
	}
	return cs, all, where
}

func (s *Station) checkType(ck *Check) (string, string) {
	cs, all, where := s.selected(ck)
	ev := fmt.Sprintf("found %d", len(cs)) + where
	ok := countOK(ck.Op, len(cs), ck.N)
	if !ok {
		ev += ", want " + want(ck.Op, ck.N)
	}
	switch {
	case len(cs) > 0:
		ev += ": " + pathList(cs)
	case ck.Under != "" && len(all) > 0:
		ev += "; elsewhere: " + pathList(all)
	}
	return verdict(ok), ev
}

// effective is the value of a slot as the station runs it: the stored value (a composite's
// nested value), else the type default. stored is false when the bog keeps no value. A dotted
// slot (address.port) is a value below a composite slot, read from Comp.Nested.
func effective(c *bog.Comp, slot, def string) (value string, stored bool) {
	if strings.Contains(slot, ".") {
		if v, ok := c.Nested.Get(slot); ok {
			return v, true
		}
		return def, false
	}
	if sl, ok := c.Slots.Get(slot); ok {
		if sl.Value != nil {
			return *sl.Value, true
		}
		if v, ok := sl.Child.Get("value"); ok {
			return v, true
		}
	}
	return def, false
}

func describe(c *bog.Comp, slot, value string, stored bool) string {
	src := "default"
	if stored {
		src = "stored"
	}
	if value == "" {
		value = `""`
	}
	return fmt.Sprintf("%s.%s = %s (%s)", c.Path, slot, value, src)
}

func (s *Station) checkSlot(ck *Check) (string, string) {
	if ck.dotted() && !s.nested {
		// Never pass a nested slot on its type default because the values were not read.
		return Skip, strings.Join(append([]string{ck.Slot}, ck.Stored...), ", ") +
			" is a nested slot: the station was read without nested values"
	}
	cs, _, where := s.selected(ck)
	if len(cs) == 0 {
		if where != "" {
			return Fail, "no " + ck.Type + " component" + where
		}
		return Fail, "no " + ck.Type + " component (searched " + key.Plural(s.b.Comps.Len(), "component") + ")"
	}
	if ck.Distinct {
		seen := map[string]bool{}
		var vals []string
		for _, c := range cs {
			if v, _ := effective(c, ck.Slot, ck.Default); !seen[v] {
				seen[v] = true
				vals = append(vals, v)
			}
		}
		sort.Strings(vals)
		ok := countOK(ck.Op, len(vals), ck.N)
		ev := fmt.Sprintf("%d distinct %s over %s", len(vals), ck.Slot, key.Plural(len(cs), "component"))
		if !ok {
			ev += ", want " + want(ck.Op, ck.N)
		}
		return verdict(ok), ev + ": " + strings.Join(vals, ", ")
	}
	var bad []string
	first := ""
	for _, c := range cs {
		v, stored := effective(c, ck.Slot, ck.Default)
		pass := v == ck.Want
		if ck.Set {
			pass = stored
		}
		if first == "" {
			first = describe(c, ck.Slot, v, stored)
		}
		why := describe(c, ck.Slot, v, stored)
		for _, also := range ck.Stored {
			if av, astored := effective(c, also, ""); pass && !astored {
				pass, why = false, describe(c, also, av, astored)
			}
		}
		if !pass {
			bad = append(bad, why)
		}
	}
	target := "want " + ck.Want
	if ck.Set {
		target = "want it stored"
	}
	if len(ck.Stored) > 0 {
		target += " with " + strings.Join(ck.Stored, ", ") + " stored"
	}
	if len(bad) == 0 {
		if len(cs) == 1 {
			return Pass, first
		}
		return Pass, fmt.Sprintf("%d of %d: %s", len(cs), len(cs), first)
	}
	return Fail, fmt.Sprintf("%d of %d pass, %s: %s", len(cs)-len(bad), len(cs), target, bad[0])
}

// linkSource resolves a link's source component: a handle, else a "slot:/path" ord.
func (s *Station) linkSource(l *bog.Link) *bog.Comp {
	if l.SrcH != nil {
		c, _ := s.b.Comps.Get(*l.SrcH)
		return c
	}
	if l.SrcOrd != nil {
		ord := *l.SrcOrd
		if i := strings.LastIndex(ord, "slot:"); i >= 0 {
			return s.b.Resolve(ord[i+len("slot:"):])
		}
	}
	return nil
}

func (s *Station) checkLink(ck *Check) (string, string) {
	var rows []string
	for _, l := range s.b.Links {
		if l.ContainerH == nil || l.TgtSlot == nil || *l.TgtSlot != ck.Slot || l.SrcSlot == nil || *l.SrcSlot != ck.FromSlot {
			continue
		}
		tgt, _ := s.b.Comps.Get(*l.ContainerH)
		src := s.linkSource(l)
		if tgt == nil || src == nil || !s.isType(tgt, ck.Type) || !s.isType(src, ck.From) {
			continue
		}
		rows = append(rows, fmt.Sprintf("%s.%s -> %s.%s", src.Path, ck.FromSlot, tgt.Path, ck.Slot))
	}
	ok := countOK(ck.Op, len(rows), ck.N)
	ev := fmt.Sprintf("found %d", len(rows))
	if !ok {
		ev += fmt.Sprintf(", want %s (searched %s)", want(ck.Op, ck.N), key.Plural(len(s.b.Links), "link"))
	}
	if len(rows) > 0 {
		ev += ": " + rows[0]
		if len(rows) > 1 {
			ev += fmt.Sprintf(" (+%d)", len(rows)-1)
		}
	}
	return verdict(ok), ev
}

func (s *Station) checkFiles(ck *Check) (string, string) {
	pattern, rels, skip := s.rootFiles(ck)
	if skip != "" {
		return Skip, skip
	}
	files := make([]string, len(rels))
	for i, rel := range rels {
		files[i] = path.Base(rel)
		if strings.HasSuffix(rel, "/") {
			files[i] += "/"
		}
	}
	ok := countOK(ck.Op, len(files), ck.N)
	ev := fmt.Sprintf("found %d %s", len(files), pattern)
	if !ok {
		ev += ", want " + want(ck.Op, ck.N)
	}
	if len(files) > 0 {
		ev += ": " + files[0]
		if len(files) > 1 {
			ev += fmt.Sprintf(" … %s", files[len(files)-1])
		}
	}
	return verdict(ok), ev
}

// globEscape makes s match only itself in a filepath.Glob pattern: each meta character becomes a
// one-character class, which works on every OS (Windows has no backslash escape in patterns).
func globEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '*' || r == '?' || r == '[':
			b.WriteString("[" + string(r) + "]")
		case r == '\\' && filepath.Separator != '\\':
			b.WriteString(`[\\]`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
