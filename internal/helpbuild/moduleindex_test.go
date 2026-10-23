package helpbuild

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
)

const fixtureKitControlRT = `<?xml version="1.0" encoding="UTF-8"?>
<module name="kitControl-rt" vendor="Tridium" vendorVersion="4.15.6.20" description="Library of Control Components" moduleName="kitControl" runtimeProfile="rt">
 <dependencies>
  <dependency name="baja" vendor="Tridium" vendorVersion="4.15.6"/>
  <dependency name="control-rt" vendor="Tridium" vendorVersion="4.15.6"/>
 </dependencies>
 <types>
  <type class="com.tridium.kitControl.util.BBooleanLatch" name="BooleanLatch"/>
 </types>
 <permissions>
  <java-permissions type="station">
   <java-permission class="java.lang.RuntimePermission" name="setIO"/>
  </java-permissions>
 </permissions>
</module>
`

const fixtureKitControlUX = `<module name="kitControl-ux" vendor="Tridium" vendorVersion="4.15.6.20" moduleName="kitControl" runtimeProfile="ux">
 <dependencies><dependency name="kitControl-rt" vendor="Tridium" vendorVersion="4.15.6"/></dependencies>
 <types>
  <type class="com.tridium.kitControl.ux.BLatchTypeExt" name="LatchTypeExt">
   <agent><on type="kitControl:BooleanLatch"/></agent>
  </type>
 </types>
</module>
`

// moduleFixtureInstall is an install whose modules/ holds two good module jars,
// one with a malformed descriptor, one without any, one whose descriptor
// repeats a module name, a corrupt jar and a file that is not a jar.
func moduleFixtureInstall(t *testing.T) string {
	t.Helper()
	install := t.TempDir()
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "kitControl-rt.jar"), [2]string{"META-INF/module.xml", fixtureKitControlRT},
		[2]string{"com/tridium/kitControl/util/BBooleanLatch.class", "x"})
	testkit.WriteJar(t, filepath.Join(mods, "kitControl-ux.jar"), [2]string{"META-INF/module.xml", fixtureKitControlUX})
	testkit.WriteJar(t, filepath.Join(mods, "broken-rt.jar"), [2]string{"META-INF/module.xml", `<module name="broken-rt"><types>`})
	testkit.WriteJar(t, filepath.Join(mods, "plain.jar"), [2]string{"a.txt", "no descriptor"})
	testkit.WriteJar(t, filepath.Join(mods, "zz-copy.jar"), [2]string{"META-INF/module.xml", fixtureKitControlUX})
	for name, data := range map[string]string{"corrupt-rt.jar": "not a zip", "README.txt": "not a jar"} {
		if err := os.WriteFile(filepath.Join(mods, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return install
}

// TestModuleIndexReadsEveryJar: every jar's descriptor is indexed under its
// module name; a malformed descriptor, a corrupt jar and a repeated module name
// are recorded with the jar and the reason and printed as warnings, never
// dropped; a jar without a descriptor is listed.
func TestModuleIndexReadsEveryJar(t *testing.T) {
	install, root := moduleFixtureInstall(t), t.TempDir()
	res, err := buildModuleIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := index.LoadObject(index.Path(root, moduleIndexName))
	if err != nil {
		t.Fatal(err)
	}
	mods, _ := idx.Get("modules")
	if got := strings.Join(mods.(*index.Object).Keys(), " "); got != "kitControl-rt kitControl-ux" {
		t.Errorf("modules = %s", got)
	}
	rt := mods.(*index.Object).Object("kitControl-rt")
	if jar, _ := rt.Get("jar"); jar != "kitControl-rt.jar" {
		t.Errorf("jar of kitControl-rt = %v", jar)
	}
	errs, _ := idx.Get("errors")
	var got []string
	for _, e := range errs.([]any) {
		o := e.(*index.Object)
		jar, _ := o.Get("jar")
		why, _ := o.Get("error")
		got = append(got, jar.(string)+": "+why.(string))
	}
	want := []string{
		"broken-rt.jar: XML syntax error on line 1: unexpected EOF",
		"corrupt-rt.jar: bad zip file",
		"zz-copy.jar: module kitControl-ux is also declared by kitControl-ux.jar",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("errors:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if without, _ := idx.Get("without_descriptor"); len(without.([]any)) != 1 || without.([]any)[0] != "plain.jar" {
		t.Errorf("without_descriptor = %v", without)
	}
	if jars, _ := idx.Get("jars"); fmtAny(jars) != "6" {
		t.Errorf("jars = %v", jars)
	}
	if len(res.Warnings) != 3 || !strings.HasPrefix(res.Warnings[0], "WARNING: "+moduleIndexScript+": broken-rt.jar: ") {
		t.Errorf("warnings = %q", res.Warnings)
	}
	wantLog := "module-index.json: 2 modules from 6 jars (1 without module.xml, 3 errors) -> " + index.Path(root, moduleIndexName)
	if len(res.Log) != 1 || res.Log[0] != wantLog {
		t.Errorf("log = %q\nwant %q", res.Log, wantLog)
	}
}

func fmtAny(v any) string {
	n, ok := index.Int(v)
	if !ok {
		return "?"
	}
	return strconv.FormatInt(n, 10)
}
