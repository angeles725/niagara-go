package helpbuild

import (
	"fmt"
	"os"

	"github.com/angeles725/niagara-go/internal/helpbuild/modulexml"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/helpbuild/registry"
	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// indexes/module-index.json: the module descriptor (META-INF/module.xml) of
// every jar of an install's modules/, keyed by module name:
//
//	{"jars": N, "jar_bytes": B,
//	 "modules": {"<name>": modulexml record + "jar"},
//	 "errors": [{"jar", "error"}], "without_descriptor": ["<jar>"]}
//
// A descriptor that cannot be read (corrupt jar, malformed XML, a module name
// another jar already declared) is an "errors" row and a warning, never a
// silent skip. jars and jar_bytes are what freshness compares the registered
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

// readDescriptor reads one jar's descriptor; ok is false for a jar without one.
func readDescriptor(jar string) (m *modulexml.Module, ok bool, err error) {
	z, err := zipfile.Open(jar)
	if err != nil {
		return nil, true, err
	}
	defer z.Close()
	last := -1 // a repeated entry name reads its last copy, as ZipFile.open does
	for i := range z.Infos {
		if z.Infos[i].Name == modulexml.Member {
			last = i
		}
	}
	if last < 0 {
		return nil, false, nil
	}
	data, err := z.ReadEntry(&z.Infos[last], true)
	if err != nil {
		return nil, true, err
	}
	m, err = modulexml.Parse(data)
	return m, true, err
}

// add indexes the descriptor of one jar, or records why it cannot.
func (x *moduleIndex) add(jar string, res *textResult) {
	name := pyBasename(jar)
	x.jars++
	if st, err := os.Stat(jar); err == nil {
		x.bytes += int(st.Size())
	}
	m, ok, err := readDescriptor(jar)
	if !ok {
		x.without = append(x.without, name)
		return
	}
	if err == nil {
		if first, dup := x.owner[m.Name]; dup {
			err = fmt.Errorf("module %s is also declared by %s", m.Name, first)
		}
	}
	if err != nil {
		x.errors = append(x.errors, pyjson.ObjectOf(pyjson.Field{Key: "jar", Value: pyjson.String(name)},
			pyjson.Field{Key: "error", Value: pyjson.String(err.Error())}))
		res.Warnings = append(res.Warnings, fmt.Sprintf("WARNING: %s: %s: %v", moduleIndexScript, name, err))
		return
	}
	x.owner[m.Name] = name
	rec := m.Value()
	rec.Set("jar", pyjson.String(name))
	x.modules.Set(m.Name, rec)
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
// modules/ of the install registered for root. A data root without one (or
// whose install has no modules/) keeps whatever module-index.json it has and
// says why it was not built.
func buildModuleIndex(root string) (*textResult, error) {
	install, err := registry.InstallDir(root)
	if err != nil {
		return nil, err
	}
	switch {
	case install == "":
		return &textResult{Log: []string{moduleIndexName + ": no install registered; skipped (the last copy is kept)"}}, nil
	case !pycompat.IsDir(pycompat.Join(install, "modules")):
		return &textResult{Log: []string{moduleIndexName + ": " + pycompat.Join(install, "modules") + " is not a folder; skipped (the last copy is kept)"}}, nil
	}
	return buildModuleIndexFrom(root, install)
}
