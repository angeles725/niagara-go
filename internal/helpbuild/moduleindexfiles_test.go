package helpbuild

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// TestModuleIndexRecordsTemplatesAndSchemas: a module's permission templates
// and schemas join its record; an unreadable schema is an errors row with its
// path while the module stays indexed; a schema in a jar without module.xml
// is an errors row too, and the sweep still sees both descriptors as indexed.
func TestModuleIndexRecordsTemplatesAndSchemas(t *testing.T) {
	install, root := t.TempDir(), t.TempDir()
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "kitControl-rt.jar"), [2]string{"META-INF/module.xml", fixtureKitControlRT},
		[2]string{"rc/module-permissions.xml", `<permissions><niagara-permission-groups type="all"/></permissions>`},
		[2]string{"rsc/a.xsd", `<xs:schema xmlns:xs="x" targetNamespace="urn:a"><xs:element name="a"/></xs:schema>`},
		[2]string{"rsc/bad.xsd", `<xs:schema>`})
	testkit.WriteJar(t, filepath.Join(mods, "plain.jar"), [2]string{"svg.dtd", `<!ELEMENT svg ANY>`})
	res, err := buildModuleIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := index.LoadObject(index.Path(root, moduleIndexName))
	if err != nil {
		t.Fatal(err)
	}
	rt := idx.Object("modules").Object("kitControl-rt")
	for key, want := range map[string]string{
		"permission_templates": `[{"path": "rc/module-permissions.xml", "java_permissions": [], "groups": [{"type": "all", "requests": []}], "velocity": []}]`,
		"schemas":              `[{"path": "rsc/a.xsd", "kind": "xsd", "root": "schema", "target_namespace": "urn:a", "elements": 1}]`,
	} {
		v, _ := rt.Get(key)
		if got := pyjson.DumpsAny(v, pyjson.DumpOpts{Indent: -1, ItemSep: ", ", KeySep: ": "}); got != want {
			t.Errorf("%s = %s", key, got)
		}
	}
	errs, _ := idx.Get("errors")
	var got []string
	for _, e := range errs.([]any) {
		o := e.(*index.Object)
		jar, _ := o.String("jar")
		path, _ := o.String("path")
		why, _ := o.String("error")
		got = append(got, jar+"!"+path+": "+why)
	}
	want := "kitControl-rt.jar!rsc/bad.xsd: XML syntax error on line 1: unexpected EOF|" +
		"plain.jar!svg.dtd: shipped by a jar without META-INF/module.xml (no module record holds it)"
	if strings.Join(got, "|") != want || len(res.Warnings) != 2 {
		t.Errorf("errors = %s\nwarnings = %q", strings.Join(got, "|"), res.Warnings)
	}
	x, err := loadShipped(root)
	if why, ok := x.held[[2]string{"kitControl-rt.jar", ""}]; err != nil || !ok || why != "" {
		t.Errorf("sweep reads kitControl-rt.jar's descriptor as %q %v (%v)", why, ok, err)
	}
}
