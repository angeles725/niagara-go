package indexes

import (
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// readSourceOrigins: no source/ or no origins file is no module-jar
// source; a file that is not an object of strings is an error; jar names
// the module jar of a file by its path under source/.
func TestReadSourceOrigins(t *testing.T) {
	base := t.TempDir()
	if o, err := readSourceOrigins(base); err != nil || len(o) != 0 {
		t.Errorf("no source/: %v %v", o, err)
	}
	testkit.WriteFile(t, filepath.Join(base, "source", "foo-rt", "A.java"), []byte("class A {}\n"))
	if o, err := readSourceOrigins(base); err != nil || len(o) != 0 {
		t.Errorf("no origins file: %v %v", o, err)
	}
	for _, bad := range []string{`["a"]`, `{"a": 1}`, `{`} {
		testkit.WriteFile(t, filepath.Join(base, "source", SourceOriginsFile), []byte(bad))
		if _, err := readSourceOrigins(base); err == nil {
			t.Errorf("origins %s accepted", bad)
		}
	}
	testkit.WriteFile(t, filepath.Join(base, "source", SourceOriginsFile), []byte(`{"aaa-rt/com/a/B.java": "aaa-rt.jar"}`))
	o, err := readSourceOrigins(base)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := absSourceDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if got := o.jar(dir, filepath.Join(dir, "aaa-rt", "com", "a", "B.java")); got != "aaa-rt.jar" {
		t.Errorf("jar of B.java = %q", got)
	}
	if got := o.jar(dir, filepath.Join(dir, "foo-rt", "A.java")); got != "" {
		t.Errorf("jar of a docSource file = %q", got)
	}
}
