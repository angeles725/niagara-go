package helpbuild

import (
	"os"
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
// full-text page (lexicon.go), the lexicons (lexicon-index.json). In an
// installed jar each is indexed when its index holds it and UNCLASSIFIED when
// it does not (not rebuilt yet) or could not read it; a copy outside modules/
// is excluded, since these indexes read the installed modules/ only.

// shippedKinds name each kind in sweep reasons, and the index holding it.
var shippedKinds = map[string][2]string{
	"descriptor": {"module descriptor", moduleIndexName},
	"template":   {"permissions template", moduleIndexName},
	"schema":     {"schema", moduleIndexName},
	"palette":    {"palette", paletteIndexName},
	"tagdict":    {"tag dictionary", tagIndexName},
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

// shippedIndexes is what the two indexes say about each jar and member: ""
// when held, the reason when it could not be read. A descriptor's member is "".
type shippedIndexes struct{ held map[[2]string]string }

// loadShipped reads module-index.json, palette-index.json, tag-index.json and
// lexicon-index.json under root; an absent index holds nothing.
func loadShipped(root string) (*shippedIndexes, error) {
	x := &shippedIndexes{held: map[[2]string]string{}}
	for _, name := range []string{moduleIndexName, paletteIndexName, tagIndexName, lexiconIndexName} {
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
			for _, key := range []string{"permission_templates", "schemas"} {
				list, _ := rec.Get(key)
				for _, r := range asList(list) {
					p, _ := r.String("path")
					x.held[[2]string{jar, p}] = ""
				}
			}
		}
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
			continue // a jar the palette, tag or lexicon builder could not open: the sweep cannot list it either
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

// heldGap is the UNCLASSIFIED verdict of member of the installed jar named
// jar when idxName does not hold it or could not read it (gap true).
func (s *sweeper) heldGap(jar, member, label, idxName string) (v verdict, gap bool, err error) {
	if s.shippedIdx == nil {
		if s.shippedIdx, err = loadShipped(s.root); err != nil {
			return verdict{}, false, err
		}
	}
	why, ok := s.shippedIdx.held[[2]string{jar, member}]
	switch {
	case !ok:
		return verdict{statusUnclassified, label + " not in " + idxName + " (rebuild-all builds it)", ""}, true, nil
	case why != "":
		return verdict{statusUnclassified, label + " " + why, ""}, true, nil
	}
	return verdict{}, false, nil
}
