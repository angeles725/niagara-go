package helpbuild

import (
	"path/filepath"
	"slices"
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
