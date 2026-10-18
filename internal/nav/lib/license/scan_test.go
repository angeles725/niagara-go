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
// and getLicenseManager() sites license_features._scan_file finds in sample
// sources (call form, receiver, vendor, feature, line and arguments; site line,
// features and reason), read back through the
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
		Scans [][5]json.RawMessage
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
		var wantCalls, wantSites []json.RawMessage
		for i, dst := range []any{&rel, &module, &cls, &wantCalls, &wantSites} {
			if err := json.Unmarshal(s[i], dst); err != nil {
				t.Fatal(err)
			}
		}
		text, ok := res.text(rel)
		if !ok {
			t.Fatalf("cannot read %s", rel)
		}
		calls, sites := scanFile(text, rel, module, cls, res)
		var gotCalls, gotSites []string
		for _, c := range calls {
			gotCalls = append(gotCalls, jsonOf(t, []any{c.Form, c.Receiver, c.Vendor, c.Feature, c.Line, c.Args}))
		}
		for _, st := range sites {
			var reason *string
			if st.Reason != "" {
				reason = &st.Reason
			}
			features := st.Features
			if features == nil {
				features = []string{}
			}
			gotSites = append(gotSites, jsonOf(t, []any{st.Line, features, reason}))
		}
		if want := compact(t, wantCalls); !reflect.DeepEqual(gotCalls, want) {
			t.Errorf("%s calls:\n got  %q\n want %q", rel, gotCalls, want)
		}
		if want := compact(t, wantSites); !reflect.DeepEqual(gotSites, want) {
			t.Errorf("%s sites:\n got  %q\n want %q", rel, gotSites, want)
		}
	}
	if res.err != nil {
		t.Fatal(res.err)
	}
}

// jsonOf is v as compact JSON.
func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// compact re-encodes every raw record as compact JSON.
func compact(t *testing.T, raws []json.RawMessage) []string {
	t.Helper()
	var out []string
	for _, r := range raws {
		var v any
		if err := json.Unmarshal(r, &v); err != nil {
			t.Fatal(err)
		}
		out = append(out, jsonOf(t, v))
	}
	return out
}

// TestIsLicenseCallReceiverEndingInDot: a receiver ending in "." leaves an
// empty variable name, which the Python regexes still search (\b\s*=...). The
// expected values come from license_features._is_license_call.
func TestIsLicenseCallReceiverEndingInDot(t *testing.T) {
	cases := []struct {
		recv, text string
		want       bool
	}{
		{"a.", "x.getFeature(a, b); int y = 2;", false},
		{"a.", "m = getLicenseManager(); x.getFeature(a, b);", true},
		{"a..", "no manager here", false},
	}
	for _, c := range cases {
		if got := isLicenseCall("getFeature", c.recv, c.text, "p/X.java"); got != c.want {
			t.Errorf("isLicenseCall(%q, %q) = %v, want %v", c.recv, c.text, got, c.want)
		}
	}
}

func TestOccurrencesEmptyWordVisitsEveryOffset(t *testing.T) {
	var got []int
	occurrences("ab", "", func(i int) bool { got = append(got, i); return false })
	if want := []int{0, 1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("occurrences(\"ab\", \"\") visited %v, want %v", got, want)
	}
}
