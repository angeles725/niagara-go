package helpbuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
	"github.com/angeles725/niagara-go/internal/index"
)

// bytecodeFixtureInstall is an install whose kitControl-rt.jar registers a
// latch with slots, its abstract super, two types of one class, a class the
// jar lacks and one that is not a class file, beside an unregistered class
// that is not one either (never read); a jar with a malformed module.xml, one
// without a descriptor and a corrupt one complete modules/.
func bytecodeFixtureInstall(t *testing.T) string {
	t.Helper()
	install := t.TempDir()
	mods := filepath.Join(install, "modules")
	latch := testkit.ClassBytes(testkit.Class{Name: "com/acme/BBooleanLatch", Super: "com/acme/BLatch",
		Methods: [][2]string{{"<init>", "()V"}, {"getOut", "()Ljavax/baja/status/BStatusBoolean;"}},
		Fields:  []string{"in", "out"},
		Props:   []testkit.Prop{{Name: "out", Type: "BStatusBoolean", Default: "new BStatusBoolean()", Flags: 5}}})
	base := testkit.ClassBytes(testkit.Class{Name: "com/acme/BLatch", Super: "javax/baja/sys/BComponent",
		Interfaces: []string{"javax/baja/status/BIStatus"}})
	testkit.WriteJar(t, filepath.Join(mods, "kitControl-rt.jar"),
		[2]string{"META-INF/module.xml", `<module name="kitControl-rt" moduleName="kitControl"><types>
<type name="BooleanLatch" class="com.acme.BBooleanLatch"/><type name="Latch" class="com.acme.BLatch"/>
<type name="LatchAlias" class="com.acme.BBooleanLatch"/><type name="Gone" class="com.acme.BGone"/>
<type name="Bad" class="com.acme.BBad"/></types></module>`},
		[2]string{"com/acme/BBooleanLatch.class", string(latch)}, [2]string{"com/acme/BLatch.class", string(base)},
		[2]string{"com/acme/BBad.class", "not a class"}, [2]string{"com/acme/BHidden.class", "never read"})
	testkit.WriteJar(t, filepath.Join(mods, "broken-rt.jar"),
		[2]string{"META-INF/module.xml", `<module name="broken-rt"><types></module>`})
	testkit.WriteJar(t, filepath.Join(mods, "plain-rt.jar"), [2]string{"a/B.class", "never read"})
	if err := os.WriteFile(filepath.Join(mods, "corrupt-rt.jar"), []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	return install
}

// TestBytecodeIndexReadsRegisteredClasses: every class a module.xml registers
// is a classes row (its types, access, super, interfaces, public fields and
// methods, slots by annotation and by field with flag letters); a registered
// class the jar lacks or cannot parse, and an unreadable jar or descriptor,
// is an errors row and a warning; an unregistered class is never read.
func TestBytecodeIndexReadsRegisteredClasses(t *testing.T) {
	install, root := bytecodeFixtureInstall(t), t.TempDir()
	res, err := buildBytecodeIndexFrom(root, install)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := index.LoadObject(index.Path(root, bytecodeIndexName))
	if err != nil {
		t.Fatal(err)
	}
	classes := strings.Join(rows(t, idx, "classes"), "\n")
	want := `{"class": "com.acme.BBooleanLatch", "jar": "kitControl-rt.jar", "types": ["kitControl:BooleanLatch", "kitControl:LatchAlias"],` +
		` "access": "public", "super": "com.acme.BLatch", "interfaces": [], "fields": ["in", "out"],` +
		` "methods": ["<init>()V", "getOut()Ljavax/baja/status/BStatusBoolean;"], "slots": [` +
		`{"name": "out", "kind": "property", "type": "BStatusBoolean", "return_type": "", "default": "new BStatusBoolean()", "flags": "5", "flag_letters": "rh", "via": "annotation"}, ` +
		`{"name": "in", "kind": "property", "type": "", "return_type": "", "default": "", "flags": "", "flag_letters": "", "via": "field"}]}
{"class": "com.acme.BLatch", "jar": "kitControl-rt.jar", "types": ["kitControl:Latch"], "access": "public",` +
		` "super": "javax.baja.sys.BComponent", "interfaces": ["javax.baja.status.BIStatus"], "fields": [], "methods": [], "slots": []}`
	if classes != want {
		t.Errorf("classes:\n%s", classes)
	}
	errs := rows(t, idx, "errors")
	for i, want := range []string{
		`{"jar": "broken-rt.jar", "path": "META-INF/module.xml", "error": "XML syntax error`,
		`{"jar": "corrupt-rt.jar", "path": "", "error": `,
		`{"jar": "kitControl-rt.jar", "path": "com/acme/BGone.class", "error": "registered in module.xml (kitControl:Gone) but not in the jar"}`,
		`{"jar": "kitControl-rt.jar", "path": "com/acme/BBad.class", "error": "classfile: bad magic`,
	} {
		if i >= len(errs) || !strings.HasPrefix(errs[i], want) {
			t.Errorf("errors[%d] lacks %s:\n%s", i, want, strings.Join(errs, "\n"))
		}
	}
	if len(errs) != 4 || len(res.Warnings) != 4 || !strings.HasPrefix(res.Warnings[2], "WARNING: bytecode_index_builder: kitControl-rt.jar!com/acme/BGone.class: ") {
		t.Errorf("errors %d warnings %q", len(errs), res.Warnings)
	}
	wantLog := "bytecode-index.json: 2 of 4 registered classes (3 of 5 registered types; 2 registered classes unreadable)," +
		" 2 slots (1 by annotation, 1 by field) from 4 jars (4 errors) -> "
	if res.Files != 2 || len(res.Log) != 1 || !strings.HasPrefix(res.Log[0], wantLog) {
		t.Errorf("files %d log %q", res.Files, res.Log)
	}
}

// TestBytecodeIndexNeedsARegisteredInstall: the rebuild-all step writes
// nothing on a data root no registry names, and says why.
func TestBytecodeIndexNeedsARegisteredInstall(t *testing.T) {
	root := t.TempDir()
	res, err := buildBytecodeIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "bytecode-index.json: no install registered; skipped (the last copy is kept)"; len(res.Log) != 1 || res.Log[0] != want {
		t.Errorf("log = %q", res.Log)
	}
	if _, err := os.Stat(index.Path(root, bytecodeIndexName)); !os.IsNotExist(err) {
		t.Errorf("bytecode-index.json written: %v", err)
	}
}
