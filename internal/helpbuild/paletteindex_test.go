package helpbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
)

const fixturePalette = `<?xml version="1.0" encoding="UTF-8"?>
<bajaObjectGraph version="1.0">
<p m="b=baja" t="b:UnrestrictedFolder">
 <p n="Logic" t="b:UnrestrictedFolder">
  <p n="BooleanLatch" m="k=kitControl" t="k:BooleanLatch"/>
 </p>
</p>
</bajaObjectGraph>
`

// paletteFixtureInstall is an install whose modules/ holds a jar with a
// palette, a jar with two (one malformed), a jar without any, a corrupt jar
// and a file that is not a jar.
func paletteFixtureInstall(t *testing.T) string {
	t.Helper()
	install := t.TempDir()
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "kitControl-rt.jar"), [2]string{"module.palette", fixturePalette})
	testkit.WriteJar(t, filepath.Join(mods, "webs-rt.jar"), [2]string{"module.palette", fixturePalette},
		[2]string{"rc/webs.palette", `<bajaObjectGraph><p t="q:Folder"/></bajaObjectGraph>`})
	testkit.WriteJar(t, filepath.Join(mods, "plain-rt.jar"), [2]string{"a.txt", "no palette"})
	for name, data := range map[string]string{"corrupt-rt.jar": "not a zip", "README.txt": "not a jar"} {
		if err := os.WriteFile(filepath.Join(mods, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return install
}

// TestPaletteIndexReadsEveryJar: every palette of every jar is indexed with
// its module, path and tree; a malformed palette and a corrupt jar are
// recorded with the reason and printed as warnings, never dropped.
func TestPaletteIndexReadsEveryJar(t *testing.T) {
	install, root := paletteFixtureInstall(t), t.TempDir()
	res, err := buildPaletteIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := index.LoadObject(index.Path(root, paletteIndexName))
	if err != nil {
		t.Fatal(err)
	}
	pals, _ := idx.Get("palettes")
	var got []string
	for _, p := range pals.([]any) {
		o := p.(*index.Object)
		mod, _ := o.String("module")
		path, _ := o.String("path")
		entries, _ := o.Get("entries")
		tree, _ := o.Get("tree")
		leaf := tree.([]any)[0].(*index.Object)
		folder, _ := leaf.String("folder")
		got = append(got, mod+" "+path+" "+fmtAny(entries)+" "+folder)
	}
	if want := "kitControl-rt module.palette 1 Logic|webs-rt module.palette 1 Logic"; strings.Join(got, "|") != want {
		t.Errorf("palettes = %s", strings.Join(got, "|"))
	}
	errs, _ := idx.Get("errors")
	got = nil
	for _, e := range errs.([]any) {
		o := e.(*index.Object)
		jar, _ := o.String("jar")
		path, _ := o.String("path")
		why, _ := o.String("error")
		got = append(got, jar+"!"+path+": "+why)
	}
	want := "corrupt-rt.jar!: bad zip file|webs-rt.jar!rc/webs.palette: type t=\"q:Folder\" names an undeclared module prefix"
	if strings.Join(got, "|") != want {
		t.Errorf("errors = %s", strings.Join(got, "|"))
	}
	if jars, _ := idx.Get("jars"); fmtAny(jars) != "4" {
		t.Errorf("jars = %v", jars)
	}
	if len(res.Warnings) != 2 || res.Warnings[1] != "WARNING: "+paletteIndexScript+": webs-rt.jar!rc/webs.palette: "+
		"type t=\"q:Folder\" names an undeclared module prefix" {
		t.Errorf("warnings = %q", res.Warnings)
	}
	wantLog := "palette-index.json: 2 palettes (2 entries) from 4 jars (2 errors) -> " + index.Path(root, paletteIndexName)
	if len(res.Log) != 1 || res.Log[0] != wantLog {
		t.Errorf("log = %q\nwant %q", res.Log, wantLog)
	}
}

// TestPaletteIndexNeedsARegisteredInstall: rebuild-all's step says why it
// built nothing on a data root no registry names, and writes nothing.
func TestPaletteIndexNeedsARegisteredInstall(t *testing.T) {
	root := t.TempDir()
	res, err := buildPaletteIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "palette-index.json: no install registered; skipped (the last copy is kept)"; len(res.Log) != 1 || res.Log[0] != want {
		t.Errorf("log = %q", res.Log)
	}
	if _, err := os.Stat(index.Path(root, paletteIndexName)); !os.IsNotExist(err) {
		t.Errorf("palette-index.json written: %v", err)
	}
}
