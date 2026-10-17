package index

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a b#.db") // characters a DSN must escape
	rw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Exec(`CREATE TABLE t (x INTEGER); INSERT INTO t VALUES (7)`); err != nil {
		t.Fatal(err)
	}
	rw.Close()

	db, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var x int
	if err := db.QueryRow(`SELECT x FROM t`).Scan(&x); err != nil || x != 7 {
		t.Fatalf("read: %d, %v", x, err)
	}
	if _, err := db.Exec(`INSERT INTO t VALUES (8)`); err == nil {
		t.Error("a write succeeded on the read-only connection")
	}
	if _, err := OpenSQLite(filepath.Join(t.TempDir(), "missing.db")); err == nil {
		t.Error("a missing database opened")
	}
}
