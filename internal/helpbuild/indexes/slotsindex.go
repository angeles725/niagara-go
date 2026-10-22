package indexes

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bajadocxml"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// slots_indexer.py: indexes/slots-index.json holds the @NiagaraProperty,
// @NiagaraAction and @NiagaraTopic annotations of source/, then every
// <property>/<action>/<topic> of the .bajadoc XML under devguide/ and
// guides/ that the source did not declare ("origin": "bajadoc").

// slotSections are the annotation kinds and the sections they fill, in
// the order parse_annotations reads them.
var slotSections = []struct{ annotation, section string }{
	{"NiagaraProperty", "properties"},
	{"NiagaraAction", "actions"},
	{"NiagaraTopic", "topics"},
}

// slotSectionOf maps a .bajadoc member kind to its section (_SECTIONS).
var slotSectionOf = map[string]string{"property": "properties", "action": "actions", "topic": "topics"}

// slotsEntry is one slots-index.json value: module, package (absent for a
// source class without one: Python's None), "origin" for a class only the
// XML declares, then the sections in the order they were added.
type slotsEntry struct {
	module   string
	pkg      string
	hasPkg   bool
	bajadoc  bool
	sections []string
	slots    map[string][]strDict
}

func (e *slotsEntry) add(section string, slot strDict) {
	if _, ok := e.slots[section]; !ok {
		e.sections = append(e.sections, section)
	}
	e.slots[section] = append(e.slots[section], slot)
}

// slotsIndex is the index dict: keys in insertion order.
type slotsIndex struct {
	keys    []string
	entries map[string]*slotsEntry
	res     *IndexResult
}

func (x *slotsIndex) put(key string, e *slotsEntry) {
	if _, ok := x.entries[key]; !ok {
		x.keys = append(x.keys, key)
	}
	x.entries[key] = e
}

// collisionKey is collision_key: the first free key of name, "name (q)"
// for each non-empty qualifier, "name (q1, q2)", then a "#n" suffix with
// the warning Python prints on stderr.
func (x *slotsIndex) collisionKey(name string, qualifiers ...string) string {
	keys := []string{name}
	var set []string
	for _, q := range qualifiers {
		if q != "" {
			keys = append(keys, name+" ("+q+")")
			set = append(set, q)
		}
	}
	keys = append(keys, name+" ("+strings.Join(set, ", ")+")")
	for _, k := range keys {
		if _, taken := x.entries[k]; !taken {
			return k
		}
	}
	last := keys[len(keys)-1]
	n := 2
	for {
		if _, taken := x.entries[fmt.Sprintf("%s #%d", last, n)]; !taken {
			break
		}
		n++
	}
	x.res.Stderr = append(x.res.Stderr, fmt.Sprintf("WARNING: slots for %s declared more than once (same module and "+
		"package); kept as '%s #%d'", name, last, n))
	return fmt.Sprintf("%s #%d", last, n)
}

// parseAnnotations is parse_annotations: the class name and package of a
// .java file and the slots of each section; ok false when it cannot be read.
func parseAnnotations(path string) (class, pkg string, hasPkg bool, slots [3][]strDict, ok bool) {
	text, ok := readJavaText(path)
	if !ok {
		return "", "", false, slots, false
	}
	for _, line := range pycompat.SplitLines(text, false) {
		line = pycompat.Strip(line)
		if !hasPkg {
			pkg, hasPkg = javaPackage(line)
		}
		if class = javaDeclaration(line); class != "" {
			break
		}
	}
	if class == "" {
		class = strings.ReplaceAll(filepath.Base(path), ".java", "")
	}
	content := []rune(text)
	for i, s := range slotSections {
		for _, body := range annotationBodies(content, s.annotation) {
			if slot := parseAnnotationBody(body); slot.get("name") != "" {
				slots[i] = append(slots[i], slot)
			}
		}
	}
	return class, pkg, hasPkg, slots, true
}

// BuildSlotsIndex is slots_indexer.build_slots_index(baseDir) (the
// script's __main__ with baseDir as its argument). Each module of source/
// is walked in the file system's directory order (os.walk unsorted), which
// fixes the key order and which same-name class gets which key. err != nil
// is where Python raises (no source/, a .bajadoc that cannot be read, a
// failed write: traceback, exit 1; nothing is written).
func BuildSlotsIndex(baseDir string) (*IndexResult, error) {
	res := &IndexResult{}
	sourceDir := pycompat.Join(baseDir, "source")
	modules, err := pyListdir(sourceDir)
	if err != nil {
		return res, err
	}
	sort.Strings(modules)
	x := &slotsIndex{entries: map[string]*slotsEntry{}, res: res}
	var totals [3]int
	files := 0
	for _, module := range modules {
		modulePath := pycompat.Join(sourceDir, module)
		if !pycompat.IsDir(modulePath) {
			continue
		}
		pycompat.WalkWith(modulePath, pycompat.WalkOptions{Unsorted: true}, func(dir string, dirs, names []string) []string {
			for _, name := range names {
				if !strings.HasSuffix(name, ".java") {
					continue
				}
				class, pkg, hasPkg, slots, _ := parseAnnotations(pycompat.Join(dir, name))
				if len(slots[0]) == 0 && len(slots[1]) == 0 && len(slots[2]) == 0 {
					continue
				}
				files++
				e := &slotsEntry{module: module, pkg: pkg, hasPkg: hasPkg && pkg != "", slots: map[string][]strDict{}}
				for i, s := range slotSections {
					for _, slot := range slots[i] {
						e.add(s.section, slot)
					}
					totals[i] += len(slots[i])
				}
				x.put(x.collisionKey(class, module, e.pkg), e)
			}
			return dirs
		})
	}
	added, err := x.mergeBajadoc(baseDir)
	if err != nil {
		return res, err
	}
	res.out("Merged %d slots from .bajadoc XML", added)
	if err := pyfs.WriteJSON(pycompat.Join(baseDir, "indexes", "slots-index.json"), x.value(), pyfs.DumpIndent2Unicode); err != nil {
		return res, err
	}
	res.out("Built slots-index.json: %d classes (%d properties, %d actions, %d topics) from %d files",
		len(x.keys), totals[0], totals[1], totals[2], files)
	return res, nil
}

// classKey is a by_class key: the simple name and the package, hasPkg
// false for Python's None.
type classKey struct {
	name, pkg string
	hasPkg    bool
}

// mergeBajadoc is merge_bajadoc_slots: the slots of devguide/ then
// guides/ (iter_classes order) that their class's entry lacks; a class
// with no entry gets a new one. It returns the number of slots added.
func (x *slotsIndex) mergeBajadoc(baseDir string) (int, error) {
	byClass := map[classKey]*slotsEntry{}
	for _, key := range x.keys {
		e := x.entries[key]
		name, _, _ := strings.Cut(key, " (")
		if k := (classKey{name, e.pkg, e.hasPkg}); byClass[k] == nil {
			byClass[k] = e
		}
	}
	added := 0
	for _, d := range []string{"devguide", "guides"} {
		root := pycompat.Join(baseDir, d)
		if !pycompat.IsDir(root) {
			continue
		}
		var paths []string
		bajadocWalk(root, func(path, _ string) { paths = append(paths, path) })
		for _, f := range parseBajadocFiles(paths) {
			if f.err != nil {
				return added, f.err
			}
			for _, cls := range f.classes {
				added += x.mergeClass(byClass, cls)
			}
		}
	}
	return added, nil
}

// mergeClass merges the slots of one .bajadoc class.
func (x *slotsIndex) mergeClass(byClass map[classKey]*slotsEntry, cls bajadocxml.Class) int {
	var members []bajadocxml.Member
	for _, m := range cls.Members {
		if slotSectionOf[m.Kind] != "" {
			members = append(members, m)
		}
	}
	if len(members) == 0 {
		return 0
	}
	k := classKey{cls.Name, cls.Pkg, true}
	e := byClass[k]
	if e == nil {
		e = &slotsEntry{module: cls.Module, pkg: cls.Pkg, hasPkg: true, bajadoc: true, slots: map[string][]strDict{}}
		q := cls.Module
		if q == "" {
			q = cls.Pkg
		}
		x.put(x.collisionKey(cls.Name, q, cls.Pkg), e)
		byClass[k] = e
	}
	added := 0
	for _, m := range members {
		sec := slotSectionOf[m.Kind]
		have := false
		for _, s := range e.slots[sec] {
			have = have || s.get("name") == m.Name
		}
		if !have {
			e.add(sec, xmlSlot(m))
			added++
		}
	}
	return added
}

// xmlSlot is _xml_slot.
func xmlSlot(m bajadocxml.Member) strDict {
	slot := strDict{{"name", m.Name}, {"origin", "bajadoc"}}
	switch m.Kind {
	case "property":
		slot.set("type", bajadocxml.Simple(m.Typ))
	case "action":
		if len(m.Params) > 0 {
			slot.set("parameterType", bajadocxml.Simple(m.Params[0][0]))
		}
		if m.Ret != "" && m.Ret != "void" {
			slot.set("returnType", bajadocxml.Simple(m.Ret))
		}
	default:
		if m.Typ != "" {
			slot.set("eventType", bajadocxml.Simple(m.Typ))
		}
	}
	return slot
}

// value is the index as json.dumps sees it.
func (x *slotsIndex) value() pyjson.Value {
	out := pyjson.NewObject()
	for _, key := range x.keys {
		e := x.entries[key]
		v := pyjson.NewObject()
		v.Set("module", pyjson.String(e.module))
		if e.hasPkg {
			v.Set("package", pyjson.String(e.pkg))
		}
		if e.bajadoc {
			v.Set("origin", pyjson.String("bajadoc"))
		}
		for _, sec := range e.sections {
			list := make([]pyjson.Value, len(e.slots[sec]))
			for i, slot := range e.slots[sec] {
				o := pyjson.NewObject()
				for _, kv := range slot {
					o.Set(kv.key, pyjson.String(kv.val))
				}
				list[i] = o
			}
			v.Set(sec, pyjson.Value{Kind: pyjson.Array, A: list})
		}
		out.Set(key, v)
	}
	return out
}
