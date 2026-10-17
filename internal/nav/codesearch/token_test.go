package codesearch

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/angeles725/niagara-go/internal/index"
)

// tokenDB builds a token-index.db with the module-navigator schema and opens
// it read-only, as the commands do.
func tokenDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token-index.db")
	w, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"CREATE TABLE files (id INTEGER PRIMARY KEY, path TEXT NOT NULL, module TEXT NOT NULL, class_name TEXT NOT NULL)",
		"CREATE TABLE postings (token TEXT NOT NULL, file_id INTEGER NOT NULL, line_no INTEGER NOT NULL)",
		"CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"CREATE INDEX idx_token ON postings(token)",
		"INSERT INTO meta VALUES ('timestamp', 't0'), ('source_root', '/x/organized')",
		`INSERT INTO files VALUES (1, 'b/b-rt/B.java', 'b-rt', 'B'), (2, 'a/a-rt/Z.java', 'a-rt', 'Z'),
			(3, 'a/a-rt/A.java', 'a-rt', 'A'), (4, 'c/c-rt/C.java', 'c-rt', 'C'), (5, 'a/a-rt/p/A.java', 'a-rt', 'A')`,
		`INSERT INTO postings VALUES ('ack', 1, 4), ('ack', 2, 9), ('ack', 3, 7), ('ack', 3, 2),
			('ack', 4, 1), ('ack', 4, 3), ('Ack', 1, 1), ('ackAll', 2, 1), ('nack', 2, 2), ('ack', 5, 5)`,
	} {
		if _, err := w.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	w.Close()
	db, err := index.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func rows(hits []TokenHit) []string {
	var out []string
	for _, h := range hits {
		out = append(out, FormatTokenRow(h))
	}
	return out
}

func TestTokensOrderAndModuleFilter(t *testing.T) {
	db := tokenDB(t)
	all, err := Tokens(db, "ack", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A:2:a-rt", "A:5:a-rt", "A:7:a-rt", "Z:9:a-rt", "B:4:b-rt", "C:1:c-rt", "C:3:c-rt"}
	var got []string
	for _, h := range all {
		got = append(got, h.Class+":"+strconv.FormatInt(h.Line, 10)+":"+h.Module)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Tokens = %q, want %q (module, class, line; exact case)", got, want)
	}
	inB, err := Tokens(db, "ack", "b-rt")
	if err != nil || len(inB) != 1 || inB[0].Path != "b/b-rt/B.java" {
		t.Errorf("module filter: %+v %v", inB, err)
	}
	if n, err := CountToken(db, "ack"); err != nil || n != 7 {
		t.Errorf("CountToken = %d %v", n, err)
	}
	similar, err := SimilarTokens(db, "ACK")
	if err != nil || !reflect.DeepEqual(similar, []string{"Ack", "ack", "ackAll", "nack"}) {
		t.Errorf("SimilarTokens = %q %v (distinct, any case, index order)", similar, err)
	}
	meta, err := TokenMeta(db)
	if err != nil || !reflect.DeepEqual(meta.Keys(), []string{"timestamp", "source_root"}) {
		t.Errorf("TokenMeta keys %v %v", meta.Keys(), err)
	}
}

func TestTokenSummaryAndFormatters(t *testing.T) {
	db := tokenDB(t)
	hits, err := Tokens(db, "ack", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatTokenHeader("ack", "", hits); got != "  TOKEN: 'ack' (7 occurrences in 4 classes, 3 modules)" {
		t.Errorf("header %q (classes are distinct class+module pairs)", got)
	}
	if got := FormatTokenHeader("ack", "a-rt", hits[:4]); got != "  TOKEN: 'ack' in a-rt (4 occurrences in 2 classes, 1 modules)" {
		t.Errorf("module header %q", got)
	}
	mods := ByModule(hits)
	var names []string
	for _, m := range mods {
		names = append(names, m.Module+"/"+strconv.Itoa(len(m.Hits))+"/"+strconv.Itoa(m.Classes))
	}
	if !reflect.DeepEqual(names, []string{"a-rt/4/2", "b-rt/1/1", "c-rt/2/1"}) {
		t.Errorf("ByModule %q", names)
	}
	var top []string
	for _, m := range TopModules(mods, 2) {
		top = append(top, FormatModuleHits(m))
	}
	want := []string{
		"    a-rt                                     4 hits,    2 classes",
		"    c-rt                                     2 hits,    1 classes",
	}
	if !reflect.DeepEqual(top, want) {
		t.Errorf("TopModules %q", top)
	}
	ties := TopModules([]ModuleHits{{Module: "x", Hits: hits[:1]}, {Module: "y", Hits: hits[:2]}, {Module: "z", Hits: hits[:1]}}, 10)
	if ties[0].Module != "y" || ties[1].Module != "x" || ties[2].Module != "z" {
		t.Errorf("ties keep name order: %+v", ties)
	}
	head := TokenTableHeader()
	if head[0] != "  CLASS                                 LINE  MODULE" || head[1] != "  "+strings.Repeat("-", 65) {
		t.Errorf("table header %q", head)
	}
	if got := rows([]TokenHit{{Class: strings.Repeat("é", 36), Line: 12345678, Module: "m"}}); got[0] != "  "+strings.Repeat("é", 32)+"...  12345678  m" {
		t.Errorf("row %q (class cut to 35 code points, wide line kept)", got[0])
	}
}
