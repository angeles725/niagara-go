package help

import (
	"path/filepath"
	"strings"
	"testing"
)

// oemRepo: docSource's BFoo (with javadoc) and the module jar aaa-rt.jar's
// own BFoo and BOnly (no javadoc: the vendor ships source, not .bajadoc).
func oemRepo(t *testing.T) string {
	t.Helper()
	dir := repo(t, `{"default": "1.0", "versions": {"1.0": {"label": "One", "install_dir": ".", "data_root": "."}}}`, ".")
	write(t, filepath.Join(dir, "indexes", "class-index.json"),
		`{"BFoo": {"path": "com/foo/BFoo.html", "package": "com.foo", "type": "class", "module": "foo-rt"}}`)
	write(t, filepath.Join(dir, "indexes", "source-index.json"), `{
 "BFoo": {"module": "foo-rt", "path": "foo-rt/com/foo/BFoo.java", "package": "com.foo", "lines": 2},
 "BFoo (aaa-rt)": {"module": "aaa-rt", "path": "aaa-rt/com/a/BFoo.java", "package": "com.a", "lines": 3, "origin": "module-jar aaa-rt.jar"},
 "BOnly": {"module": "aaa-rt", "path": "aaa-rt/com/a/BOnly.java", "package": "com.a", "lines": 2, "origin": "module-jar aaa-rt.jar"}}`)
	write(t, filepath.Join(dir, "source", "aaa-rt", "com", "a", "BOnly.java"), "package com.a;\npublic class BOnly {}\n")
	return dir
}

func TestSourceShowsModuleJarOrigin(t *testing.T) {
	dir := oemRepo(t)
	for _, tt := range []struct {
		name string
		args []string
		want []string
		not  []string
	}{
		{"module-jar class", []string{"source", "BOnly"},
			[]string{"SOURCE: BOnly\n", "ORIGIN: module-jar aaa-rt.jar\n"}, nil},
		{"docSource wins, alternate listed with its origin", []string{"source", "BFoo"},
			[]string{"SOURCE: BFoo\n", "MODULE: foo-rt\n", "ORIGIN: docSource\n", "ALSO: BFoo (aaa-rt) [module-jar aaa-rt.jar]\n"}, nil},
		{"the alternate itself", []string{"source", "BFoo (aaa-rt)"},
			[]string{"ORIGIN: module-jar aaa-rt.jar\n", "ALSO: BFoo [docSource]\n"}, nil},
		{"listing names the jar", []string{"source", "B*"},
			[]string{"  BOnly (aaa-rt, 2 lines, module-jar aaa-rt.jar) - com.a\n", "  BFoo (foo-rt, 2 lines) - com.foo\n"}, nil},
		{"code of a module-jar class", []string{"source", "BOnly", "--code"},
			[]string{"   2 | public class BOnly {}\n"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, errs, code := run(t, dir, tt.args...)
			if code != 0 || errs != "" {
				t.Fatalf("code %d err %q out %q", code, errs, out)
			}
			for _, w := range tt.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %q:\n%s", w, out)
				}
			}
		})
	}
}

// A class with Java source but no javadoc is not a class miss: nh class
// shows its source record and origin, and a class with javadoc names its
// module-jar alternates.
func TestClassFallsBackToModuleJarSource(t *testing.T) {
	dir := oemRepo(t)
	out, errs, code := run(t, dir, "class", "BOnly")
	want := "CLASS: BOnly\nPACKAGE: com.a\nMODULE: aaa-rt\n" +
		"No javadoc (.bajadoc) for this class; its Java source is indexed:\n" +
		"SOURCE: source/aaa-rt/com/a/BOnly.java (2 lines)\nORIGIN: module-jar aaa-rt.jar\n" +
		"Run: nh source BOnly --code\n"
	if code != 0 || errs != "" || out != want {
		t.Errorf("class BOnly: code %d err %q\n%s\nwant\n%s", code, errs, out, want)
	}
	out, _, code = run(t, dir, "class", "com.a.BOnly")
	if code != 0 || !strings.HasPrefix(out, "CLASS: BOnly\nPACKAGE: com.a\n") {
		t.Errorf("class com.a.BOnly: code %d\n%s", code, out)
	}
	out, _, code = run(t, dir, "class", "BFoo")
	if code != 0 || !strings.Contains(out, "ALSO: Java source BFoo (aaa-rt) [module-jar aaa-rt.jar]\n") {
		t.Errorf("class BFoo: code %d, no module-jar alternate:\n%s", code, out)
	}
	out, _, code = run(t, dir, "class", "com.zzz.BOnly")
	if code != 1 || !strings.Contains(out, "No match for 'com.zzz.BOnly' (class)") {
		t.Errorf("class com.zzz.BOnly: code %d\n%s", code, out)
	}
}
