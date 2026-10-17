package course

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// fixtureJars is the content of the synthetic install testdata/install/modules/*.jar: the
// entries the check spec reads, with stand-in bytes. The jars are generated from this table
// (COURSE_UPDATE_FIXTURES=1 go test -run TestFixtureJars) and TestFixtureJars fails when a
// committed jar drifts from it.
var fixtureJars = map[string]map[string]string{
	"training-rt": {
		"META-INF/MANIFEST.MF":  "Manifest-Version: 1.0\n",
		"META-INF/CODESIGN.SF":  "Signature-Version: 1.0\n",
		"META-INF/CODESIGN.RSA": "stand-in signature block\n",
		"META-INF/module.xml":   fixtureModuleXML,
		"module.palette":        `<p n="FirstComponent"/><p n="kitControl"><p n="Add"/></p>` + "\n",
		"training-rt.lexicon":   "url=Url\n",
		"META-INF/maven/com.fasterxml.jackson.core/jackson-databind/pom.properties": "version=2.21.2\n",
		"com/tridiumuniversity/training/BFirstComponent.class":                      "stand-in class\n",
		"com/tridiumuniversity/training/kitControl/BAdd.class":                      "stand-in class\n",
	},
	"training-wb": {
		"META-INF/CODESIGN.SF":  "Signature-Version: 1.0\n",
		"META-INF/CODESIGN.RSA": "stand-in signature block\n",
		"module.palette":        "<p/>\n",
	},
	"training-rtTest": {
		"test/com/tridiumuniversity/training/workOrder/BWorkOrderServiceTest.class": "stand-in class\n",
		"org/mockito/Mockito.class": "stand-in class\n",
	},
	"trafficLightDriver-rt": {
		"META-INF/module.xml": "<module><permissions><name>NETWORK_COMMUNICATION</name></permissions></module>\n",
		"com/tridiumuniversity/trafficLightDriver/learn/BTrafficLightDriverDeviceDiscoveryLeaf.class": "stand-in class\n",
		"com/tridiumuniversity/trafficLightDriver/point/BTrafficLightDriverPointDiscoveryLeaf.class":  "stand-in class\n",
	},
	"trafficLightDriver-wb": {
		"com/tridiumuniversity/trafficLightDriver/ui/BTrafficLightDriverPointManager.class": "stand-in class\n",
	},
	"trafficLightServer-rt": {
		"META-INF/CODESIGN.SF":  "Signature-Version: 1.0\n",
		"META-INF/CODESIGN.EC":  "stand-in signature block\n",
		"META-INF/MANIFEST.MF":  "Manifest-Version: 1.0\n",
		"module.palette":        "<p/>\n",
		"trafficLightServer.px": "<px/>\n",
	},
}

const fixtureModuleXML = `<module name="training-rt">
 <dependencies>
  <dependency name="net-rt"/>
  <dependency name="history-rt"/>
  <dependency name="file-rt"/>
  <dependency name="bql-rt"/>
 </dependencies>
 <types>
  <type name="FirstComponent"/>
  <type name="Add"/>
 </types>
 <permissions>
  <name>NETWORK_COMMUNICATION</name>
  <name>REFLECTION</name>
  <name>MANAGE_EXECUTION</name>
 </permissions>
</module>
`

const fixtureInstall = "testdata/install"

func buildJar(entries map[string]string) []byte {
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: n, Method: zip.Deflate,
			Modified: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			panic(err)
		}
		_, _ = io.WriteString(w, entries[n])
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func readJar(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("%s: %v (regenerate: COURSE_UPDATE_FIXTURES=1 go test -run TestFixtureJars)", path, err)
	}
	defer zr.Close()
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = string(data)
	}
	return out
}

func TestFixtureJars(t *testing.T) {
	dir := filepath.Join(fixtureInstall, "modules")
	if os.Getenv("COURSE_UPDATE_FIXTURES") == "1" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, entries := range fixtureJars {
			if err := os.WriteFile(filepath.Join(dir, name+".jar"), buildJar(entries), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jar"))
	if len(files) != len(fixtureJars) {
		t.Errorf("%s holds %d jars, want %d", dir, len(files), len(fixtureJars))
	}
	for name, entries := range fixtureJars {
		if got := readJar(t, filepath.Join(dir, name+".jar")); !reflect.DeepEqual(got, entries) {
			t.Errorf("%s.jar drifted from fixtureJars (regenerate: COURSE_UPDATE_FIXTURES=1 go test -run TestFixtureJars)", name)
		}
	}
}
