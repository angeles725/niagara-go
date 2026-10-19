package fulltext

// The canonical dump of an index (testdata/synthetic.dump holds the Python
// builder's, frozen when D47 retired the Python reference). SQLite
// files are never byte-identical (page layout, FTS5 segment ids and merge
// history differ), so two indexes are equal when these records are: the
// schema text, the FTS5 config, meta (the fingerprint), every docs row with
// its SQLite types, the per-document token counts (fts_docsize), the column
// totals (the averages record) and every indexed token instance
// (fts5vocab 'instance': term, doc, column, position), which is the whole
// content of a contentless FTS5 table.

import (
	"database/sql"
	"fmt"
	"iter"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/help/fts"
)

// dumpQueries are the sections, each a query whose columns are joined by
// tabs after the section name.
var dumpQueries = []struct{ name, query string }{
	{"schema", `SELECT type, name, tbl_name, sql FROM sqlite_master ORDER BY type, name`},
	{"config", `SELECT k, quote(v) FROM fts_config ORDER BY k`},
	{"meta", `SELECT quote(key), quote(value) FROM meta ORDER BY key`},
	{"docs", `SELECT quote(id), quote(source), quote(path), quote(title), quote(line), quote(page) FROM docs ORDER BY id`},
	{"docsize", `SELECT id, hex(sz) FROM fts_docsize ORDER BY id`},
	{"averages", `SELECT hex(block) FROM fts_data WHERE id = 1`},
	{"inst", `SELECT term, doc, col, "offset" FROM temp.inst ORDER BY term, doc, col, "offset"`},
}

func openRO(path string) (*sql.DB, error) {
	dsn := (&url.URL{Scheme: "file", Opaque: url.PathEscape(path), RawQuery: "mode=ro"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // temp.inst lives on the one connection
	if _, err := db.Exec(`CREATE VIRTUAL TABLE temp.inst USING fts5vocab(main, fts, 'instance')`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// dump yields the canonical records of the index at path; rewrite edits
// each record (to hide a temp directory in the fingerprint). A failure is
// yielded as a final "error" record, so it shows up as a difference.
func dump(path string, rewrite func(string) string) iter.Seq[string] {
	return func(yield func(string) bool) {
		db, err := openRO(path)
		if err != nil {
			yield("error\t" + err.Error())
			return
		}
		defer db.Close()
		for _, q := range dumpQueries {
			more, err := section(db, q.name, q.query, rewrite, yield)
			if err != nil {
				yield("error\t" + q.name + ": " + err.Error())
				return
			}
			if !more {
				return // the consumer stopped: run no further query
			}
		}
	}
}

// section yields one section's records; more is false once yield asked
// to stop.
func section(db *sql.DB, name, query string, rewrite func(string) string, yield func(string) bool) (more bool, err error) {
	rows, err := db.Query(query)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	var b strings.Builder
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return false, err
		}
		b.Reset()
		b.WriteString(name)
		for _, v := range vals {
			b.WriteByte('\t')
			if !v.Valid {
				b.WriteString("NULL")
				continue
			}
			s := v.String
			if name == "schema" {
				s = strings.Join(strings.Fields(s), " ")
			}
			b.WriteString(s)
		}
		line := b.String()
		if rewrite != nil {
			line = rewrite(line)
		}
		if !yield(line) {
			return false, nil
		}
	}
	return true, rows.Err()
}

// dumpText is the whole dump as text, one record per line.
func dumpText(path string, rewrite func(string) string) string {
	var b strings.Builder
	for line := range dump(path, rewrite) {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// compareDumps walks two dumps in lockstep (they can hold tens of millions
// of records) and returns the record count and up to max differences.
func compareDumps(a, b iter.Seq[string], max int) (n int, diffs []string) {
	nextA, stopA := iter.Pull(a)
	defer stopA()
	nextB, stopB := iter.Pull(b)
	defer stopB()
	for {
		la, okA := nextA()
		lb, okB := nextB()
		if !okA && !okB {
			return n, diffs
		}
		n++
		if la != lb || okA != okB {
			if len(diffs) < max {
				diffs = append(diffs, fmt.Sprintf("record %d:\n  want: %q\n  got:  %q", n, la, lb))
			} else {
				return n, append(diffs, "... (stopped)")
			}
		}
	}
}

func TestCompareDumpsReportsFirstDifferences(t *testing.T) {
	seq := func(xs ...string) iter.Seq[string] {
		return func(yield func(string) bool) {
			for _, x := range xs {
				if !yield(x) {
					return
				}
			}
		}
	}
	tests := []struct {
		name  string
		a, b  []string
		diffs int
	}{
		{"equal", []string{"x", "y"}, []string{"x", "y"}, 0},
		{"changed record", []string{"x", "y"}, []string{"x", "z"}, 1},
		{"missing tail", []string{"x", "y"}, []string{"x"}, 1},
		{"longer second dump", []string{"x"}, []string{"x", "y", "z"}, 2},
		{"capped at max, then stopped", []string{"a", "b", "c", "d", "e", "f", "g"}, []string{"1", "2", "3", "4", "5", "6", "7"}, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, d := compareDumps(seq(tt.a...), seq(tt.b...), 5); len(d) != tt.diffs {
				t.Errorf("got %d differences %v, want %d", len(d), d, tt.diffs)
			}
		})
	}
}

func TestDumpStopsQueryingWhenTheConsumerStops(t *testing.T) {
	root := writeTree(t)
	out := filepath.Join(t.TempDir(), fts.DBName)
	if _, err := Build(root, Options{Output: out}); err != nil {
		t.Fatal(err)
	}
	produced := 0
	for range dump(out, func(s string) string { produced++; return s }) {
		break
	}
	if produced != 1 {
		t.Errorf("%d records produced after the consumer stopped at the first, want 1", produced)
	}
}
