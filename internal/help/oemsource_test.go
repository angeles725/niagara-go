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
