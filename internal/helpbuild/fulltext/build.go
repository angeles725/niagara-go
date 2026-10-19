// Package fulltext builds niagara-help's full-text index of one data root,
// indexes/fulltext.sqlite (fulltext_index.py build and _fill): a contentless
// SQLite FTS5 database with one row per document in docs, its title and body
// terms (internal/help/fts.Terms) in fts, and the source fingerprint in meta,
// so the Python tool and the Go reader (internal/help/fts.Open) accept it as
// current. The SQLite file is not byte-identical to Python's; its logical
// content (schema, docs rows, indexed tokens, fingerprint) is.
package fulltext

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/pycompat"

	_ "modernc.org/sqlite" // database/sql driver "sqlite"
)

// schema is _fill's script, statement for statement (sqlite_master keeps the
// CREATE text as written).
var schema = []string{
	`PRAGMA journal_mode = OFF`,
	`PRAGMA synchronous = OFF`,
	`CREATE TABLE meta(key TEXT PRIMARY KEY, value TEXT)`,
	`CREATE TABLE docs(id INTEGER PRIMARY KEY, source TEXT, path TEXT,
                          title TEXT, line INTEGER, page INTEGER)`,
	`CREATE VIRTUAL TABLE fts USING fts5(title, body, content='')`,
}

// Options tune Build; the zero value builds indexes/fulltext.sqlite.
type Options struct {
	// Output is the database path (build's path argument); "" is
	// <versionRoot>/indexes/fulltext.sqlite.
	Output string
	// Workers read and tokenize files in parallel (the database is written
	// by one goroutine in doc id order); 0 is min(4, GOMAXPROCS).
	Workers int
}

// Result describes a finished build.
type Result struct {
	Path        string   // the database written
	Documents   int      // rows of docs (the last doc id)
	Files       int      // indexed files read (unreadable ones are skipped)
	Sources     []string // labels of the indexed sources, in doc id order
	Fingerprint string   // stored in meta (fulltext_index._fingerprint)
	// Log holds the lines Python's build prints on stderr, for the caller to
	// print verbatim; open_index's "building the full-text index for ..."
	// notice is the caller's.
	Log []string
}

// Build is fulltext_index.build(versionRoot, path=opts.Output): it indexes
// the text sources of versionRoot (internal/help/fts.SourceDirs, so docs-text
// and course-text may come from another registered version) into a temp file
// beside the target and renames it into place, so a failed build never
// leaves a partial database where a reader would look. It prints nothing.
func Build(versionRoot string, opts Options) (*Result, error) {
	dirs := fts.SourceDirs(versionRoot)
	fingerprint := fts.Fingerprint(dirs)
	final := opts.Output
	if final == "" {
		final = DefaultOutput(versionRoot)
	}
	tmp, err := atomicfile.TempPath(final) // 0666 less the umask, as SQLite creates a database
	if err != nil {
		return nil, err
	}
	res := &Result{Path: final, Fingerprint: fingerprint}
	for _, d := range dirs {
		res.Sources = append(res.Sources, d.Label)
	}
	err = atomicfile.Replace(final, tmp, func(tmp string) error {
		return fill(tmp, dirs, fingerprint, workers(opts.Workers), res)
	}, nil)
	if err != nil {
		return nil, err
	}
	n, err := pycompat.Format(res.Documents, ",")
	if err != nil {
		return nil, err
	}
	res.Log = []string{fmt.Sprintf("full-text index: %s documents -> %s", n, final)}
	return res, nil
}

// DefaultOutput is fulltext_index.db_path(versionRoot) as Python spells it,
// os.path.join(versionRoot, "indexes", "fulltext.sqlite"), the path Build
// writes and names in its log line when Options.Output is "".
func DefaultOutput(versionRoot string) string {
	return pycompat.Join(versionRoot, "indexes", fts.DBName)
}

func workers(n int) int {
	if n > 0 {
		return n
	}
	return min(4, runtime.GOMAXPROCS(0))
}

// fill is fulltext_index._fill on a new database at path.
func fill(path string, dirs []fts.SourceDir, fingerprint string, nWorkers int, res *Result) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // the PRAGMAs and the transaction share one connection
	ctx := context.Background()
	for _, stmt := range schema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op after Commit
	insDoc, err := tx.PrepareContext(ctx, `INSERT INTO docs VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	insFTS, err := tx.PrepareContext(ctx, `INSERT INTO fts(rowid, title, body) VALUES (?,?,?)`)
	if err != nil {
		return err
	}
	id := 0
	err = tokenized(files(dirs), nWorkers, func(rows []row) error {
		res.Files++
		for _, r := range rows {
			id++
			d := r.doc
			if _, err := insDoc.ExecContext(ctx, id, d.source, d.path, d.title, d.line, d.page); err != nil {
				return err
			}
			if _, err := insFTS.ExecContext(ctx, id, r.titleTerms, r.bodyTerms); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO meta VALUES ('fingerprint', ?)`, fingerprint); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO fts(fts) VALUES ('optimize')`); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	res.Documents = id
	return db.Close()
}

// row is one document with its fts column values (space-joined terms).
type row struct {
	doc                   document
	titleTerms, bodyTerms string
}

func tokenize(f file) ([]row, bool) {
	text, ok := readText(f.path)
	if !ok {
		return nil, false
	}
	docs := documentsOf(f, text)
	rows := make([]row, len(docs))
	for i, d := range docs {
		rows[i] = row{d, strings.Join(fts.Terms(d.titleText), " "), strings.Join(fts.Terms(d.body), " ")}
	}
	return rows, true
}

// batch is how many files the workers tokenize between two writes.
const batch = 256

// tokenized reads and tokenizes files on nWorkers goroutines and calls emit
// once per readable file, in order, on the calling goroutine.
func tokenized(fs []file, nWorkers int, emit func([]row) error) error {
	type out struct {
		rows []row
		ok   bool
	}
	results := make([]out, batch)
	for start := 0; start < len(fs); start += batch {
		chunk := fs[start:min(start+batch, len(fs))]
		jobs := make(chan int)
		done := make(chan struct{})
		for range min(nWorkers, len(chunk)) {
			go func() {
				for i := range jobs {
					rows, ok := tokenize(chunk[i])
					results[i] = out{rows, ok}
				}
				done <- struct{}{}
			}()
		}
		for i := range chunk {
			jobs <- i
		}
		close(jobs)
		for range min(nWorkers, len(chunk)) {
			<-done
		}
		for i := range chunk {
			if results[i].ok {
				if err := emit(results[i].rows); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// readText is open(path, encoding="utf-8", errors="replace").read(); ok is
// false when the file cannot be read (iter_documents skips it).
// Duplicates the unexported internal/help/fts readText (dedup candidate).
func readText(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return pycompat.UniversalNewlines(pycompat.DecodeReplace(data)), true
}

// file is one indexed file of iter_documents, in doc id order.
type file struct{ source, dir, path string }

// files lists every indexed file of dirs in index (doc id) order.
func files(dirs []fts.SourceDir) []file {
	var out []file
	for _, d := range dirs {
		for _, p := range fts.IterFiles(d.Label, d.Dir) {
			out = append(out, file{d.Label, d.Dir, p})
		}
	}
	return out
}

// documentsOf is the iter_documents part for one file and its text: the
// path stored in docs is relative to the source directory, "/"-separated.
func documentsOf(f file, text string) []document {
	rel, err := filepath.Rel(f.dir, f.path)
	if err != nil {
		rel = f.path
	}
	return documents(f.source, filepath.ToSlash(rel), text)
}
