package helpbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

const (
	fixtureDefs = `{"_kind": "grid", "cols": [{"name": "def"}], "rows": [
	 {"def": {"_kind": "symbol", "val": "marker"}, "doc": "Marker"},
	 {"def": {"_kind": "symbol", "val": "ahu"}, "doc": "Air Handling Unit", "is": [{"_kind": "symbol", "val": "marker"}]}]}`
	fixtureTagsCSV = "name,kind,SmartType\nahu,Marker,\n"
	fixtureNiagara = `<bajaObjectGraph><p m="b=baja" t="b:UnrestrictedFolder">
<p n="Niagara" m="td=tagdictionary" t="td:NiagaraTagDictionary"><p n="tagDefinitions" t="td:TagInfoList">
<p n="point" t="td:SimpleTagInfo"><p n="validity" t="td:Always"/></p></p></p></p></bajaObjectGraph>`
)

// tagFixtureInstall is an install whose modules/ holds haystack-rt (two
// dictionary files, a malformed one and one of an unknown shape),
// tagdictionary-rt, a jar shipping JSON that is not a dictionary jar, a
// corrupt haystack jar and a file that is not a jar.
func tagFixtureInstall(t *testing.T) string {
	t.Helper()
	install := t.TempDir()
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "haystack-rt.jar"),
		[2]string{"com/tridium/haystack/data/haystack-4-defs/defs/defs.json", fixtureDefs},
		[2]string{"com/tridium/haystack/data/tags.csv", fixtureTagsCSV},
		[2]string{"com/tridium/haystack/data/broken.json", "{"},
		[2]string{"com/tridium/haystack/data/other.csv", "a,b\n"},
		[2]string{"com/tridium/haystack/data/notes.txt", "not read"})
	testkit.WriteJar(t, filepath.Join(mods, "tagdictionary-rt.jar"), [2]string{"bog/Niagara.bog", fixtureNiagara})
	testkit.WriteJar(t, filepath.Join(mods, "webChart-ux.jar"), [2]string{"data/tags.csv", fixtureTagsCSV})
	for name, data := range map[string]string{"haystack-wb.jar": "not a zip", "README.txt": "not a jar"} {
		if err := os.WriteFile(filepath.Join(mods, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return install
}

func rows(t *testing.T, idx *index.Object, key string) []string {
	t.Helper()
	list, _ := idx.Get(key)
	var out []string
	for _, v := range list.([]any) {
		out = append(out, pyjson.DumpsAny(v, pyjson.Defaults(-1)))
	}
	return out
}

// TestTagIndexReadsTheDictionaries: every dictionary file of the dictionary
// jars is recorded with its kind and record count and its records are
// indexed; an unreadable file or jar is an errors row and a warning.
func TestTagIndexReadsTheDictionaries(t *testing.T) {
	install, root := tagFixtureInstall(t), t.TempDir()
	res, err := buildTagIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := index.LoadObject(index.Path(root, tagIndexName))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := idx.Get("jars"); func() int64 { n, _ := index.Int(v); return n }() != 4 {
		t.Errorf("jars = %v, want every jar of modules/", v)
	}
	files := strings.Join(rows(t, idx, "files"), "\n")
	wantFiles := `{"jar": "haystack-rt.jar", "path": "com/tridium/haystack/data/haystack-4-defs/defs/defs.json", "kind": "defs", "records": 2}
{"jar": "haystack-rt.jar", "path": "com/tridium/haystack/data/tags.csv", "kind": "tags", "records": 1}
{"jar": "tagdictionary-rt.jar", "path": "bog/Niagara.bog", "kind": "niagara", "records": 1}`
	if files != wantFiles {
		t.Errorf("files:\n%s", files)
	}
	tags := strings.Join(rows(t, idx, "tags"), "\n")
	for _, want := range []string{
		`{"dict": "hs", "name": "ahu", "file": "haystack-rt.jar!com/tridium/haystack/data/haystack-4-defs/defs/defs.json", "kind": "Marker"`,
		`{"dict": "hs", "name": "ahu", "file": "haystack-rt.jar!com/tridium/haystack/data/tags.csv", "kind": "Marker"}`,
		`{"dict": "n", "name": "point", "file": "tagdictionary-rt.jar!bog/Niagara.bog", "kind": "Marker", "validity": {"condition": "always"}}`,
	} {
		if !strings.Contains(tags, want) {
			t.Errorf("tags lack %s:\n%s", want, tags)
		}
	}
	errs := strings.Join(rows(t, idx, "errors"), "\n")
	for _, want := range []string{`"jar": "haystack-rt.jar", "path": "com/tridium/haystack/data/broken.json"`,
		`"path": "com/tridium/haystack/data/other.csv", "error": "no header row`, `"jar": "haystack-wb.jar", "path": ""`} {
		if !strings.Contains(errs, want) {
			t.Errorf("errors lack %s:\n%s", want, errs)
		}
	}
	if len(res.Warnings) != 3 || !strings.HasPrefix(res.Warnings[0], "WARNING: tag_index_builder: haystack-rt.jar!com/tridium/haystack/data/broken.json: ") {
		t.Errorf("warnings = %q", res.Warnings)
	}
	if res.Files != 3 || len(res.Log) != 1 || !strings.HasPrefix(res.Log[0],
		"tag-index.json: 4 tag rows (3 distinct tags), 0 protos, 0 groups, 0 relations, 0 rules, 0 migrations from 3 files in 4 jars (3 errors) -> ") {
		t.Errorf("files %d log %q", res.Files, res.Log)
	}
}

// TestTagIndexNeedsARegisteredInstall: rebuild-all's step says why it built
// nothing on a data root no registry names, and writes nothing.
func TestTagIndexNeedsARegisteredInstall(t *testing.T) {
	root := t.TempDir()
	res, err := buildTagIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "tag-index.json: no install registered; skipped (the last copy is kept)"; len(res.Log) != 1 || res.Log[0] != want {
		t.Errorf("log = %q", res.Log)
	}
	if _, err := os.Stat(index.Path(root, tagIndexName)); !os.IsNotExist(err) {
		t.Errorf("tag-index.json written: %v", err)
	}
}
