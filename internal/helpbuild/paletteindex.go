package helpbuild

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/palettexml"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// indexes/palette-index.json: every *.palette of every jar of an install's
// modules/, in jar order:
//
//	{"jars": N, "jar_bytes": B,
//	 "palettes": [{"module", "jar", "path", "folders", "entries", "tree"}],
//	 "errors": [{"jar", "path", "error"}]}
//
// "module" is the jar's name without .jar (the module name nh module takes);
// "tree" is palettexml's Value. A palette (or jar) that cannot be read is an
// "errors" row and a warning, never a silent skip; jars and jar_bytes are what
// freshness compares the registered install with.
const (
	paletteIndexName   = "palette-index.json"
	paletteIndexScript = "palette_index_builder"
)

// isPalette reports whether a jar member is a palette file.
func isPalette(name string) bool { return strings.HasSuffix(name, ".palette") }

// paletteIndex is the index being built.
type paletteIndex struct {
	jarCensus
	entries  int
	palettes []pyjson.Value
}

// add indexes the palettes of one jar, or records why it cannot.
func (x *paletteIndex) add(jar string, res *textResult) {
	name := x.count(jar)
	members, err := readMembers(jar, isPalette)
	if err != nil {
		x.fail(res, name, "", err)
		return
	}
	for _, m := range members {
		p, err := m.palette()
		if err != nil {
			x.fail(res, name, m.name, err)
			continue
		}
		c := p.Counts()
		x.entries += c.Entries
		x.palettes = append(x.palettes, pyjson.ObjectOf(
			pyjson.Field{Key: "module", Value: pyjson.String(name[:len(name)-len(".jar")])},
			pyjson.Field{Key: "jar", Value: pyjson.String(name)},
			pyjson.Field{Key: "path", Value: pyjson.String(m.name)},
			pyjson.Field{Key: "folders", Value: pyjson.IntValue(int64(c.Folders))},
			pyjson.Field{Key: "entries", Value: pyjson.IntValue(int64(c.Entries))},
			pyjson.Field{Key: "tree", Value: p.Value()}))
	}
}

func (m jarMember) palette() (*palettexml.Palette, error) {
	if m.err != nil {
		return nil, m.err
	}
	return palettexml.Parse(m.data)
}

// buildPaletteIndexFrom writes root/indexes/palette-index.json from the jars
// of installDir/modules.
func buildPaletteIndexFrom(root, installDir string) (*textResult, error) {
	jars, err := lexiconJars(installDir)
	if err != nil {
		return nil, err
	}
	res := &textResult{}
	x := &paletteIndex{jarCensus: jarCensus{script: paletteIndexScript}}
	for _, jar := range jars {
		x.add(jar, res)
	}
	out := pycompat.Join(root, "indexes", paletteIndexName)
	v := pyjson.ObjectOf(
		pyjson.Field{Key: "jars", Value: pyjson.IntValue(int64(x.jars))},
		pyjson.Field{Key: "jar_bytes", Value: pyjson.IntValue(int64(x.bytes))},
		pyjson.Field{Key: "palettes", Value: pyjson.Value{Kind: pyjson.Array, A: x.palettes}},
		pyjson.Field{Key: "errors", Value: pyjson.Value{Kind: pyjson.Array, A: x.errors}},
	)
	if err := pyfs.WriteJSON(out, v, pyfs.DumpIndent2Unicode); err != nil {
		return nil, err
	}
	res.Files = len(x.palettes)
	res.Log = append(res.Log, fmt.Sprintf("%s: %d palettes (%d entries) from %d jars (%d errors) -> %s",
		paletteIndexName, res.Files, x.entries, x.jars, len(x.errors), out))
	return res, nil
}

// buildPaletteIndex is the rebuild-all step: palette-index.json from the
// modules/ of the install registered for root.
var buildPaletteIndex = fromInstall(paletteIndexName, buildPaletteIndexFrom)
