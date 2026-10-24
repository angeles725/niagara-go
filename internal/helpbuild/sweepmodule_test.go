package helpbuild

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// TestSweepCountsModuleDescriptors: META-INF/module.xml is no longer outside
// the documentation scope. An installed jar's descriptor is indexed when
// module-index.json holds its module, UNCLASSIFIED when it does not (or could
// not read it); a copy outside modules/ is excluded with its reason.
func TestSweepCountsModuleDescriptors(t *testing.T) {
	install, root := t.TempDir(), t.TempDir()
	desc := func(name string) [2]string { return [2]string{"META-INF/module.xml", `<module name="` + name + `"/>`} }
	testkit.WriteJar(t, filepath.Join(install, "modules", "a-rt.jar"), desc("a-rt"))
	testkit.WriteJar(t, filepath.Join(install, "modules", "b-rt.jar"), desc("b-rt"))
	testkit.WriteJar(t, filepath.Join(install, "modules", "c-rt.jar"), desc("c-rt"))
	testkit.WriteJar(t, filepath.Join(install, "sw", "1.0", "a-rt.jar"), desc("a-rt"))
	testkit.WriteFile(t, filepath.Join(root, "indexes", moduleIndexName), []byte(`{"jars": 3, "jar_bytes": 0,
	 "modules": {"a-rt": {"name": "a-rt", "jar": "a-rt.jar"}},
	 "errors": [{"jar": "c-rt.jar", "error": "XML syntax error on line 1: unexpected EOF"}], "without_descriptor": []}`))
	res := scanOK(t, install, root, SweepOptions{Rules: sweepTestRules(t)})
	if row := res.Rows["modules/"]; row.DocLike != 3 || row.Indexed != 1 || row.Unclassified != 2 {
		t.Errorf("modules/ row = %+v, want 3 doc-like, 1 indexed, 2 unclassified", *row)
	}
	want := []gap{
		{"modules/b-rt.jar", "META-INF/module.xml", "module descriptor not in module-index.json (rebuild-all builds it)"},
		{"modules/c-rt.jar", "META-INF/module.xml", "module descriptor module-index.json could not read: XML syntax error on line 1: unexpected EOF"},
	}
	if !slices.Equal(res.Unclassified, want) {
		t.Errorf("unclassified = %q", res.Unclassified)
	}
	if got := ruleCounts(res); !mapsEqual(got, map[string]int{"module-descriptor-outside-modules": 1}) {
		t.Errorf("reasons by rule = %v", got)
	}
	if n := res.OutOfScope[".xml"]; n != 0 {
		t.Errorf("%d .xml entries still out of scope", n)
	}
}

// TestSweepCountsShippedFiles: palettes, module-permissions templates and
// schemas are no longer outside the documentation scope. In an installed
// jar each is indexed when its index holds it, UNCLASSIFIED when it does not
// or could not read it; a copy outside modules/ is excluded with its reason.
func TestSweepCountsShippedFiles(t *testing.T) {
	install, root := t.TempDir(), t.TempDir()
	testkit.WriteJar(t, filepath.Join(install, "modules", "a-rt.jar"), [2]string{"module.palette", "<x/>"},
		[2]string{"rc/module-permissions.xml", "<x/>"}, [2]string{"rsc/a.xsd", "<x/>"}, [2]string{"rsc/b.dtd", ""})
	testkit.WriteJar(t, filepath.Join(install, "modules", "b-rt.jar"), [2]string{"module.palette", "<x/>"},
		[2]string{"gradle/module-permissions.xml.vm", "#parse"})
	testkit.WriteJar(t, filepath.Join(install, "sw", "1.0", "a-rt.jar"), [2]string{"module.palette", "<x/>"})
	testkit.WriteFile(t, filepath.Join(root, "indexes", moduleIndexName), []byte(`{"jars": 2, "jar_bytes": 0,
	 "modules": {"a-rt": {"name": "a-rt", "jar": "a-rt.jar",
	   "permission_templates": [{"path": "rc/module-permissions.xml"}], "schemas": [{"path": "rsc/a.xsd"}]}},
	 "errors": [{"jar": "a-rt.jar", "path": "rsc/b.dtd", "error": "bad"}], "without_descriptor": []}`))
	testkit.WriteFile(t, filepath.Join(root, "indexes", paletteIndexName), []byte(`{"jars": 2, "jar_bytes": 0,
	 "palettes": [{"module": "a-rt", "jar": "a-rt.jar", "path": "module.palette"}],
	 "errors": [{"jar": "b-rt.jar", "path": "module.palette", "error": "worse"}]}`))
	res := scanOK(t, install, root, SweepOptions{Rules: sweepTestRules(t)})
	if row := res.Rows["modules/"]; row.DocLike != 6 || row.Indexed != 3 || row.Unclassified != 3 {
		t.Errorf("modules/ row = %+v, want 6 doc-like, 3 indexed, 3 unclassified", *row)
	}
	want := []gap{
		{"modules/a-rt.jar", "rsc/b.dtd", "schema module-index.json could not read: bad"},
		{"modules/b-rt.jar", "gradle/module-permissions.xml.vm", "permissions template not in module-index.json (rebuild-all builds it)"},
		{"modules/b-rt.jar", "module.palette", "palette palette-index.json could not read: worse"},
	}
	got := slices.Clone(res.Unclassified)
	slices.SortFunc(got, func(a, b gap) int {
		return strings.Compare(a.Jar+a.Entry, b.Jar+b.Entry)
	})
	if !slices.Equal(got, want) {
		t.Errorf("unclassified = %q", got)
	}
	if got := ruleCounts(res); !mapsEqual(got, map[string]int{"shipped-file-outside-modules": 1}) {
		t.Errorf("reasons by rule = %v", got)
	}
	for _, ext := range []string{".palette", ".xml", ".xsd", ".dtd", ".vm"} {
		if n := res.OutOfScope[ext]; n != 0 {
			t.Errorf("%d %s entries still out of scope", n, ext)
		}
	}
}

// TestSweepCountsTagDictionaries: the tag dictionary files (the .json and
// .csv of a haystack jar, the .bog of a tagdictionary jar) are no longer
// outside the documentation scope. In an installed jar each is indexed when
// tag-index.json holds it, UNCLASSIFIED when it does not or could not read
// it; a copy outside modules/ is excluded; the same names in another jar stay
// out of scope.
func TestSweepCountsTagDictionaries(t *testing.T) {
	install, root := t.TempDir(), t.TempDir()
	testkit.WriteJar(t, filepath.Join(install, "modules", "haystack-rt.jar"), [2]string{"data/defs.json", "{}"},
		[2]string{"data/tags.csv", "a\n"}, [2]string{"data/broken.json", "{"})
	testkit.WriteJar(t, filepath.Join(install, "modules", "tagdictionary-rt.jar"), [2]string{"bog/Niagara.bog", "<x/>"})
	testkit.WriteJar(t, filepath.Join(install, "modules", "webChart-ux.jar"), [2]string{"data/tags.csv", "a\n"})
	testkit.WriteJar(t, filepath.Join(install, "sw", "1.0", "haystack-rt.jar"), [2]string{"data/tags.csv", "a\n"})
	testkit.WriteFile(t, filepath.Join(root, "indexes", tagIndexName), []byte(`{"jars": 3, "jar_bytes": 0,
	 "files": [{"jar": "haystack-rt.jar", "path": "data/defs.json"}, {"jar": "haystack-rt.jar", "path": "data/tags.csv"}],
	 "errors": [{"jar": "haystack-rt.jar", "path": "data/broken.json", "error": "bad"}]}`))
	res := scanOK(t, install, root, SweepOptions{Rules: sweepTestRules(t)})
	if row := res.Rows["modules/"]; row.DocLike != 4 || row.Indexed != 2 || row.Unclassified != 2 {
		t.Errorf("modules/ row = %+v, want 4 doc-like, 2 indexed, 2 unclassified", *row)
	}
	want := []gap{
		{"modules/haystack-rt.jar", "data/broken.json", "tag dictionary tag-index.json could not read: bad"},
		{"modules/tagdictionary-rt.jar", "bog/Niagara.bog", "tag dictionary not in tag-index.json (rebuild-all builds it)"},
	}
	got := slices.Clone(res.Unclassified)
	slices.SortFunc(got, func(a, b gap) int { return strings.Compare(a.Jar+a.Entry, b.Jar+b.Entry) })
	if !slices.Equal(got, want) {
		t.Errorf("unclassified = %q", got)
	}
	if got := ruleCounts(res); !mapsEqual(got, map[string]int{"shipped-file-outside-modules": 1}) {
		t.Errorf("reasons by rule = %v", got)
	}
	if n := res.OutOfScope[".csv"]; n != 1 {
		t.Errorf("%d .csv entries out of scope, want webChart-ux's only", n)
	}
}

// TestSweepCountsRegisteredClasses: the class file of each type an installed
// jar's module.xml registers (module-index.json) is no longer outside the
// documentation scope: indexed when bytecode-index.json holds it, UNCLASSIFIED
// when it does not or could not read it; other classes stay out of scope.
func TestSweepCountsRegisteredClasses(t *testing.T) {
	install, root := t.TempDir(), t.TempDir()
	testkit.WriteJar(t, filepath.Join(install, "modules", "a-rt.jar"), [2]string{"META-INF/module.xml", `<module name="a-rt"/>`},
		[2]string{"a/BOne.class", "x"}, [2]string{"a/BTwo.class", "x"}, [2]string{"a/BThree.class", "x"},
		[2]string{"a/Helper.class", "x"})
	testkit.WriteFile(t, filepath.Join(root, "indexes", moduleIndexName), []byte(`{"jars": 1, "jar_bytes": 0,
	 "modules": {"a-rt": {"name": "a-rt", "jar": "a-rt.jar", "types": [{"name": "One", "class": "a.BOne"},
	   {"name": "Two", "class": "a.BTwo"}, {"name": "Three", "class": "a.BThree"}]}},
	 "errors": [], "without_descriptor": []}`))
	testkit.WriteFile(t, filepath.Join(root, "indexes", bytecodeIndexName), []byte(`{"jars": 1, "jar_bytes": 0,
	 "classes": [{"class": "a.BOne", "jar": "a-rt.jar"}],
	 "errors": [{"jar": "a-rt.jar", "path": "a/BTwo.class", "error": "classfile: bad magic"}]}`))
	res := scanOK(t, install, root, SweepOptions{Rules: sweepTestRules(t)})
	if row := res.Rows["modules/"]; row.DocLike != 4 || row.Indexed != 2 || row.Unclassified != 2 {
		t.Errorf("modules/ row = %+v, want 4 doc-like, 2 indexed, 2 unclassified", *row)
	}
	want := []gap{
		{"modules/a-rt.jar", "a/BTwo.class", "registered class bytecode-index.json could not read: classfile: bad magic"},
		{"modules/a-rt.jar", "a/BThree.class", "registered class not in bytecode-index.json (rebuild-all builds it)"},
	}
	if !slices.Equal(res.Unclassified, want) {
		t.Errorf("unclassified = %q", res.Unclassified)
	}
	if n := res.OutOfScope[".class"]; n != 1 {
		t.Errorf("%d .class entries out of scope, want the unregistered one", n)
	}
}
