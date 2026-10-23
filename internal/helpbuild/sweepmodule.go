package helpbuild

import (
	"os"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/modulexml"
	"github.com/angeles725/niagara-go/internal/index"
)

// The coverage sweep classifies META-INF/module.xml like documentation: its
// content is indexed in module-index.json (module_index_builder), not in the
// full-text index, so it is no longer counted outside the documentation scope.

// moduleIndexJars is what module-index.json says about each installed jar:
// "" for a jar whose module it holds, the reason for one it could not read.
// An absent index holds nothing.
func moduleIndexJars(root string) (map[string]string, error) {
	out := map[string]string{}
	path := index.Path(root, moduleIndexName)
	if _, err := os.Stat(path); err != nil {
		return out, nil
	}
	idx, err := index.LoadObject(path)
	if err != nil {
		return nil, err
	}
	if mods := idx.Object("modules"); mods != nil {
		for _, k := range mods.Keys() {
			if jar, ok := mods.Object(k).String("jar"); ok {
				out[jar] = ""
			}
		}
	}
	errs, _ := idx.Get("errors")
	list, _ := errs.([]any)
	for _, e := range list {
		if o, ok := e.(*index.Object); ok {
			jar, _ := o.String("jar")
			why, _ := o.String("error")
			out[jar] = "module-index.json could not read: " + why
		}
	}
	return out, nil
}

// descriptor is the verdict of the module.xml of the jar at install path rel.
func (s *sweeper) descriptor(rel string) (verdict, error) {
	if !strings.HasPrefix(rel, "modules/") || strings.Count(rel, "/") != 1 {
		return verdict{statusExcluded, "module descriptor of a jar outside modules/ (module-index.json reads " +
			"the installed modules/ only)", "module-descriptor-outside-modules"}, nil
	}
	if s.moduleJars == nil {
		jars, err := moduleIndexJars(s.root)
		if err != nil {
			return verdict{}, err
		}
		s.moduleJars = jars
	}
	why, ok := s.moduleJars[pyBasename(rel)]
	switch {
	case !ok:
		return verdict{statusUnclassified, "module descriptor not in module-index.json (rebuild-all builds it)", ""}, nil
	case why != "":
		return verdict{statusUnclassified, "module descriptor " + why, ""}, nil
	}
	return verdict{statusIndexed, moduleIndexName, "module-index"}, nil
}

// isDescriptor reports whether entry is a module jar's descriptor.
func isDescriptor(entry string) bool { return entry == modulexml.Member }
