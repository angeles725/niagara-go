package helpbuild

import (
	"os"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/modulexml"
	"github.com/angeles725/niagara-go/internal/helpbuild/shippedxml"
	"github.com/angeles725/niagara-go/internal/index"
)

// The coverage sweep classifies what module jars ship besides documentation
// and nh indexes in a structure, not in the full-text index: the module
// descriptor and the module-permissions templates and schemas
// (module-index.json), palettes (palette-index.json), the tag dictionaries
// of the haystack and tagdictionary jars (tag-index.json) and, besides their
// full-text page (lexicon.go), the lexicons (lexicon-index.json), and the
// class files of the types module.xml registers (module-index.json names
// them, bytecode-index.json holds them; other classes stay outside the
// scope, counted by type). In an installed jar each is indexed when its index
// holds it and UNCLASSIFIED when it does not (not rebuilt yet) or could not
// read it; a copy outside modules/ is excluded, since these indexes read the
// installed modules/ only (a registered class outside modules/ stays an
// out-of-scope .class).

// shippedKinds name each kind in sweep reasons, and the index holding it.
var shippedKinds = map[string][2]string{
	"descriptor": {"module descriptor", moduleIndexName},
	"template":   {"permissions template", moduleIndexName},
	"schema":     {"schema", moduleIndexName},
	"palette":    {"palette", paletteIndexName},
	"tagdict":    {"tag dictionary", tagIndexName},
	"class":      {"registered class", bytecodeIndexName},
}

// isDescriptor reports whether entry is a module jar's descriptor.
func isDescriptor(entry string) bool { return entry == modulexml.Member }

// shippedKind is the kind of entry of the jar at install path rel the sweep
// checks against an index, "" for any other entry. A doc-like entry stays
// with the documentation rules.
func shippedKind(rel, entry string, rules *Rules) string {
	switch {
	case isDescriptor(entry):
		return "descriptor"
	case rules.IsDocLike(entry):
		return ""
	case isPalette(entry):
		return "palette"
	case isTagDictionary(pyBasename(rel), entry):
		return "tagdict"
	case shippedxml.IsPermissionsTemplate(entry):
		return "template"
	case shippedxml.IsSchema(entry):
		return "schema"
	}
	return ""
}

// shippedIndexes is what the indexes say about each jar and member: "" when
// held, the reason when it could not be read (a descriptor's member is ""),
// and the class files each jar's module.xml registers, by jar.
type shippedIndexes struct {
	held       map[[2]string]string
	registered map[string][]string
}

// loadShipped reads module-index.json, palette-index.json, tag-index.json,
// lexicon-index.json and bytecode-index.json under root; an absent index
// holds nothing.
func loadShipped(root string) (*shippedIndexes, error) {
	x := &shippedIndexes{held: map[[2]string]string{}, registered: map[string][]string{}}
	for _, name := range []string{moduleIndexName, paletteIndexName, tagIndexName, lexiconIndexName, bytecodeIndexName} {
		path := index.Path(root, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		idx, err := index.LoadObject(path)
		if err != nil {
			return nil, err
		}
		x.read(name, idx)
	}
	return x, nil
}

func (x *shippedIndexes) read(name string, idx *index.Object) {
	if mods := idx.Object("modules"); mods != nil {
		for _, k := range mods.Keys() {
			rec := mods.Object(k)
			jar, _ := rec.String("jar")
			x.held[[2]string{jar, ""}] = ""
			types, _ := rec.Get("types")
			for _, t := range asList(types) {
				if c, _ := t.String("class"); c != "" && !slices.Contains(x.registered[jar], classMember(c)) {
					x.registered[jar] = append(x.registered[jar], classMember(c))
				}
			}
			for _, key := range []string{"permission_templates", "schemas"} {
				list, _ := rec.Get(key)
				for _, r := range asList(list) {
					p, _ := r.String("path")
					x.held[[2]string{jar, p}] = ""
				}
			}
		}
	}
	classes, _ := idx.Get("classes") // bytecode-index.json's registered classes
	for _, r := range asList(classes) {
		jar, _ := r.String("jar")
		c, _ := r.String("class")
		x.held[[2]string{jar, classMember(c)}] = ""
	}
	pals, _ := idx.Get("palettes")
	files, _ := idx.Get("files") // tag-index.json's dictionary files, lexicon-index.json's lexicons
	for _, r := range append(asList(pals), asList(files)...) {
		jar, _ := r.String("jar")
		p, _ := r.String("path")
		x.held[[2]string{jar, p}] = ""
	}
	errs, _ := idx.Get("errors")
	for _, r := range asList(errs) {
		jar, _ := r.String("jar")
		p, _ := r.String("path") // a module-index row without a path is the descriptor's
		if p == "" && name != moduleIndexName {
			continue // a jar the palette, tag, lexicon or bytecode builder could not open: the sweep cannot list it either
		}
		x.held[[2]string{jar, p}] = name + " could not read: " + getErr(r)
	}
}

func getErr(r *index.Object) string { why, _ := r.String("error"); return why }

// asList is the objects of a JSON list value.
func asList(v any) []*index.Object {
	l, _ := v.([]any)
	out := make([]*index.Object, 0, len(l))
	for _, e := range l {
		if o, ok := e.(*index.Object); ok {
			out = append(out, o)
		}
	}
	return out
}

// shipped is the verdict of entry, of the given kind, in the jar at install path rel.
func (s *sweeper) shipped(rel, entry, kind string) (verdict, error) {
	label, idxName := shippedKinds[kind][0], shippedKinds[kind][1]
	if !strings.HasPrefix(rel, "modules/") || strings.Count(rel, "/") != 1 {
		if kind == "descriptor" {
			return verdict{statusExcluded, "module descriptor of a jar outside modules/ (module-index.json reads " +
				"the installed modules/ only)", "module-descriptor-outside-modules"}, nil
		}
		return verdict{statusExcluded, label + " of a jar outside modules/ (" + idxName + " reads the installed " +
			"modules/ only)", "shipped-file-outside-modules"}, nil
	}
	member := entry
	if kind == "descriptor" {
		member = ""
	}
	if v, gap, err := s.heldGap(pyBasename(rel), member, label, idxName); gap || err != nil {
		return v, err
	}
	return verdict{statusIndexed, idxName, strings.TrimSuffix(idxName, ".json")}, nil
}

// shippedIndexes loads what the shipped-file indexes hold on first use.
func (s *sweeper) shippedIndexes() (*shippedIndexes, error) {
	if s.shippedIdx == nil {
		x, err := loadShipped(s.root)
		if err != nil {
			return nil, err
		}
		s.shippedIdx = x
	}
	return s.shippedIdx, nil
}

// heldGap is the UNCLASSIFIED verdict of member of the installed jar named
// jar when idxName does not hold it or could not read it (gap true).
func (s *sweeper) heldGap(jar, member, label, idxName string) (v verdict, gap bool, err error) {
	x, err := s.shippedIndexes()
	if err != nil {
		return verdict{}, false, err
	}
	why, ok := x.held[[2]string{jar, member}]
	switch {
	case !ok:
		return verdict{statusUnclassified, label + " not in " + idxName + " (rebuild-all builds it)", ""}, true, nil
	case why != "":
		return verdict{statusUnclassified, label + " " + why, ""}, true, nil
	}
	return verdict{}, false, nil
}

// classFiles counts the n class files of the jar at install path rel (the
// listing keeps only their number): in an installed jar, the verdict of each
// one its module.xml registers (bytecode-index.json holds it, could not read
// it or lacks it); the rest are outside the documentation scope.
func (s *sweeper) classFiles(res *sweepResult, row *sweepRow, rel string, n int) error {
	if isInstalledJar(rel) {
		x, err := s.shippedIndexes()
		if err != nil {
			return err
		}
		label, idxName := shippedKinds["class"][0], shippedKinds["class"][1]
		members := x.registered[pyBasename(rel)]
		for _, m := range members {
			v, gap, err := s.heldGap(pyBasename(rel), m, label, idxName)
			if err != nil {
				return err
			}
			if !gap {
				v = verdict{statusIndexed, idxName, strings.TrimSuffix(idxName, ".json")}
			}
			res.count(row, v, rel, m)
		}
		n -= min(len(members), n)
	}
	if n > 0 {
		res.OutOfScope[".class"] += n
	}
	return nil
}
