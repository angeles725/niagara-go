package helpbuild

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/indexes"
	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// Java source a vendor ships inside a module jar maps to
// source/<jar stem>/<entry>, the docSource layout; third-party packages
// (the third-party-java exclude rule: org/, net/, io/, javax/ but
// javax/baja/ ...) and META-INF/ stay out, and doc/ keeps its guide path.
func TestModuleJarJavaSourceDest(t *testing.T) {
	rules := mustDefault(t)
	for _, tt := range []struct {
		name, jar, entry, want string
	}{
		{"vendor class", "clHVACHeating-rt.jar", "cl/hvac/heating/BCmBOA_StagedBoiler.java",
			"source/clHVACHeating-rt/cl/hvac/heating/BCmBOA_StagedBoiler.java"},
		{"honeywell class", "honPlantControllerHMI-rt.jar", "com/honeywell/hmi/BAlarmPriorityConfig.java",
			"source/honPlantControllerHMI-rt/com/honeywell/hmi/BAlarmPriorityConfig.java"},
		{"devkit template", "devkit-wb.jar", "templates/ndriver/BTemplateNetwork.java",
			"source/devkit-wb/templates/ndriver/BTemplateNetwork.java"},
		{"baja api package", "x-rt.jar", "javax/baja/x/BX.java", "source/x-rt/javax/baja/x/BX.java"},
		{"mockito", "test-wb.jar", "org/mockito/Mockito.java", ""},
		{"other third-party", "x-rt.jar", "net/sf/cglib/Enhancer.java", ""},
		{"jdk package", "x-rt.jar", "javax/swing/X.java", ""},
		{"meta-inf", "x-rt.jar", "META-INF/X.java", ""},
		{"compiled class", "x-rt.jar", "com/x/BX.class", ""},
		{"doc folder", "x-rt.jar", "doc/Example.java", "guides/x-rt/Example.java"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := destPath(tt.jar, tt.entry, rules)
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("destPath(%s, %s) = %q %v, want %q", tt.jar, tt.entry, got, ok, tt.want)
			}
		})
	}
}

// oemSourceInstall: docSource ships foo-rt's BFoo; the foo-rt module jar
// ships its own BFoo (different bytes), BBar and a bundled mockito class;
// bar-ux ships one class and nothing else.
func oemSourceInstall(t *testing.T) (install, root string) {
	t.Helper()
	dir := t.TempDir()
	install = filepath.Join(dir, "install")
	mods := filepath.Join(install, "modules")
	testkit.WriteJar(t, filepath.Join(mods, "docSource-doc.jar"),
		[2]string{"foo-rt/com/foo/BFoo.java", "package com.foo;\npublic class BFoo {}\n"})
	testkit.WriteJar(t, filepath.Join(mods, "foo-rt.jar"), [2]string{"META-INF/module.xml", "<module/>"},
		[2]string{"com/foo/BFoo.class", "x"}, [2]string{"com/foo/BFoo.java", "package com.foo;\n// vendor copy\npublic class BFoo {}\n"},
		[2]string{"com/foo/BBar.java", "package com.foo;\npublic class BBar {}\n"},
		[2]string{"org/mockito/Mockito.java", "package org.mockito;\npublic class Mockito {}\n"})
	testkit.WriteJar(t, filepath.Join(mods, "bar-ux.jar"), [2]string{"com/bar/BBar.java", "package com.bar;\npublic class BBar {}\n"})
	return install, filepath.Join(dir, "root")
}

func readOrigins(t *testing.T, root string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "source", indexes.SourceOriginsFile))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

var wantOrigins = map[string]string{"bar-ux/com/bar/BBar.java": "bar-ux.jar", "foo-rt/com/foo/BBar.java": "foo-rt.jar"}

// extract writes the module-jar sources next to docSource's, keeps the
// docSource file where both map one path, records which jar each module
// source came from, and fidelity checks exactly what was taken.
func TestExtractModuleJarSource(t *testing.T) {
	install, root := oemSourceInstall(t)
	st, err := extract(install, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Collision{{"source/foo-rt/com/foo/BFoo.java", "docSource-doc.jar", "foo-rt.jar"}}; st.Files != 3 ||
		!reflect.DeepEqual(st.SourceKept, want) || len(st.Collisions) != 0 {
		t.Errorf("extract %+v, want 3 files and kept %v", st, want)
	}
	if got := copiedFiles(t, filepath.Join(root, "source")); !reflect.DeepEqual(got, []string{"bar-ux/com/bar/BBar.java",
		"foo-rt/com/foo/BBar.java", "foo-rt/com/foo/BFoo.java", indexes.SourceOriginsFile}) {
		t.Errorf("source/ holds %q", got)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "source", "foo-rt", "com", "foo", "BFoo.java")); string(data) !=
		"package com.foo;\npublic class BFoo {}\n" {
		t.Errorf("docSource's BFoo was replaced: %q", data)
	}
	if got := readOrigins(t, root); !reflect.DeepEqual(got, wantOrigins) {
		t.Errorf("origins %v, want %v", got, wantOrigins)
	}
	rep, err := fidelity(install, root, nil)
	if err != nil || rep.Checked != 3 || rep.Matched != 3 || len(rep.Mismatched)+len(rep.Missing) != 0 || rep.Jars != 3 {
		t.Errorf("fidelity %+v %v", rep, err)
	}
}

// fill-missing (the only extraction allowed on a repo-root data root)
// takes the same module sources and writes the same origins.
func TestFillMissingModuleJarSource(t *testing.T) {
	install, root := oemSourceInstall(t)
	written, err := fillMissing(install, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"source/foo-rt/com/foo/BFoo.java", "source/bar-ux/com/bar/BBar.java",
		"source/foo-rt/com/foo/BBar.java"}; !reflect.DeepEqual(written, want) {
		t.Errorf("written %q, want %q", written, want)
	}
	if got := readOrigins(t, root); !reflect.DeepEqual(got, wantOrigins) {
		t.Errorf("origins %v, want %v", got, wantOrigins)
	}
	// Again: nothing to write, the origins stay.
	if written, err = fillMissing(install, root, nil); err != nil || len(written) != 0 {
		t.Errorf("second fill-missing %q %v", written, err)
	}
	if got := readOrigins(t, root); !reflect.DeepEqual(got, wantOrigins) {
		t.Errorf("origins after a second fill %v", got)
	}
}
