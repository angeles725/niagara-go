package course

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
)

// Verdicts. A check that could not run (no --install, an unreadable jar) is SKIP, never PASS.
const (
	verdictPass = "PASS"
	verdictFail = "FAIL"
	verdictSkip = "SKIP"
)

// Result is one check run against a station.
type Result struct {
	*Check
	Verdict  string
	Evidence string
}

// station is the input of a check run: the parsed bog, its directory and the optional install.
type station struct {
	dir     string // the station directory (holds config.bog and shared/)
	name    string // the root component's stationName, else the directory name
	b       *bog.Bog
	install string // "" when --install was not given
	jars    map[string]*jarFile
}

type jarFile struct {
	zr  *zip.ReadCloser
	err error
}

// openStation reads a station directory or a bog file inside one. The error is the bog
// reader's message (exit 2).
func openStation(path, install string) (*station, error) {
	file, dir := path, filepath.Dir(path)
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		file, dir = filepath.Join(path, "config.bog"), path
	}
	b, err := bog.Load(file)
	if err != nil {
		return nil, err
	}
	s := &station{dir: dir, name: filepath.Base(dir), b: b, install: install, jars: map[string]*jarFile{}}
	for _, h := range b.Roots {
		if c, _ := b.Comps.Get(h); c != nil {
			if sl, ok := c.Slots.Get("stationName"); ok && sl.Value != nil && *sl.Value != "" {
				s.name = *sl.Value
			}
		}
	}
	return s, nil
}

func (s *station) close() {
	for _, j := range s.jars {
		if j.zr != nil {
			j.zr.Close()
		}
	}
}

// isType reports whether c is "module:Type"; the module comes from the bog's final prefix
// map, so a prefix declared after its first use still resolves.
func (s *station) isType(c *bog.Comp, spec string) bool {
	module, typ, _ := strings.Cut(spec, ":")
	m, _ := s.b.Prefixes.Get(c.Pfx)
	return m == module && c.SimpleType() == typ
}

func (s *station) within(c *bog.Comp, spec string) bool {
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
func (s *station) comps(spec string) []*bog.Comp {
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

func (s *station) run(ck *Check) Result {
	r := Result{Check: ck}
	switch ck.Kind {
	case kindType:
		r.Verdict, r.Evidence = s.checkType(ck)
	case kindSlot:
		r.Verdict, r.Evidence = s.checkSlot(ck)
	case kindLink:
		r.Verdict, r.Evidence = s.checkLink(ck)
	case kindFiles:
		r.Verdict, r.Evidence = s.checkFiles(ck)
	default:
		r.Verdict, r.Evidence = s.checkJar(ck)
	}
	return r
}

func verdict(ok bool) string {
	if ok {
		return verdictPass
	}
	return verdictFail
}

func (s *station) checkType(ck *Check) (string, string) {
	all := s.comps(ck.Type)
	cs := all
	if ck.Within != "" {
		cs = nil
		for _, c := range all {
			if s.within(c, ck.Within) {
				cs = append(cs, c)
			}
		}
	}
	ev := fmt.Sprintf("found %d", len(cs))
	if ck.Within != "" {
		ev += " under " + ck.Within
		if len(all) != len(cs) {
			ev += fmt.Sprintf(" (%d in total)", len(all))
		}
	}
	ok := countOK(ck.Op, len(cs), ck.N)
	if !ok {
		ev += ", want " + want(ck.Op, ck.N)
	}
	if len(cs) > 0 {
		ev += ": " + pathList(cs)
	}
	return verdict(ok), ev
}

// effective is the value of a slot as the station runs it: the stored value (a composite's
// nested value), else the type default. stored is false when the bog keeps no value.
func effective(c *bog.Comp, slot, def string) (value string, stored bool) {
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

func (s *station) checkSlot(ck *Check) (string, string) {
	cs := s.comps(ck.Type)
	if len(cs) == 0 {
		return verdictFail, "no " + ck.Type + " component (searched " + plural(s.b.Comps.Len(), "component") + ")"
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
		ev := fmt.Sprintf("%d distinct %s over %s", len(vals), ck.Slot, plural(len(cs), "component"))
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
		if !pass {
			bad = append(bad, describe(c, ck.Slot, v, stored))
		}
	}
	target := "want " + ck.Want
	if ck.Set {
		target = "want it stored"
	}
	if len(bad) == 0 {
		if len(cs) == 1 {
			return verdictPass, first
		}
		return verdictPass, fmt.Sprintf("%d of %d: %s", len(cs), len(cs), first)
	}
	return verdictFail, fmt.Sprintf("%d of %d pass, %s: %s", len(cs)-len(bad), len(cs), target, bad[0])
}

// linkSource resolves a link's source component: a handle, else a "slot:/path" ord.
func (s *station) linkSource(l *bog.Link) *bog.Comp {
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

func (s *station) checkLink(ck *Check) (string, string) {
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
		ev += fmt.Sprintf(", want %s (searched %s)", want(ck.Op, ck.N), plural(len(s.b.Links), "link"))
	}
	if len(rows) > 0 {
		ev += ": " + rows[0]
		if len(rows) > 1 {
			ev += fmt.Sprintf(" (+%d)", len(rows)-1)
		}
	}
	return verdict(ok), ev
}

func (s *station) checkFiles(ck *Check) (string, string) {
	pattern := strings.ReplaceAll(ck.Glob, "{station}", s.name)
	matches, err := filepath.Glob(filepath.Join(s.dir, filepath.FromSlash(pattern)))
	if err != nil {
		return verdictSkip, fmt.Sprintf("bad pattern %s: %v", pattern, err)
	}
	var files []string
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil && st.Mode().IsRegular() {
			files = append(files, filepath.Base(m))
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

// globRe turns a jar-entry glob into a regexp: ** crosses '/', * and ? do not.
func globRe(glob string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(".*")
			i++
		case glob[i] == '*':
			b.WriteString("[^/]*")
		case glob[i] == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(glob[i : i+1]))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

func (s *station) jar(name string) (*zip.ReadCloser, error) {
	if j, ok := s.jars[name]; ok {
		return j.zr, j.err
	}
	zr, err := zip.OpenReader(filepath.Join(s.install, "modules", name+".jar"))
	s.jars[name] = &jarFile{zr, err}
	return zr, err
}

func (s *station) checkJar(ck *Check) (string, string) {
	rel := "modules/" + ck.Jar + ".jar"
	if s.install == "" {
		return verdictSkip, "no --install DIR: " + rel + " not checked"
	}
	zr, err := s.jar(ck.Jar)
	if os.IsNotExist(err) {
		return verdictFail, "missing " + rel + " under " + s.install
	}
	if err != nil {
		return verdictSkip, fmt.Sprintf("cannot read %s: %v", rel, err)
	}
	switch ck.Kind {
	case kindJar:
		return verdictPass, fmt.Sprintf("%s (%s)", rel, entryCount(len(zr.File)))
	case kindJarSigned:
		sf, block := entries(zr, globRe("META-INF/*.SF")), entries(zr, regexp.MustCompile(`^META-INF/[^/]+\.(?:RSA|DSA|EC)$`))
		if len(sf) == 0 || len(block) == 0 {
			return verdictFail, fmt.Sprintf("%s: %d signature files (.SF), %d signature blocks (.RSA/.DSA/.EC)", rel, len(sf), len(block))
		}
		return verdictPass, fmt.Sprintf("%s: %s, %s", rel, sf[0].Name, block[0].Name)
	case kindJarEntry:
		found := entries(zr, globRe(ck.Glob))
		ok := countOK(ck.Op, len(found), ck.N)
		var ev string
		if len(found) == 1 {
			ev = fmt.Sprintf("%s: 1 entry matches %s", rel, ck.Glob)
		} else {
			ev = fmt.Sprintf("%s: %d entries match %s", rel, len(found), ck.Glob)
		}
		if !ok {
			ev += fmt.Sprintf(", want %s (searched %s)", want(ck.Op, ck.N), entryCount(len(zr.File)))
		}
		return verdict(ok), ev
	default: // kindJarText
		return jarText(zr, rel, ck)
	}
}

func entries(zr *zip.ReadCloser, re *regexp.Regexp) []*zip.File {
	var out []*zip.File
	for _, f := range zr.File {
		if re.MatchString(f.Name) {
			out = append(out, f)
		}
	}
	return out
}

func jarText(zr *zip.ReadCloser, rel string, ck *Check) (string, string) {
	found := entries(zr, globRe(ck.Glob))
	if len(found) == 0 {
		return verdictFail, fmt.Sprintf("%s: no entry %s (searched %s)", rel, ck.Glob, entryCount(len(zr.File)))
	}
	rc, err := found[0].Open()
	if err != nil {
		return verdictSkip, fmt.Sprintf("cannot read %s!%s: %v", rel, found[0].Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 16<<20))
	if err != nil {
		return verdictSkip, fmt.Sprintf("cannot read %s!%s: %v", rel, found[0].Name, err)
	}
	var missing []string
	for _, want := range ck.Contains {
		if !strings.Contains(string(data), want) {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		return verdictFail, fmt.Sprintf("%s!%s lacks %s (%d of %d found)", rel, found[0].Name,
			strings.Join(missing, ", "), len(ck.Contains)-len(missing), len(ck.Contains))
	}
	return verdictPass, fmt.Sprintf("%s!%s has %s", rel, found[0].Name, strings.Join(ck.Contains, ", "))
}

func entryCount(n int) string {
	if n == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", n)
}
