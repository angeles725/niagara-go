package license

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestScanFileMatchesPython replays testdata/scan.json (gen_scan.py): the calls
// license_features._scan_file finds in sample sources, read back through the
// resolver (replacement decoding, universal newlines) with a files table for
// the cross-file constant and superclass lookups.
func TestScanFileMatchesPython(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "scan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Files map[string]string `json:"files"`
		Raw   map[string]string `json:"raw"`
		Scans [][4]json.RawMessage
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "files.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT, module TEXT, class_name TEXT)"); err != nil {
		t.Fatal(err)
	}
	write := func(rel string, data []byte) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		cls := strings.TrimSuffix(filepath.Base(rel), ".java")
		if _, err := db.Exec("INSERT INTO files (path, module, class_name) VALUES (?, ?, ?)",
			rel, strings.Split(rel, "/")[1], cls); err != nil {
			t.Fatal(err)
		}
	}
	for rel, text := range fx.Files {
		write(rel, []byte(text))
	}
	for rel, h := range fx.Raw {
		b, err := hex.DecodeString(h)
		if err != nil {
			t.Fatal(err)
		}
		write(rel, b)
	}
	res := newResolver(root, db)
	for _, s := range fx.Scans {
		var rel, module, cls string
		var want [][4]*string
		for i, dst := range []any{&rel, &module, &cls, &want} {
			if err := json.Unmarshal(s[i], dst); err != nil {
				t.Fatal(err)
			}
		}
		text, ok := res.text(rel)
		if !ok {
			t.Fatalf("cannot read %s", rel)
		}
		var got [][4]*string
		for _, c := range scanFile(text, rel, module, cls, res) {
			form, recv := c.Form, c.Receiver
			got = append(got, [4]*string{&form, &recv, c.Vendor, c.Feature})
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got  %s\n want %s", rel, show(got), show(want))
		}
	}
	if res.err != nil {
		t.Fatal(res.err)
	}
}

func show(calls [][4]*string) string {
	var b strings.Builder
	for _, c := range calls {
		b.WriteString("[")
		for _, p := range c {
			if p == nil {
				b.WriteString(" nil")
			} else {
				b.WriteString(" " + *p)
			}
		}
		b.WriteString(" ]")
	}
	return b.String()
}
