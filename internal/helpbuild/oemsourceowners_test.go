package helpbuild

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/indexes"
)

// sourceOwners: a doc jar's path is taken (a later doc jar still
// overwrites, as before), a module-jar source on a path a doc jar maps is
// left to it, and only the module-jar sources taken are recorded.
func TestSourceOwnersDocSourceWins(t *testing.T) {
	o := newSourceOwners()
	steps := []struct {
		jar, rel, prev string
		take           bool
	}{
		{"docSource-doc.jar", "source/foo-rt/com/foo/BFoo.java", "", true},
		{"foo-rt.jar", "source/foo-rt/com/foo/BFoo.java", "docSource-doc.jar", false},
		{"foo-rt.jar", "source/foo-rt/com/foo/BBar.java", "", true},
		{"docA-doc.jar", "guides/A/a.html", "", true},
		{"docB-doc.jar", "guides/A/a.html", "docA-doc.jar", true},
		{"x-rt.jar", "guides/x-rt/doc.html", "", true},
	}
	for _, s := range steps {
		if prev, take := o.take(s.jar, s.rel); prev != s.prev || take != s.take {
			t.Errorf("take(%s, %s) = %q %v, want %q %v", s.jar, s.rel, prev, take, s.prev, s.take)
		}
	}
	if len(o.origin) != 1 || o.origin["foo-rt/com/foo/BBar.java"] != "foo-rt.jar" {
		t.Errorf("origins %v, want only foo-rt/com/foo/BBar.java", o.origin)
	}
}

// write saves the origins sorted under source/, and nothing when none.
func TestSourceOwnersWrite(t *testing.T) {
	root := t.TempDir()
	if err := newSourceOwners().write(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "source")); !os.IsNotExist(err) {
		t.Errorf("a pass with no module source wrote source/ (stat: %v)", err)
	}
	o := newSourceOwners()
	o.take("b-rt.jar", "source/b-rt/B.java")
	o.take("a-rt.jar", "source/a-rt/A.java")
	if err := o.write(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "source", indexes.SourceOriginsFile))
	want := "{\n  \"a-rt/A.java\": \"a-rt.jar\",\n  \"b-rt/B.java\": \"b-rt.jar\"\n}"
	if err != nil || string(data) != want && string(data) != want+"\n" {
		t.Errorf("origins file %q (%v), want %q", data, err, want)
	}
}
