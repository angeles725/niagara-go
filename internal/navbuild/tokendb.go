package navbuild

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"

	_ "modernc.org/sqlite" // database/sql driver "sqlite"
)

// The SQLite side of the token index. internal/index opens SQLite read-only
// only, so the writer uses modernc.org/sqlite directly, as
// internal/helpbuild/fulltext does.

// tokenSchema is build_index's CREATE TABLE statements as sqlite_master keeps
// them (the text of Python's triple-quoted strings, outer whitespace dropped).
var tokenSchema = []string{
	"CREATE TABLE files (\n            id INTEGER PRIMARY KEY,\n            path TEXT NOT NULL,\n" +
		"            module TEXT NOT NULL,\n            class_name TEXT NOT NULL\n        )",
	"CREATE TABLE postings (\n            token TEXT NOT NULL,\n            file_id INTEGER NOT NULL,\n" +
		"            line_no INTEGER NOT NULL\n        )",
	"CREATE TABLE meta (\n            key TEXT PRIMARY KEY,\n            value TEXT NOT NULL\n        )",
}

// tokenIndexes are created after the bulk insert, as in Python, with Python's
// names (idx_token_module has no module column: the names are kept for parity).
var tokenIndexes = []string{
	"CREATE INDEX idx_token ON postings(token)",
	"CREATE INDEX idx_token_module ON postings(token, file_id)",
	"CREATE INDEX idx_file_module ON files(module)",
}

// postingsPerInsert is how many rows one multi-row INSERT carries (3 bound
// parameters each, far below SQLite's limit).
const postingsPerInsert = 300

// writeTokenDB builds the database in <index>.tmp and renames it into place
// (build.tmp_db/publish_db): a stale temp database of a killed build is
// removed first, and a failed build leaves the previous index untouched and
// removes its temp database.
func writeTokenDB(ti *TokenIndex, sources []tokenSource) error {
	if err := os.MkdirAll(filepath.Dir(ti.Path), 0o777); err != nil {
		return err
	}
	tmp := ti.Path + ".tmp"
	removeDB(tmp)
	if err := fillTokenDB(tmp, ti, sources); err != nil {
		removeDB(tmp)
		return err
	}
	if err := os.Rename(tmp, ti.Path); err != nil {
		removeDB(tmp)
		return err
	}
	return nil
}

func removeDB(path string) {
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		os.Remove(path + suffix)
	}
}

// fillTokenDB is build_index from the CREATE TABLEs to the VACUUM.
func fillTokenDB(path string, ti *TokenIndex, sources []tokenSource) error {
	// A file: URI, so a '?' or '#' in the path is part of the name.
	db, err := sql.Open("sqlite", (&url.URL{Scheme: "file", Opaque: url.PathEscape(path)}).String())
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // the PRAGMAs and the transaction share one connection
	ctx := context.Background()
	exec := func(stmts ...string) error {
		for _, s := range stmts {
			if _, err := db.ExecContext(ctx, s); err != nil {
				return fmt.Errorf("%s: %w", strings.Fields(s)[0], err)
			}
		}
		return nil
	}
	// No rollback journal: a failed build discards the whole file anyway.
	if err := exec("PRAGMA journal_mode=OFF", "PRAGMA synchronous=OFF",
		"PRAGMA cache_size=-64000", "PRAGMA temp_store=MEMORY"); err != nil {
		return err
	}
	if err := exec(tokenSchema...); err != nil {
		return err
	}
	start := time.Now()
	if err := insertTokens(ctx, db, ti, sources); err != nil {
		return err
	}
	ti.InsertTime = time.Since(start)
	t0 := time.Now()
	if err := exec(tokenIndexes...); err != nil {
		return err
	}
	ti.IndexTime = time.Since(t0)
	ti.Elapsed = time.Since(start)
	meta := [][2]string{
		{"timestamp", now().Format("2006-01-02 15:04:05")},
		{"build_time_sec", pycompat.FloatRepr(round1(ti.Elapsed.Seconds()))},
		{"files_scanned", strconv.Itoa(ti.FilesScanned)},
		{"files_with_tokens", strconv.Itoa(ti.FilesWithTokens)},
		{"total_lines", strconv.Itoa(ti.TotalLines)},
		{"total_postings", strconv.Itoa(ti.TotalPostings)},
		{"unique_tokens", strconv.Itoa(ti.UniqueTokens)},
		{"min_token_len", strconv.Itoa(ti.MinTokenLen)},
		{"stop_words_count", strconv.Itoa(len(tokenStopWords))},
		{"source_root", ti.Source},
	}
	for _, kv := range meta {
		if _, err := db.ExecContext(ctx, "INSERT INTO meta (key, value) VALUES (?, ?)", kv[0], kv[1]); err != nil {
			return err
		}
	}
	// ANALYZE feeds the readers' query planner; VACUUM leaves the same
	// compact file as Python's.
	if err := exec("ANALYZE", "VACUUM"); err != nil {
		return err
	}
	return db.Close()
}

// insertTokens tokenizes the sources in parallel and inserts their files and
// postings rows in scan order inside one transaction.
func insertTokens(ctx context.Context, db *sql.DB, ti *TokenIndex, sources []tokenSource) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	insFile, err := tx.PrepareContext(ctx, "INSERT INTO files (id, path, module, class_name) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	one := "(?, ?, ?)"
	insMany, err := tx.PrepareContext(ctx, "INSERT INTO postings (token, file_id, line_no) VALUES "+
		strings.TrimSuffix(strings.Repeat(one+", ", postingsPerInsert), ", "))
	if err != nil {
		return err
	}
	insOne, err := tx.PrepareContext(ctx, "INSERT INTO postings (token, file_id, line_no) VALUES "+one)
	if err != nil {
		return err
	}
	unique := map[string]struct{}{}
	args := make([]any, 0, 3*postingsPerInsert)
	pending := 0
	flush := func(stmt *sql.Stmt) error {
		_, err := stmt.ExecContext(ctx, args...)
		args, pending = args[:0], 0
		return err
	}
	ti.FilesScanned = len(sources)
	err = inOrder(len(sources), func(i int) tokenizedFile {
		return tokenizeFile(sources[i].full, ti.MinTokenLen)
	}, func(i int, tf tokenizedFile) error {
		if !tf.readable || tf.lines == 0 {
			return nil
		}
		s := sources[i]
		if s.moduleNull {
			// files.module is NOT NULL: Python dies with an IntegrityError here.
			return fmt.Errorf("%s: classes[%q] has a null module (files.module is NOT NULL)", ClassIndexFile, s.className)
		}
		fileID := i + 1
		if _, err := insFile.ExecContext(ctx, fileID, s.rel, s.module, s.className); err != nil {
			return err
		}
		ti.TotalLines += tf.lines
		if tf.hasTokens {
			ti.FilesWithTokens++
		}
		for _, p := range tf.postings {
			if _, ok := unique[p.token]; !ok {
				unique[strings.Clone(p.token)] = struct{}{}
			}
			args = append(args, p.token, fileID, p.line)
			pending++
			if pending == postingsPerInsert {
				if err := flush(insMany); err != nil {
					return err
				}
			}
		}
		ti.TotalPostings += len(tf.postings)
		return nil
	})
	if err != nil {
		return err
	}
	for len(args) > 0 {
		row := args[:3]
		if _, err := insOne.ExecContext(ctx, row...); err != nil {
			return err
		}
		args = args[3:]
	}
	ti.UniqueTokens = len(unique)
	return tx.Commit()
}
