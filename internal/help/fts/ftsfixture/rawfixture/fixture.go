// Package rawfixture installs the full-text test checkout: a copy of
// internal/help/testdata/fulltext with the mtimes and the Python-built indexes
// that testdata/fulltext_gen.py used, so the stored fingerprints match the
// copy on Linux. It does not import fts, so fts's own tests can use it;
// every other test uses ftsfixture, which also fixes the fingerprints for
// the host.
package rawfixture

import (
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite" // read-write access to the copied fixture databases
)

// FixedNS is fulltext_gen.py FIXED_NS, the mtime of every fixture file.
const FixedNS = 1_700_000_000_123_456_789

// Root is the placeholder for the checkout folder in the stored fingerprints
// and in the recorded outputs.
const Root = "<ROOT>"

// Versions maps each fixture version to its data root inside the checkout.
var Versions = map[string]string{"1.0": ".", "2.0": filepath.Join("versions", "2.0")}

// Checkout copies testdata/fulltext (testdata is the help testdata folder)
// to a temporary folder, sets every file's mtime to FixedNS and, unless
// withoutIndexes, installs fulltext-db/<version>.sqlite as each data root's
// indexes/fulltext.sqlite with <ROOT> in its fingerprint replaced by the
// copy's path. It returns the checkout path.
func Checkout(t testing.TB, testdata string, withoutIndexes bool) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join(testdata, "fulltext"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "checkout")
	err = filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dst, strings.TrimPrefix(path, src))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		mtime := time.Unix(0, FixedNS)
		return os.Chtimes(target, mtime, mtime)
	})
	if err != nil {
		t.Fatal(err)
	}
	if withoutIndexes {
		return dst
	}
	for vid, rel := range Versions {
		db := filepath.Join(dst, rel, "indexes", "fulltext.sqlite")
		Install(t, filepath.Join(testdata, "fulltext-db", vid+".sqlite"), db, dst)
	}
	return dst
}

// Install copies a fixture database to dest with <ROOT> in its stored
// fingerprint replaced by root.
func Install(t testing.TB, fixture, dest, root string) {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		t.Fatal(err)
	}
	SetFingerprint(t, dest, "", Root, root)
}

// SetFingerprint rewrites the stored fingerprint of db: to value, or (value
// empty) with old replaced by new.
func SetFingerprint(t testing.TB, db, value, old, new string) {
	t.Helper()
	con, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	defer con.Close()
	if value != "" {
		_, err = con.Exec(`UPDATE meta SET value = ? WHERE key = 'fingerprint'`, value)
	} else {
		_, err = con.Exec(`UPDATE meta SET value = replace(value, ?, ?) WHERE key = 'fingerprint'`, old, new)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// Fingerprinter is fts.Fingerprint(fts.SourceDirs(base)), handed in by the
// caller: ftsfixture cannot import fts, whose own tests use this package.
type Fingerprinter func(base string) string

// Refresh rewrites, on Windows only, the stored fingerprint of db (the index
// of the data root base) to fp(base). The fixture databases were built on
// Linux, and Windows never reproduces their fingerprints: NTFS keeps mtimes in
// 100 ns ticks and the source directories are spelled with "\". Off Windows
// it does nothing, so the Python-built value is still what the tests check.
func Refresh(t testing.TB, db, base string, fp Fingerprinter) {
	t.Helper()
	if runtime.GOOS == "windows" {
		SetFingerprint(t, db, fp(base), "", "")
	}
}

// RefreshAll is Refresh for the index of every version of a checkout.
func RefreshAll(t testing.TB, checkout string, fp Fingerprinter) {
	t.Helper()
	for _, rel := range Versions {
		base := filepath.Join(checkout, rel)
		Refresh(t, filepath.Join(base, "indexes", "fulltext.sqlite"), base, fp)
	}
}
