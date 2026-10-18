package license

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// tokenBase writes a token index listing files (path -> source; "" = not on
// disk) under a temporary base and corpus, and returns both.
func tokenBase(t *testing.T, files map[string]string) (base, corpus string) {
	t.Helper()
	base, corpus = t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(base, "indexes"), 0o755)
	db, err := sql.Open("sqlite", filepath.Join(base, "indexes", "token-index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT, module TEXT, class_name TEXT);
		CREATE TABLE postings (token TEXT, file_id INTEGER, line_no INTEGER);
		CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT); INSERT INTO meta VALUES ('timestamp', 'T3')`); err != nil {
		t.Fatal(err)
	}
	for rel, src := range files {
		res, err := db.Exec("INSERT INTO files (path, module, class_name) VALUES (?, 'm-rt', 'C')", rel)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		db.Exec("INSERT INTO postings VALUES ('checkFeature', ?, 1), ('getFeature', ?, 2)", id, id)
		if src != "" {
			p := filepath.Join(corpus, rel)
			os.MkdirAll(filepath.Dir(p), 0o755)
			if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return base, corpus
}

// scan runs Scan and renders its outcome the way the CLI reports it: the
// stderr text and the exit status.
func scan(t *testing.T, base, corpus string) (*Report, string, int) {
	t.Helper()
	t.Setenv("NAV_CORPUS_BASE", corpus)
	rep, f := Scan(base)
	switch {
	case f != nil:
		stderr := "ERROR: " + f.Message + "\n"
		if f.Hint != "" {
			stderr += "  " + f.Hint + "\n"
		}
		return rep, stderr, 2
	case rep.Warning != "":
		return rep, "WARNING: " + rep.Warning + "\n", 0
	}
	return rep, "", 0
}

// TestScan pins license_report's outcomes: calls with the candidate counts,
// the unreadable-files warning, the all-unreadable and missing-index errors.
func TestScan(t *testing.T) {
	src := `class C { void f() { Sys.getLicenseManager().checkFeature("v", "f"); } }`
	base, corpus := tokenBase(t, map[string]string{"m/m-rt/a/C.java": src, "m/m-rt/a/Gone.java": ""})
	rep, stderr, code := scan(t, base, corpus)
	want := "WARNING: could not read 1 of 2 candidate files under " + corpus + " (e.g. m/m-rt/a/Gone.java); " +
		"results exclude them (license-features --unresolved lists them)\n"
	if rep == nil || stderr != want || code != 0 {
		t.Fatalf("report %v, stderr %q, exit %d", rep, stderr, code)
	}
	if rep.Files != 2 || rep.Modules != 1 || rep.IndexBuilt != "T3" || len(rep.Calls) != 1 ||
		*rep.Calls[0].Vendor != "v" || *rep.Calls[0].Feature != "f" || rep.Calls[0].Module != "m-rt" {
		t.Errorf("report %+v", rep)
	}

	base, corpus = tokenBase(t, map[string]string{"m/m-rt/a/Gone.java": ""})
	rep, stderr, code = scan(t, base, corpus)
	want = "ERROR: could not read 1 of 1 candidate files under " + corpus + " (e.g. m/m-rt/a/Gone.java)\n" +
		"  Check the corpus or rebuild the token index: ./reindex.sh\n"
	if rep != nil || stderr != want || code != 2 {
		t.Errorf("all unreadable: report %v, stderr %q, exit %d", rep, stderr, code)
	}

	empty := t.TempDir()
	rep, stderr, code = scan(t, empty, corpus)
	want = "ERROR: token index not found: " + filepath.Join(empty, "indexes", "token-index.db") +
		"\n  Run: ./reindex.sh\n"
	if rep != nil || stderr != want || code != 2 {
		t.Errorf("no token index: report %v, stderr %q, exit %d", rep, stderr, code)
	}
}

// TestScanIndexTakesExplicitInputs: the explicit-input entry ignores
// NAV_CORPUS_BASE and reads the corpus it is given.
func TestScanIndexTakesExplicitInputs(t *testing.T) {
	src := `class C { void f() { Sys.getLicenseManager().checkFeature("v", "f"); } }`
	base, corpus := tokenBase(t, map[string]string{"m/m-rt/a/C.java": src})
	t.Setenv("NAV_CORPUS_BASE", t.TempDir())
	rep, f := ScanIndex(filepath.Join(base, "indexes", "token-index.db"), corpus)
	if f != nil || len(rep.Calls) != 1 || len(rep.Sites) != 1 || rep.Sites[0].Features[0] != "f" {
		t.Fatalf("ScanIndex = %+v, %v", rep, f)
	}
	if _, f := ScanIndex(filepath.Join(base, "missing.db"), corpus); f == nil || f.Hint != "Run: ./reindex.sh" {
		t.Errorf("missing index: %v", f)
	}
}
