package modulefind

import (
	"errors"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/blockfiles"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// omap is a Python dict: values by key, iterated in first-insertion order.
type omap[V any] struct {
	keys []string
	m    map[string]V
}

func newOmap[V any]() *omap[V] { return &omap[V]{m: map[string]V{}} }

func (o *omap[V]) get(k string) (V, bool) { v, ok := o.m[k]; return v, ok }

func (o *omap[V]) set(k string, v V) {
	if _, ok := o.m[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.m[k] = v
}

func (o *omap[V]) sortedKeys() []string {
	ks := append([]string(nil), o.keys...)
	sort.Strings(ks)
	return ks
}

// slotInfo is one @NiagaraProperty: type, MIN facet (nil = None) and the flags expression.
type slotInfo struct {
	typ   string
	min   *float64
	flags string
}

// module is module-find.py Module: the parsed source tree, keyed by class name.
type module struct {
	root       string
	slots      *omap[*omap[slotInfo]] // class -> slot -> declaration
	actions    *omap[*omap[string]]   // class -> action -> flag character
	extends    *omap[string]          // class -> superclass
	writers    map[string]map[string]bool
	dynWriters map[string]map[string]bool
	ords       [][2]string // (class, ORD literal) in scan order
	files      int
	unreadable map[string]string // path -> OS error text
}

var (
	setLiteralRe = pyre.MustCompile(`\.set\(\s*"([^"]+)"\s*,`, 0)
	setterRe     = pyre.MustCompile(`\bset([A-Z]\w*)\s*\(`, 0)
	dynSetRe     = pyre.MustCompile(`(\w+)\.set\(\s*([A-Za-z_]\w*)\s*,`, 0)
	ordRe        = pyre.MustCompile(`"((?:slot|station|local|history|h|module|file):[^"]+)"`, 0)
	segmentRe    = pyre.MustCompile(`@Niagara(?:Property|Action)\b.*?(?=@Niagara(?:Property|Action)\b|\Z)`, pyre.DOTALL)
	nameRe       = pyre.MustCompile(`name\s*=\s*"([^"]+)"`, 0)
	typeRe       = pyre.MustCompile(`type\s*=\s*"([^"]+)"`, 0)
	flagsRe      = pyre.MustCompile(`flags\s*=\s*([^,)]+)`, 0)
	minRe        = pyre.MustCompile(`BFacets\.MIN[^,)]*,\s*BDouble\.make\(\s*(-?[0-9.]+)[dDfFlL]?\s*\)`, 0)
	actionFlagRe = pyre.MustCompile(`flags\s*=\s*Flags\.(\w+)`, 0)
	// complexType is _COMPLEX_TYPE: StatusNumeric/Boolean/Enum as "baja:Status*" or "BStatus*".
	complexType = pyre.MustCompile(`(?:baja:|B)?Status\w+$`, 0)
)

var flagMap = map[string]string{"HIDDEN": "h", "OPERATOR": "o", "READONLY": "r", "SUMMARY": "s", "TRANSIENT": "t"}

func newModule(root string) *module {
	m := &module{root: root, slots: newOmap[*omap[slotInfo]](), actions: newOmap[*omap[string]](),
		extends: newOmap[string](), writers: map[string]map[string]bool{},
		dynWriters: map[string]map[string]bool{}, unreadable: map[string]string{}}
	walkJava(root, func(path, name string) {
		content, err := blockfiles.ReadTextReplace(path)
		if err != nil {
			m.unreadable[path] = blockfiles.Strerror(err)
			return
		}
		m.files++
		m.scanFile(strings.TrimSuffix(name, ".java"), content)
	})
	return m
}

// walkJava calls fn for every .java file under root in os.walk order: the sorted files of
// a directory, then its sorted subdirectories; dot-dirs pruned, symlinked dirs not entered.
func walkJava(root string, fn func(path, name string)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	var dirs []string
	for _, e := range entries { // os.ReadDir sorts by name
		name, path := e.Name(), pyJoin(root, e.Name())
		if e.IsDir() {
			if !strings.HasPrefix(name, ".") {
				dirs = append(dirs, path)
			}
			continue
		}
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(path); err == nil && st.IsDir() {
				continue
			}
		}
		if strings.HasSuffix(name, ".java") {
			fn(path, name)
		}
	}
	for _, d := range dirs {
		walkJava(d, fn)
	}
}

// pyJoin is posixpath.join(a, b) for a relative b: no cleaning, so paths print as Python's.
func pyJoin(a, b string) string {
	if a == "" || strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

// sortedSet is sorted(a Python set).
func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func addTo(set map[string]map[string]bool, key, value string) {
	if set[key] == nil {
		set[key] = map[string]bool{}
	}
	set[key][value] = true
}

func (m *module) scanFile(cls, content string) {
	text := []rune(content)
	extendsRe := pyre.MustCompile(`\bclass\s+`+escape(cls)+`\s+extends\s+(\w+)`, 0)
	if mt := extendsRe.SearchRunes(text, 0); mt != nil {
		m.extends.set(cls, mt.Group(1))
	}
	for _, mt := range setLiteralRe.FindIter(text) {
		addTo(m.writers, mt.Group(1), cls+`: .set("`+mt.Group(1)+`", ...)`)
	}
	for _, mt := range setterRe.FindIter(text) {
		g := []rune(mt.Group(1))
		slot := string(unicode.ToLower(g[0])) + string(g[1:])
		addTo(m.writers, slot, cls+": set"+mt.Group(1)+"(...)")
	}
	for _, mt := range dynSetRe.FindIter(text) {
		addTo(m.dynWriters, cls, cls+": "+mt.Group(1)+".set("+mt.Group(2)+", ...)  [slot resolved at runtime]")
	}
	for _, mt := range ordRe.FindIter(text) {
		m.ords = append(m.ords, [2]string{cls, mt.Group(1)})
	}
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); {
		ln := lines[i]
		if !strings.Contains(ln, "@NiagaraProperty") && !strings.Contains(ln, "@NiagaraAction") {
			i++
			continue
		}
		// paren-balanced join of a multi-line annotation (bog-audit D9)
		buf := ln
		depth := strings.Count(ln, "(") - strings.Count(ln, ")")
		j := i + 1
		for depth > 0 && j < len(lines) {
			buf += " " + lines[j]
			depth += strings.Count(lines[j], "(") - strings.Count(lines[j], ")")
			j++
		}
		i = j
		// one joined span may hold several annotations (@NiagaraProperties({ .., .. }))
		for _, seg := range segmentRe.FindAll(buf) {
			m.scanSegment(cls, seg)
		}
	}
}

func (m *module) scanSegment(cls, seg string) {
	nm := nameRe.Search(seg)
	if nm == nil {
		return
	}
	name := nm.Group(1)
	if strings.HasPrefix(seg, "@NiagaraProperty") {
		info := slotInfo{}
		if tm := typeRe.Search(seg); tm != nil {
			info.typ = tm.Group(1)
		}
		if fm := flagsRe.Search(seg); fm != nil {
			info.flags = strings.Join(pycompat.Fields(fm.Group(1)), " ")
		}
		if mm := minRe.Search(seg); mm != nil {
			if v, ok := pyFloat(mm.Group(1)); ok {
				info.min = &v
			}
		}
		slots, ok := m.slots.get(cls)
		if !ok {
			slots = newOmap[slotInfo]()
			m.slots.set(cls, slots)
		}
		slots.set(name, info)
		return
	}
	if strings.HasPrefix(seg, "@NiagaraAction") {
		flag := "o"
		if fm := actionFlagRe.Search(seg); fm != nil {
			fw := strings.ToUpper(fm.Group(1))
			if c, known := flagMap[fw]; known {
				flag = c
			} else if fw != "" {
				r, _ := utf8.DecodeRuneInString(fw)
				flag = strings.ToLower(string(r))
			}
		}
		acts, ok := m.actions.get(cls)
		if !ok {
			acts = newOmap[string]()
			m.actions.set(cls, acts)
		}
		acts.set(name, flag)
	}
}

// pyFloat is float(text) for the -?[0-9.]+ the MIN regex captures: false on ValueError
// ("1.2.3", "."); a value beyond float range is ±inf, as in Python.
func pyFloat(text string) (float64, bool) {
	v, err := strconv.ParseFloat(text, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	return v, true
}

// escape is re.escape for a literal inside a pattern.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// resolveChain is Module.resolve_chain: cls, its superclass, ... until unknown or a cycle.
func (m *module) resolveChain(cls string) []string {
	chain, seen := []string{cls}, map[string]bool{}
	for {
		sup, ok := m.extends.get(cls)
		if !ok || seen[cls] {
			return chain
		}
		seen[cls] = true
		cls = sup
		chain = append(chain, cls)
	}
}

type srcLine struct {
	path string
	n    int
	text string // with its "\n", as Python iterates a file
}

// javaLines is modulefind.sources.java_lines: every line of every .java file; a file
// that cannot be read is recorded in unreadable.
func javaLines(root string, unreadable map[string]string) []srcLine {
	var out []srcLine
	walkJava(root, func(path, _ string) {
		text, err := blockfiles.ReadTextReplace(path)
		if err != nil {
			unreadable[path] = blockfiles.Strerror(err)
			return
		}
		for i, line := range strings.SplitAfter(text, "\n") {
			if line != "" {
				out = append(out, srcLine{path, i + 1, line})
			}
		}
	})
	return out
}
