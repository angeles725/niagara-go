package indexes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/helpbuild/internal/testkit"
)

// originFixture: docSource's foo-rt holds BFoo; the module jar aaa-rt.jar
// (its folder sorts first) ships its own BFoo and BOnly, both with a slot.
func originFixture(t *testing.T, withOrigins bool) string {
	t.Helper()
	base := t.TempDir()
	slot := "@NiagaraProperty(name = \"x\", type = \"int\", defaultValue = \"0\")\n"
	testkit.WriteFile(t, filepath.Join(base, "source", "foo-rt", "com", "foo", "BFoo.java"),
		[]byte("package com.foo;\n"+slot+"public class BFoo {}\n"))
	testkit.WriteFile(t, filepath.Join(base, "source", "aaa-rt", "com", "a", "BFoo.java"),
		[]byte("package com.a;\n"+slot+"public class BFoo {}\n"))
	testkit.WriteFile(t, filepath.Join(base, "source", "aaa-rt", "com", "a", "BOnly.java"),
		[]byte("package com.a;\n"+slot+"public class BOnly {}\n"))
	if withOrigins {
		testkit.WriteFile(t, filepath.Join(base, "source", SourceOriginsFile),
			[]byte(`{"aaa-rt/com/a/BFoo.java": "aaa-rt.jar", "aaa-rt/com/a/BOnly.java": "aaa-rt.jar"}`))
	}
	return base
}

type indexRow struct {
	key    string
	record map[string]any
}

// readIndexRows reads an index object in key order.
func readIndexRows(t *testing.T, path string) []indexRow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	if _, err := dec.Token(); err != nil {
		t.Fatal(err)
	}
	var rows []indexRow
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, indexRow{k.(string), v})
	}
	return rows
}

func rowKeys(rows []indexRow) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.key)
	}
	return out
}

// The source index keeps docSource first: its BFoo has the plain key, the
// module jar's BFoo is the alternate "BFoo (aaa-rt)", and every module-jar
// record names its jar in "origin" (docSource records have none).
func TestSourceIndexOrigin(t *testing.T) {
	base := originFixture(t, true)
	if _, err := BuildSourceIndex(base); err != nil {
		t.Fatal(err)
	}
	rows := readIndexRows(t, filepath.Join(base, "indexes", "source-index.json"))
	got := rowKeys(rows)
	sort.Strings(got[1:]) // a folder's files come in the order the file system lists them
	if want := []string{"BFoo", "BFoo (aaa-rt)", "BOnly"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("keys %q, want %q", rowKeys(rows), want)
	}
	if r := rows[0].record; r["module"] != "foo-rt" || r["origin"] != nil {
		t.Errorf("BFoo %v: want docSource's foo-rt, no origin", r)
	}
	for _, r := range rows[1:] {
		if r.record["module"] != "aaa-rt" || r.record["origin"] != "module-jar aaa-rt.jar" {
			t.Errorf("%s %v: want module aaa-rt, origin module-jar aaa-rt.jar", r.key, r.record)
		}
	}
}

// Without the origins file (a docSource-only corpus) the index is what it
// was: folders in sorted order (aaa-rt's BFoo takes the plain key), no
// origin field.
func TestSourceIndexWithoutOrigins(t *testing.T) {
	base := originFixture(t, false)
	if _, err := BuildSourceIndex(base); err != nil {
		t.Fatal(err)
	}
	rows := readIndexRows(t, filepath.Join(base, "indexes", "source-index.json"))
	modules := map[string]any{}
	for _, r := range rows {
		modules[r.key] = r.record["module"]
		if _, has := r.record["origin"]; has {
			t.Errorf("%s has an origin", r.key)
		}
	}
	if want := map[string]any{"BFoo": "aaa-rt", "BOnly": "aaa-rt", "BFoo (foo-rt)": "foo-rt"}; !reflect.DeepEqual(modules, want) {
		t.Errorf("modules %v, want %v", modules, want)
	}
}

// A malformed origins file is an error, never a silent docSource label.
func TestSourceIndexMalformedOrigins(t *testing.T) {
	base := originFixture(t, false)
	testkit.WriteFile(t, filepath.Join(base, "source", SourceOriginsFile), []byte(`["not", "an object"]`))
	if _, err := BuildSourceIndex(base); err == nil {
		t.Error("BuildSourceIndex accepted a malformed origins file")
	}
}
