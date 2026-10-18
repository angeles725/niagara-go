package navbuild

import (
	"bufio"
	"database/sql"
	"fmt"
	"iter"
	"net/url"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestTokenizeLine(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{"public class BFoo extends BComponent {\n", []string{"BFoo", "BComponent"}},
		// one-character runs, stop words and decompiler variables are dropped
		{"int a = var12 + var + var1x + x2;", []string{"var", "var1x", "x2"}},
		// non-ASCII characters split like any punctuation (RE_TOKEN_SPLIT is ASCII)
		{"String café = \"ñandú_x\";", []string{"String", "caf", "and", "_x"}},
		{"__ 12 0x1F", []string{"__", "12", "0x1F"}},
		{"", nil},
	} {
		if got := tokenizeLine(tc.line); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("tokenizeLine(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
	if n := len(tokenStopWords); n != 60 {
		t.Errorf("%d stop words, STOP_WORDS has 60", n)
	}
}

func TestTokenizeFileLinesAndMinLen(t *testing.T) {
	path := t.TempDir() + "/A.java"
	// CR and CRLF are line ends (universal newlines); the same token twice on
	// a line is one posting; the last line has no newline.
	mustWrite(t, path, "foo foo bar\r\nab\rfoo;\n\nbaz")
	tf := tokenizeFile(path, 3)
	want := []posting{{"foo", 1}, {"bar", 1}, {"foo", 3}, {"baz", 5}}
	if !tf.readable || tf.lines != 5 || !tf.hasTokens || !reflect.DeepEqual(tf.postings, want) {
		t.Errorf("tokenizeFile = %+v, want 5 lines and %v", tf, want)
	}
	if tf := tokenizeFile(path+".missing", 2); tf.readable {
		t.Errorf("a missing file is readable: %+v", tf)
	}
}

// tokenDumpQueries are the canonical sections of a token index: SQLite files
// are never byte-identical, so two indexes are equal when these records are.
var tokenDumpQueries = []struct{ name, query string }{
	{"schema", `SELECT type, name, tbl_name, quote(sql) FROM sqlite_master ORDER BY type, name`},
	{"meta", `SELECT key, quote(value) FROM meta ORDER BY rowid`},
	// ANALYZE's statistics (both SQLite builds have STAT4), which steer the readers' query plans.
	{"stat1", `SELECT tbl, quote(idx), quote(stat) FROM sqlite_stat1 ORDER BY tbl, idx`},
	{"stat4", `SELECT tbl, idx, neq, nlt, ndlt, hex(sample) FROM sqlite_stat4 ORDER BY tbl, idx, sample`},
	{"files", `SELECT quote(id), quote(path), quote(module), quote(class_name) FROM files ORDER BY id`},
	{"postings", `SELECT quote(token), quote(file_id), quote(line_no) FROM postings ORDER BY rowid`},
}

// volatileTokenMeta are the meta keys that differ between two builds.
var volatileTokenMeta = map[string]bool{"timestamp": true, "build_time_sec": true, "source_root": true}

// recordEscaper keeps a record on one line of a golden.
var recordEscaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`)

// tokenDump yields the canonical records of the token index at path, the
// volatile meta values blanked; a failure is a final "error" record.
func tokenDump(path string) iter.Seq[string] {
	return func(yield func(string) bool) {
		dsn := (&url.URL{Scheme: "file", Opaque: url.PathEscape(path), RawQuery: "mode=ro"}).String()
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			yield("error\t" + err.Error())
			return
		}
		defer db.Close()
		for _, q := range tokenDumpQueries {
			rows, err := db.Query(q.query)
			if err != nil {
				yield("error\t" + q.name + ": " + err.Error())
				return
			}
			cols, _ := rows.Columns()
			vals := make([]sql.NullString, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			for rows.Next() {
				if err := rows.Scan(ptrs...); err != nil {
					rows.Close()
					yield("error\t" + err.Error())
					return
				}
				rec := []string{q.name}
				for i, v := range vals {
					s := v.String
					if q.name == "meta" && i == 1 && volatileTokenMeta[vals[0].String] {
						s = "<VOLATILE>"
					}
					rec = append(rec, recordEscaper.Replace(s))
				}
				if !yield(strings.Join(rec, "\t")) {
					rows.Close()
					return
				}
			}
			if err := rows.Err(); err != nil {
				yield("error\t" + err.Error())
			}
			rows.Close()
		}
	}
}

// sameTokenDB compares two token indexes record by record (both streams are
// read in step, so a full-corpus index is never held in memory). With -update
// on the synthetic corpus it writes the Python dump as the golden.
func sameTokenDB(t *testing.T, pyPath, goPath string) {
	t.Helper()
	if *update && strings.HasSuffix(t.Name(), "/synthetic") {
		writeGoldenLines(t, goldenPath(TokenFile), tokenDump(pyPath))
	}
	if n, diff := diffSeqs(tokenDump(pyPath), tokenDump(goPath)); diff != "" {
		t.Errorf("%s differs at record %d: %s", TokenFile, n, diff)
	} else {
		t.Logf("%s: %d records identical", TokenFile, n)
	}
}

// diffSeqs walks two record streams in step: the number of records compared
// and, at the first difference, both records.
func diffSeqs(want, got iter.Seq[string]) (int, string) {
	next, stop := iter.Pull(got)
	defer stop()
	n := 0
	for w := range want {
		n++
		g, ok := next()
		// A failed dump is a difference even when both sides fail alike.
		if strings.HasPrefix(w, "error\t") || strings.HasPrefix(g, "error\t") {
			return n, fmt.Sprintf("dump failed: python: %.300q, go: %.300q", w, g)
		}
		if !ok {
			return n, fmt.Sprintf("python: %.300q, go: <end>", w)
		}
		if g != w {
			return n, fmt.Sprintf("python: %.300q, go: %.300q", w, g)
		}
	}
	if g, ok := next(); ok {
		return n + 1, fmt.Sprintf("python: <end>, go: %.300q", g)
	}
	if n == 0 {
		return 0, "no records on either side"
	}
	return n, ""
}

func writeGoldenLines(t *testing.T, path string, recs iter.Seq[string]) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := bufio.NewWriter(f)
	for r := range recs {
		w.WriteString(r + "\n")
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// goldenLines yields the records of a golden written by writeGoldenLines.
func goldenLines(t *testing.T, path string) iter.Seq[string] {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return func(yield func(string) bool) {
		for _, l := range strings.SplitAfter(string(data), "\n") {
			if l != "" && !yield(strings.TrimSuffix(l, "\n")) {
				return
			}
		}
	}
}

func TestDiffSeqsFailsOnDumpErrors(t *testing.T) {
	seq := func(recs ...string) iter.Seq[string] { return slices.Values(recs) }
	for _, tc := range []struct {
		name      string
		want, got iter.Seq[string]
		differ    bool
	}{
		{"equal", seq("a", "b"), seq("a", "b"), false},
		{"the same error on both sides", seq("error\tno such table"), seq("error\tno such table"), true},
		{"nothing at all", seq(), seq(), true},
		{"go is shorter", seq("a", "b"), seq("a"), true},
		{"go is longer", seq("a"), seq("a", "b"), true},
	} {
		if _, diff := diffSeqs(tc.want, tc.got); (diff != "") != tc.differ {
			t.Errorf("%s: diff %q", tc.name, diff)
		}
	}
}

func TestBuildTokenIndexFailures(t *testing.T) {
	organized := t.TempDir()
	mustWrite(t, organized+"/m/A.java", "class A {}\n")
	mustWrite(t, organized+"/m/Empty.java", "")
	for _, tc := range []struct {
		name, classes string
		fails         bool
	}{
		// Python inserts no files row for an empty file, so its null module never fails.
		{"null module of an empty file", `{"Empty": [{"path": "m/Empty.java", "module": null}], "A": [{"path": "m/A.java", "module": "m"}]}`, false},
		{"null module of a scanned file", `{"A": [{"path": "m/A.java", "module": null}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := newBase(t)
			mustWrite(t, IndexPath(base, ClassIndexFile), `{"classes": `+tc.classes+`}`)
			// a stale temp database of a killed build is replaced, never reused
			mustWrite(t, IndexPath(base, TokenFile)+".tmp", "not a database")
			_, err := BuildTokenIndex(base, organized, DefaultMinTokenLen)
			if (err != nil) != tc.fails {
				t.Fatalf("err = %v, want failure %v", err, tc.fails)
			}
			if isFile(IndexPath(base, TokenFile) + ".tmp") {
				t.Error("the temp database is left behind")
			}
			if isFile(IndexPath(base, TokenFile)) == tc.fails {
				t.Errorf("index written = %v after err %v", !tc.fails, err)
			}
		})
	}
}
