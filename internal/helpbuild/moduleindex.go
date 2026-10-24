package helpbuild

import (
	"fmt"
	"os"

	"github.com/angeles725/niagara-go/internal/helpbuild/modulexml"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/helpbuild/registry"
	"github.com/angeles725/niagara-go/internal/helpbuild/shippedxml"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// indexes/module-index.json: the module descriptor (META-INF/module.xml) of
// every jar of an install's modules/, keyed by module name:
//
//	{"jars": N, "jar_bytes": B,
//	 "modules": {"<name>": modulexml record + "jar"
//	   + "permission_templates" and "schemas" (shippedxml records) when it ships any},
//	 "errors": [{"jar", "path"?, "error"}], "without_descriptor": ["<jar>"]}
//
// A descriptor that cannot be read (corrupt jar, malformed XML, a module name
// another jar already declared) is an "errors" row and a warning, never a
// silent skip; so is a template or schema that cannot be read (its row
// carries its "path"; the module is still indexed) or that a jar without a
// descriptor ships. jars and jar_bytes are what freshness compares the registered
// install with (the install folder itself is the registry's, not kept here).
const (
	moduleIndexName   = "module-index.json"
	moduleIndexScript = "module_index_builder"
)

// moduleIndex is the index being built.
type moduleIndex struct {
	jars, bytes int
	modules     pyjson.Value
	owner       map[string]string // module name -> the jar that declared it first
	errors      []pyjson.Value
	without     []string
}

// shipped reports whether a jar member is read into the jar's module record:
// its descriptor, a module-permissions template or a schema.
func shipped(name string) bool {
	return name == modulexml.Member || shippedxml.IsPermissionsTemplate(name) || shippedxml.IsSchema(name)
}

// jarFiles is what one jar ships that module-index.json records.
type jarFiles struct {
	module  *modulexml.Module // nil for a jar without a descriptor
	records map[string][]pyjson.Value
	errs    [][2]string // member path, why it could not be read
}

// readJar reads the descriptor, permission templates and schemas of a jar;
// err is for the jar or its descriptor, a template or schema that cannot be
// read is one of errs.
func readJar(jar string) (*jarFiles, error) {
	members, err := readMembers(jar, shipped)
	if err != nil {
		return nil, err
	}
	f := &jarFiles{records: map[string][]pyjson.Value{}}
	for _, m := range members {
		if m.name == modulexml.Member {
			if m.err == nil {
				f.module, m.err = modulexml.Parse(m.data)
			}
			if m.err != nil {
				return nil, m.err
			}
			continue
		}
		key, v := "schemas", pyjson.Value{}
		if m.err == nil && shippedxml.IsSchema(m.name) {
			var s shippedxml.Schema
			s, m.err = shippedxml.ParseSchema(m.name, m.data)
			v = s.Value()
		} else if m.err == nil {
			var p *shippedxml.Permissions
			if p, m.err = shippedxml.ParsePermissions(m.name, m.data); m.err == nil {
				key, v = "permission_templates", p.Value()
			}
		}
		if m.err != nil {
			f.errs = append(f.errs, [2]string{m.name, m.err.Error()})
			continue
		}
		f.records[key] = append(f.records[key], v)
	}
	return f, nil
}

// fail records why jar (or its member path, when not "") cannot be indexed.
func (x *moduleIndex) fail(res *textResult, jar, path, why string) {
	row := pyjson.ObjectOf(pyjson.Field{Key: "jar", Value: pyjson.String(jar)})
	where := jar
	if path != "" {
		row.Set("path", pyjson.String(path))
		where += "!" + path
	}
	row.Set("error", pyjson.String(why))
	x.errors = append(x.errors, row)
	res.Warnings = append(res.Warnings, fmt.Sprintf("WARNING: %s: %s: %s", moduleIndexScript, where, why))
}

// add indexes the descriptor of one jar, or records why it cannot. A jar
// without a descriptor that ships templates or schemas has them recorded as
// errors: there is no module record to hold them.
func (x *moduleIndex) add(jar string, res *textResult) {
	name := pyBasename(jar)
	x.jars++
	if st, err := os.Stat(jar); err == nil {
		x.bytes += int(st.Size())
	}
	f, err := readJar(jar)
	if err == nil && f.module != nil {
		if first, dup := x.owner[f.module.Name]; dup {
			err = fmt.Errorf("module %s is also declared by %s", f.module.Name, first)
		}
	}
	if err != nil {
		x.fail(res, name, "", err.Error())
		return
	}
	for _, e := range f.errs {
		x.fail(res, name, e[0], e[1])
	}
	if f.module == nil {
		x.without = append(x.without, name)
		for _, k := range []string{"permission_templates", "schemas"} {
			for _, r := range f.records[k] {
				path, _ := r.Get("path")
				x.fail(res, name, path.S, "shipped by a jar without META-INF/module.xml (no module record holds it)")
			}
		}
		return
	}
	x.owner[f.module.Name] = name
	rec := f.module.Value()
	rec.Set("jar", pyjson.String(name))
	for _, k := range []string{"permission_templates", "schemas"} {
		if len(f.records[k]) > 0 {
			rec.Set(k, pyjson.Value{Kind: pyjson.Array, A: f.records[k]})
		}
	}
	x.modules.Set(f.module.Name, rec)
}

// buildModuleIndexFrom writes root/indexes/module-index.json from the jars of
// installDir/modules.
func buildModuleIndexFrom(root, installDir string) (*textResult, error) {
	jars, err := lexiconJars(installDir)
	if err != nil {
		return nil, err
	}
	res := &textResult{}
	x := &moduleIndex{modules: pyjson.NewObject(), owner: map[string]string{}}
	for _, jar := range jars {
		x.add(jar, res)
	}
	out := pycompat.Join(root, "indexes", moduleIndexName)
	v := pyjson.ObjectOf(
		pyjson.Field{Key: "jars", Value: pyjson.IntValue(int64(x.jars))},
		pyjson.Field{Key: "jar_bytes", Value: pyjson.IntValue(int64(x.bytes))},
		pyjson.Field{Key: "modules", Value: x.modules},
		pyjson.Field{Key: "errors", Value: pyjson.Value{Kind: pyjson.Array, A: x.errors}},
		pyjson.Field{Key: "without_descriptor", Value: pyjson.StringList(x.without)},
	)
	if err := pyfs.WriteJSON(out, v, pyfs.DumpIndent2Unicode); err != nil {
		return nil, err
	}
	res.Files = len(x.owner)
	res.Log = append(res.Log, fmt.Sprintf("%s: %d modules from %d jars (%d without module.xml, %d errors) -> %s",
		moduleIndexName, res.Files, x.jars, len(x.without), len(x.errors), out))
	return res, nil
}

// buildModuleIndex is the rebuild-all step: module-index.json from the
// modules/ of the install registered for root.
var buildModuleIndex = fromInstall(moduleIndexName, buildModuleIndexFrom)

// fromInstall is the rebuild-all step that writes name with build from the
// modules/ of the install registered for root. A data root without one (or
// whose install has no modules/) keeps whatever copy of name it has and says
// why it was not built.
func fromInstall(name string, build func(root, install string) (*textResult, error)) func(string) (*textResult, error) {
	return func(root string) (*textResult, error) {
		install, err := registry.InstallDir(root)
		if err != nil {
			return nil, err
		}
		switch {
		case install == "":
			return &textResult{Log: []string{name + ": no install registered; skipped (the last copy is kept)"}}, nil
		case !pycompat.IsDir(pycompat.Join(install, "modules")):
			return &textResult{Log: []string{name + ": " + pycompat.Join(install, "modules") + " is not a folder; skipped (the last copy is kept)"}}, nil
		}
		return build(root, install)
	}
}
