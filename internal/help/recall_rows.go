package help

// The rows of recall.check_all, one function per recall family: classes
// (class, search, package, api, hierarchy), implementors, enums, modules,
// JDK classes, members (method, slots), source files (source, xref), guide
// pages and the find/fuzzy samples. checkAll calls them in table order.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/bajadocxml"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// recallCheck is what the rows of one check_all run share: the ground
// truth, the loaded indexes and the first resolver error, which fails the
// run once every row ran.
type recallCheck struct {
	e                  *Env
	truth              recallTruth
	ci, jdkCI, mi, si  *index.Object
	src, inh, xref, gi *index.Object
	ikeys              *inheritanceKeys
	firstErr           error
}

// newRecallCheck parses the ground truth and loads every index a row reads.
func (e *Env) newRecallCheck() (*recallCheck, error) {
	c := &recallCheck{e: e, truth: groundTruth(e.BaseDir)}
	idx := map[string]*index.Object{}
	for _, n := range []string{"class-index.json", "jdk-class-index.json", "method-index.json",
		"slots-index.json", "source-index.json", "inheritance.json", "xref-index.json", "guides-index.json"} {
		var err error
		if idx[n], err = e.loadIndex(n, false); err != nil {
			return nil, err
		}
	}
	c.ci, c.jdkCI, c.mi, c.si = idx["class-index.json"], idx["jdk-class-index.json"], idx["method-index.json"], idx["slots-index.json"]
	c.src, c.inh, c.xref, c.gi = idx["source-index.json"], idx["inheritance.json"], idx["xref-index.json"], idx["guides-index.json"]
	c.ikeys = newInheritanceKeys(c.inh)
	return c, nil
}

// noteErr keeps the first resolver error.
func (c *recallCheck) noteErr(err error) {
	if err != nil && c.firstErr == nil {
		c.firstErr = err
	}
}

// hasClass: the class index ix resolves k's qualified name to k's package.
func (c *recallCheck) hasClass(ix *index.Object, k classKey) bool {
	hits, err := classMatches(ix, classQName(k))
	c.noteErr(err)
	for _, h := range hits {
		if h.Package == k.pkg {
			return true
		}
	}
	return false
}

// classRows are the .bajadoc parse row and the rows of the commands that
// resolve a class: class, search, package, api / related and hierarchy.
func (c *recallCheck) classRows() ([]recallRow, error) {
	truth, ci := c.truth, c.ci
	var rows []recallRow
	rows = append(rows, newRow("(.bajadoc parse)", truth.xmlFiles, func(s string) string { return s },
		func(rel string) bool { return !truth.xmlFailed[rel] }, "an unparseable file would hide its classes"))
	keys := truth.classes
	rows = append(rows, newRow("class", keys, classQName, func(k classKey) bool { return c.hasClass(ci, k) }, ""))
	rows = append(rows, newRow("search", keys, classQName, func(k classKey) bool {
		if !ci.Has(k.name) {
			return false
		}
		hits, err := classMatches(ci, k.name)
		c.noteErr(err)
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
			return nil, err
		}
		for _, cl := range all {
			if pkgs[cl.Package] == nil {
				pkgs[cl.Package] = map[string]bool{}
			}
			pkgs[cl.Package][key] = true
		}
	}
	rows = append(rows, newRow("package", keys, classQName, func(k classKey) bool { return pkgs[k.pkg][k.name] }, ""))
	rows = append(rows, newRow("api / related", keys, classQName, func(k classKey) bool {
		hits, err := classMatches(ci, classQName(k))
		c.noteErr(err)
		return len(hits) > 0 && hits[0].Package == k.pkg
	}, "shared resolver resolve_entry"))
	rows = append(rows, newRow("hierarchy", keys, classQName, func(k classKey) bool { return c.ikeys.resolve(classQName(k)) != "" }, ""))
	return rows, nil
}

// implementorsRow: implementors lists classes; an interface's
// super-interfaces are its hierarchy, not implementors.
func (c *recallCheck) implementorsRow() recallRow {
	type implPair struct{ iface, name, pkg string }
	pairSet := map[implPair]bool{}
	var pairs []implPair
	for _, cl := range c.truth.xml {
		if cl.Kind != "class" && cl.Kind != "enum" {
			continue
		}
		for _, iface := range cl.Implements {
			simple := strings.ReplaceAll(strings.SplitN(iface, "<", 2)[0], "$", ".")
			simple = simple[strings.LastIndex(simple, ".")+1:]
			if p := (implPair{simple, cl.Name, cl.Pkg}); !pairSet[p] {
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
	return newRow("implementors", pairs,
		func(p implPair) string { return p.iface + " <- " + p.pkg + "." + p.name },
		func(p implPair) bool {
			if implCache[p.iface] == nil {
				set := map[implementor]bool{}
				for _, im := range implementorMatches(c.inh, p.iface) {
					set[im] = true
				}
				implCache[p.iface] = set
			}
			return implCache[p.iface][implementor{p.name, p.pkg}]
		}, "")
}

// enumRow: the classes whose extends chain reaches an enum root.
func (c *recallCheck) enumRow() recallRow {
	// extends keeps the first position and the last value of a repeated
	// (package, name); simple_ext keeps the first per simple name.
	var extOrder []classKey
	extends := map[classKey]string{}
	for _, cl := range c.truth.xml {
		k := classKey{cl.Pkg, cl.Name}
		if _, ok := extends[k]; !ok {
			extOrder = append(extOrder, k)
		}
		extends[k] = cl.Extends
	}
	simpleExt := map[string]string{}
	for _, k := range extOrder {
		if _, ok := simpleExt[k.name]; !ok {
			simpleExt[k.name] = extends[k]
		}
	}
	isEnum := func(cl bajadocxml.Class) bool {
		seen := map[string]bool{}
		for cur := cl.Extends; cur != "" && !seen[cur]; {
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
	for _, cl := range c.truth.xml {
		if k := (classKey{cl.Pkg, cl.Name}); !enumSet[k] && isEnum(cl) {
			enumSet[k] = true
			enums = append(enums, k)
		}
	}
	sortStable(enums, lessClassKey)
	return newRow("enum", enums, classQName,
		func(k classKey) bool { return resolveEnum(c.src, c.inh, c.ikeys, classQName(k)) }, "")
}

// moduleRow: every module a class names has classes in the class index.
func (c *recallCheck) moduleRow() recallRow {
	modSet := map[string]bool{}
	var modules []string
	for _, cl := range c.truth.xml {
		if cl.Module != "" && !modSet[cl.Module] {
			modSet[cl.Module] = true
			modules = append(modules, cl.Module)
		}
	}
	sortStable(modules, func(a, b string) bool { return a < b })
	return newRow("module", modules, func(s string) string { return s },
		func(m string) bool { return len(moduleClassMatches(c.ci, m)) > 0 }, "")
}

// jdkRow: every JDK class resolves in the JDK class index.
func (c *recallCheck) jdkRow() recallRow {
	jdkSet := map[classKey]bool{}
	var jdkKeys []classKey
	for _, cl := range c.truth.jdk {
		if k := (classKey{cl.Pkg, cl.Name}); !jdkSet[k] {
			jdkSet[k] = true
			jdkKeys = append(jdkKeys, k)
		}
	}
	sortStable(jdkKeys, lessClassKey)
	return newRow("class --jdk", jdkKeys, classQName, func(k classKey) bool { return c.hasClass(c.jdkCI, k) }, "")
}

// memberRows are the method and slots rows.
func (c *recallCheck) memberRows() []recallRow {
	mi, si := c.mi, c.si
	miLower := lowerKeys(mi)
	memberCache := map[string][]*index.Object{}
	method := newRow("method", c.truth.methods, memberLabel, func(m memberKey) bool {
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
	}, "")

	sk := slotKeys(si)
	slots := newRow("slots", c.truth.slots, memberLabel, func(m memberKey) bool {
		for _, k := range sk[textfold.Lower(m.class)] {
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
	}, "")
	return []recallRow{method, slots}
}

// sourceRows are the source and xref rows of every .java file.
func (c *recallCheck) sourceRows() []recallRow {
	source := newRow("source", c.truth.java, func(s string) string { return s }, func(rel string) bool {
		simple := strings.TrimSuffix(pycompat.PosixBasename(rel), ".java")
		for _, sm := range sourceMatches(c.src, simple) {
			if corpusPath(sm.data) == rel {
				return true
			}
		}
		return false
	}, "")
	xref := newRow("xref", c.truth.java, func(s string) string { return s }, func(rel string) bool {
		simple := strings.TrimSuffix(pycompat.PosixBasename(rel), ".java")
		entry := getObj(c.xref, simple)
		return entry != nil && entry.Has("package")
	}, "the file's own entry (package), not only used_by")
	return []recallRow{source, xref}
}

// guideRows are the guide-search, guide and devguide-search rows.
func (c *recallCheck) guideRows() []recallRow {
	base := c.e.BaseDir
	gclean, gsrc := filepath.Join(base, "guides-clean"), filepath.Join(base, "guides")
	guideSearch := newRow("guide-search", c.truth.guides, func(s string) string { return s }, func(rel string) bool {
		return isRegularFile(filepath.Join(gclean, filepath.FromSlash(cleanTextName(rel, gsrc))))
	}, "")
	topics := map[string]map[string]bool{}
	for _, f := range c.gi.Keys() {
		d := c.gi.Object(f)
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
	guide := newRow("guide", c.truth.guides, func(s string) string { return s }, func(rel string) bool {
		folder, rest, _ := strings.Cut(rel, "/")
		stem, _ := pycompat.PosixSplitext(rest)
		return topics[folder][rest] || topics[folder][stem]
	}, "topic = page path in guides-index.json")
	dclean, dsrc := filepath.Join(base, "devguide-clean"), filepath.Join(base, "devguide")
	devguideSearch := newRow("devguide-search", c.truth.devguide, func(s string) string { return s }, func(rel string) bool {
		return isRegularFile(filepath.Join(dclean, filepath.FromSlash(cleanTextName(rel, dsrc))))
	}, "")
	return []recallRow{guideSearch, guide, devguideSearch}
}

// sampleRows are the find and fuzzy rows on an evenly spaced sample of the
// classes whose name does not start with "_" (both scan the whole corpus per
// query). The first error of a sampled command fails the run.
func (c *recallCheck) sampleRows() ([]recallRow, error) {
	keys := c.truth.classes
	var named []classKey
	for _, k := range keys {
		if !strings.HasPrefix(k.name, "_") {
			named = append(named, k)
		}
	}
	sample := recallSampleOf(named, recallSample)
	var sampleErr error
	in := func(fn func(sub *Env) error, k classKey) bool {
		if sampleErr != nil {
			return false
		}
		out, err := c.e.captured(fn)
		if err != nil {
			sampleErr = err
			return false
		}
		return strings.Contains(out, k.name)
	}
	find := newRow("find", sample, classQName, func(k classKey) bool {
		return in(func(sub *Env) error { return sub.Find(FulltextParams{Query: k.name, Max: 500}) }, k)
	}, fmt.Sprintf("sample %d of %d classes (full corpus scan per query)", len(sample), len(keys)))
	fuzzy := newRow("fuzzy", sample, classQName, func(k classKey) bool {
		return in(func(sub *Env) error { return sub.fuzzy(k.name, 15) }, k)
	}, fmt.Sprintf("sample %d of %d classes", len(sample), len(keys)))
	if sampleErr != nil {
		return nil, sampleErr
	}
	return []recallRow{find, fuzzy}, nil
}
