package fts

// Reading the full-text index (fulltext_index.py). Each data root has one
// contentless FTS5 database, indexes/fulltext.sqlite, or a copy in the user
// cache directory when the data root is not writable, rebuilt whenever the
// text sources change. This package only reads it: a database is used when
// its stored fingerprint equals the fingerprint of the current sources (the
// same value fulltext_index.py computes). Open refuses a missing or stale index
// (*StaleIndexError); nh searches through internal/help/scope.OpenFulltext,
// which rebuilds it first like open_index (decision D38).

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
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
// sources exists: Open never builds one (scope.OpenFulltext does).
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
	return fmt.Sprintf("full-text index indexes/%s under %s %s; this reader never "+
		"builds it, rebuild it with: nh %sfind index",
		DBName, e.BaseDir, state, version)
}

// Index is an open, up-to-date full-text database (fulltext_index.FtsIndex).
type Index struct{ db *sql.DB }

// Opener opens the up-to-date index of a data root for a search: Open, or a
// caller's opener that rebuilds a missing or stale index first.
type Opener func(baseDir, versionID string) (*Index, error)

// Open is fulltext_index.open_index restricted to reading: the first of
// indexes/fulltext.sqlite and the cache copy whose stored fingerprint equals
// the current one, else a *StaleIndexError.
func Open(baseDir, versionID string) (*Index, error) {
	path, missing := FreshPath(baseDir, Fingerprint(SourceDirs(baseDir)))
	if path == "" {
		return nil, &StaleIndexError{BaseDir: baseDir, VersionID: versionID, Missing: missing}
	}
	return OpenPath(path)
}

// FreshPath is open_index's lookup: the first of indexes/fulltext.sqlite and
// the cache copy whose stored fingerprint equals fingerprint, "" when none
// does; missing reports that neither file exists.
func FreshPath(baseDir, fingerprint string) (path string, missing bool) {
	missing = true
	for _, p := range []string{DBPath(baseDir), CachePath(baseDir)} {
		if !pycompat.IsFile(p) {
			continue
		}
		missing = false
		if storedFingerprint(p) == fingerprint {
			return p, false
		}
	}
	return "", missing
}

// OpenPath opens the full-text database at path, read only.
func OpenPath(path string) (*Index, error) {
	db, err := index.OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	return &Index{db}, nil
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

// maxVars is fulltext_index.MAX_VARS: the terms bound per statement.
const maxVars = 500

// DocCounts is FtsIndex.doc_counts: for each distinct term the number of
// indexed documents containing it (from an fts5vocab "row" table); a term
// in no document is absent from the map.
func (x *Index) DocCounts(terms []string) (map[string]int, error) {
	ctx := context.Background()
	conn, err := x.db.Conn(ctx) // the temp table lives on one connection
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	// The database file is opened mode=ro; query_only is lifted only to
	// create the in-memory temp table, then restored.
	for _, stmt := range []string{"PRAGMA query_only=0",
		"CREATE VIRTUAL TABLE IF NOT EXISTS temp.vocab USING fts5vocab(main, fts, 'row')",
		"PRAGMA query_only=1"} {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return nil, err
		}
	}
	uniq := map[string]bool{}
	var sorted []string
	for _, t := range terms {
		if !uniq[t] {
			uniq[t] = true
			sorted = append(sorted, t)
		}
	}
	sort.Strings(sorted)
	counts := map[string]int{}
	for i := 0; i < len(sorted); i += maxVars {
		chunk := sorted[i:min(i+maxVars, len(sorted))]
		rs, err := conn.QueryContext(ctx, "SELECT term, doc FROM temp.vocab WHERE term IN ("+
			marks(len(chunk))+")", args(nil, chunk)...)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			var term string
			var doc int
			if err := rs.Scan(&term, &doc); err != nil {
				rs.Close()
				return nil, err
			}
			counts[term] = doc
		}
		if err := rs.Close(); err != nil {
			return nil, err
		}
	}
	return counts, nil
}
