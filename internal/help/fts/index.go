package fts

// Reading the full-text index (fulltext_index.py). The Python tool keeps one
// contentless FTS5 database per data root, indexes/fulltext.sqlite, or a copy
// in the user cache directory when the data root is not writable, and
// rebuilds it whenever the text sources change. Go only reads it: a database
// is used when its stored fingerprint equals the fingerprint Python computes
// for the current sources; otherwise the search cannot run (exit 2) and the
// error names the Python command that rebuilds it.

import (
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// EngineVersion is fulltext_index.ENGINE_VERSION, part of the fingerprint.
const EngineVersion = "3"

// DBName is fulltext_index.DB_NAME.
const DBName = "fulltext.sqlite"

// Fingerprint is fulltext_index._fingerprint: per source its directory, file
// count, bytes, newest mtime (ns) and a SHA-1 over every file's relative
// path, size and mtime, so any rename, copy or edit marks the index stale.
func Fingerprint(dirs []SourceDir) string {
	parts := []string{"engine=" + EngineVersion}
	for _, d := range dirs {
		files, size, newest := 0, int64(0), int64(0)
		h := sha1.New()
		for _, path := range IterFiles(d.Label, d.Dir) {
			st, err := os.Stat(path)
			if err != nil {
				continue
			}
			mtime := st.ModTime().UnixNano()
			files++
			size += st.Size()
			newest = max(newest, mtime)
			r, err := filepath.Rel(d.Dir, path) // os.path.relpath: OS separators
			if err != nil {
				r = path
			}
			fmt.Fprintf(h, "%s\x00%d\x00%d\n", r, st.Size(), mtime)
		}
		parts = append(parts, fmt.Sprintf("%s=%s:%d:%d:%d:%s", d.Label, d.Dir, files, size,
			newest, hex.EncodeToString(h.Sum(nil))))
	}
	return strings.Join(parts, "|")
}

// DBPath is fulltext_index.db_path.
func DBPath(baseDir string) string { return filepath.Join(baseDir, "indexes", DBName) }

// CachePath is fulltext_index.cache_path: the index of a data root that is
// not writable, $XDG_CACHE_HOME (or ~/.cache)/niagara-help/<root hash>/.
func CachePath(baseDir string) string {
	home := os.Getenv("XDG_CACHE_HOME")
	if home == "" {
		h, _ := os.UserHomeDir()
		home = filepath.Join(h, ".cache")
	}
	sum := sha1.Sum([]byte(abs(baseDir)))
	return filepath.Join(home, "niagara-help", hex.EncodeToString(sum[:])[:16], DBName)
}

// StaleIndexError means no index whose fingerprint matches the current text
// sources exists: Go never builds one (the search exits 2).
type StaleIndexError struct {
	BaseDir   string
	VersionID string // for the rebuild command; "" without a registry
	Missing   bool   // neither indexes/ nor the cache directory holds one
}

func (e *StaleIndexError) Error() string {
	state := "is out of date (the text sources changed since it was built)"
	if e.Missing {
		state = "is missing"
	}
	version := ""
	if e.VersionID != "" {
		version = "--niagara-version " + e.VersionID + " "
	}
	return fmt.Sprintf("full-text index indexes/%s under %s %s; this port reads it but "+
		"never builds it, rebuild it with: python3 tools/niagara_help.py %sfind index",
		DBName, e.BaseDir, state, version)
}

// Index is an open, up-to-date full-text database (fulltext_index.FtsIndex).
type Index struct{ db *sql.DB }

// Open is fulltext_index.open_index restricted to reading: the first of
// indexes/fulltext.sqlite and the cache copy whose stored fingerprint equals
// the current one, else a *StaleIndexError.
func Open(baseDir, versionID string) (*Index, error) {
	current := Fingerprint(SourceDirs(baseDir))
	missing := true
	for _, path := range []string{DBPath(baseDir), CachePath(baseDir)} {
		if !isFile(path) {
			continue
		}
		missing = false
		if storedFingerprint(path) == current {
			db, err := index.OpenSQLite(path)
			if err != nil {
				return nil, err
			}
			return &Index{db}, nil
		}
	}
	return nil, &StaleIndexError{BaseDir: baseDir, VersionID: versionID, Missing: missing}
}

// storedFingerprint is fulltext_index._stored_fingerprint: "" when the
// database cannot be read or has none.
func storedFingerprint(path string) string {
	db, err := index.OpenSQLite(path)
	if err != nil {
		return ""
	}
	defer db.Close()
	var v sql.NullString
	if err := db.QueryRow(`SELECT value FROM meta WHERE key='fingerprint'`).Scan(&v); err != nil {
		return ""
	}
	return v.String
}

// Close releases the database.
func (x *Index) Close() error { return x.db.Close() }

// row is one matching document: (id, source, path, title, line, page, bm25).
type row struct {
	id                  int64
	source, path, title string
	line, page          int
	bm25                float64
}

// matchExpr is fulltext_index._match_expr: each group an OR of quoted terms
// (a prefix term as "term"*), the groups joined by AND or OR.
func matchExpr(groups []Group, mode string) string {
	parts := make([]string, len(groups))
	for i, g := range groups {
		terms := make([]string, len(g))
		for j, t := range g {
			if strings.HasSuffix(t, "*") {
				terms[j] = `"` + t[:len(t)-1] + `"*`
			} else {
				terms[j] = `"` + t + `"`
			}
		}
		parts[i] = "(" + strings.Join(terms, " OR ") + ")"
	}
	if mode == "and" {
		return strings.Join(parts, " AND ")
	}
	return strings.Join(parts, " OR ")
}

func marks(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

func args(first []any, srcs []string) []any {
	for _, s := range srcs {
		first = append(first, s)
	}
	return first
}

// rows is FtsIndex.rows: every document of srcs matching the groups (all of
// them for mode "and") with its BM25 (title column weighted titleWeight).
// The statement is Python's, without ORDER BY: rows come in SQLite's order.
func (x *Index) rows(groups []Group, mode string, srcs []string, titleWeight float64) ([]row, error) {
	q := "SELECT d.id, d.source, d.path, d.title, d.line, d.page, bm25(fts, ?, 1.0) " +
		"FROM fts JOIN docs d ON d.id = fts.rowid WHERE fts MATCH ? AND d.source IN (" +
		marks(len(srcs)) + ")"
	rs, err := x.db.Query(q, args([]any{titleWeight, matchExpr(groups, mode)}, srcs)...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.id, &r.source, &r.path, &r.title, &r.line, &r.page, &r.bm25); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// groupIDs is FtsIndex.group_ids: the ids of the documents of srcs that
// match each group.
func (x *Index) groupIDs(groups []Group, srcs []string) ([]map[int64]bool, error) {
	q := "SELECT d.id FROM fts JOIN docs d ON d.id = fts.rowid WHERE fts MATCH ? AND d.source IN (" +
		marks(len(srcs)) + ")"
	out := make([]map[int64]bool, len(groups))
	for i, g := range groups {
		rs, err := x.db.Query(q, args([]any{matchExpr([]Group{g}, "or")}, srcs)...)
		if err != nil {
			return nil, err
		}
		set := map[int64]bool{}
		for rs.Next() {
			var id int64
			if err := rs.Scan(&id); err != nil {
				rs.Close()
				return nil, err
			}
			set[id] = true
		}
		err = rs.Err()
		rs.Close()
		if err != nil {
			return nil, err
		}
		out[i] = set
	}
	return out, nil
}
