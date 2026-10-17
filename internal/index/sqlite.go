package index

import (
	"database/sql"
	"net/url"

	_ "modernc.org/sqlite" // the pure-Go SQLite driver (no cgo); the only one allowed
)

// OpenSQLite opens the SQLite index at path read-only, the way module-navigator's
// indexes.connect does (PRAGMA query_only=ON, a 16 MB page cache). The file must
// exist: the driver is asked not to create it. One connection is kept so the
// pragmas hold for every query.
func OpenSQLite(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Set("mode", "ro")
	q.Add("_pragma", "query_only(1)")
	q.Add("_pragma", "cache_size(-16000)")
	dsn := (&url.URL{Scheme: "file", Opaque: url.PathEscape(path), RawQuery: q.Encode()}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
