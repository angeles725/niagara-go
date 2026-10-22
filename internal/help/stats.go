package help

// Corpus overview (stats.py): index entry counts, text directory file
// counts and sizes and raw HTML counts. call-index.json is retired (I3-5):
// it is neither counted nor listed, even when a stale one is on disk.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/help/scope"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// fileCount is stats._count_files: files ending in ext under dir (as
// os.walk lists them) and their total size; a file whose size cannot be
// read counts with no size.
type fileCount struct{ files, bytes int }

func countFiles(dir, ext string) fileCount {
	var c fileCount
	if !pycompat.IsDir(dir) {
		return c
	}
	fts.WalkFiles(dir, func(path string) {
		if strings.HasSuffix(filepath.Base(path), ext) {
			c.files++
			if st, err := os.Stat(path); err == nil {
				c.bytes += int(st.Size())
			}
		}
	})
	return c
}

// mb is "{:.1f}".format(n / 1024 / 1024).
func mb(n int) string { return strconv.FormatFloat(float64(n)/1024/1024, 'f', 1, 64) }

// pyLen is len(v) for a decoded JSON value (0 for a scalar Python would
// reject).
func pyLen(v any) int {
	switch x := v.(type) {
	case []any:
		return len(x)
	case *index.Object:
		return x.Len()
	case string:
		return runeLen(x)
	}
	return 0
}

// statsIndexNames are the indexes stats counts (all required), in its order.
var statsIndexNames = []string{"class-index", "devguide-toc", "guides-index", "inheritance",
	"pdf-index", "method-index", "source-index", "xref-index", "slots-index"}

// statsTextDirs are the text directories stats counts, in its order.
var statsTextDirs = []string{"bajadoc-clean", "devguide-clean", "guides-clean", "docs-text", "source"}

// corpusStats is what cmd_stats prints: the loaded indexes, the text directory counts and
// the totals computed from them.
type corpusStats struct {
	idx                             map[string]*index.Object
	text                            map[string]fileCount
	props, actions, topics, methods int
	edges, lines, dgItems, packages int
	abs, vid                        string // the absolute data root, its version ("" unregistered)
}

// n is len(index): the entries of the index name.
func (s *corpusStats) n(name string) int { return s.idx[name].Len() }

// Stats is cmd_stats.
func (e *Env) Stats(asJSON bool) error {
	s, err := e.corpusStats()
	if err != nil {
		return err
	}
	if asJSON {
		e.statsJSON(s)
	} else {
		e.statsText(s)
	}
	return nil
}

// corpusStats loads the indexes (the first missing one is the error), counts the text
// directories and sums the index entries stats reports.
func (e *Env) corpusStats() (*corpusStats, error) {
	s := &corpusStats{idx: map[string]*index.Object{}, text: map[string]fileCount{}}
	for _, n := range statsIndexNames {
		o, err := e.loadIndex(n+".json", true)
		if err != nil {
			return nil, err
		}
		s.idx[n] = o
	}
	for _, d := range statsTextDirs {
		ext := ".txt"
		if d == "source" {
			ext = ".java"
		}
		s.text[d] = countFiles(filepath.Join(e.BaseDir, d), ext)
	}
	sum := func(name string, f func(o *index.Object, k string) int) int {
		n, o := 0, s.idx[name]
		for _, k := range o.Keys() {
			n += f(o, k)
		}
		return n
	}
	listOf := func(key string) func(*index.Object, string) int {
		return func(o *index.Object, k string) int { return listLen(o.Object(k), key) }
	}
	s.props, s.actions, s.topics = sum("slots-index", listOf("properties")), sum("slots-index", listOf("actions")), sum("slots-index", listOf("topics"))
	s.methods = sum("method-index", func(o *index.Object, k string) int { v, _ := o.Get(k); return pyLen(v) })
	s.edges = sum("xref-index", listOf("uses"))
	s.lines = sum("source-index", func(o *index.Object, k string) int {
		n, _ := index.Int(get(o.Object(k), "lines", nil))
		return int(n)
	})
	for _, sec := range objList(s.idx["devguide-toc"], "sections") {
		s.dgItems += listLen(sec, "items")
	}
	packages := map[string]bool{}
	for _, k := range s.idx["class-index"].Keys() {
		if p := truthyStr(s.idx["class-index"].Object(k), "package"); p != "" {
			packages[p] = true
		}
	}
	s.packages = len(packages)
	s.abs, _ = filepath.Abs(e.BaseDir)
	s.vid = scope.Identify(e.BaseDir)
	return s, nil
}

// statsJSON is stats --json: niagara_version is null for an unregistered data root.
func (e *Env) statsJSON(s *corpusStats) {
	var version any
	if s.vid != "" {
		version = s.vid
	}
	dirs := pyjson.Obj{}
	for _, d := range statsTextDirs {
		dirs = append(dirs, pyjson.KV{d, pyjson.Obj{{"files", s.text[d].files}, {"bytes", s.text[d].bytes}}})
	}
	data := pyjson.Obj{
		{"niagara_version", version},
		{"base_dir", s.abs},
		{"indexes", pyjson.Obj{
			{"class-index", s.n("class-index")},
			{"devguide-toc", s.dgItems},
			{"guides-index", s.n("guides-index")},
			{"inheritance", s.n("inheritance")},
			{"pdf-index", s.n("pdf-index")},
			{"method-index", pyjson.Obj{{"names", s.n("method-index")}, {"entries", s.methods}}},
			{"source-index", pyjson.Obj{{"classes", s.n("source-index")}, {"lines", s.lines}}},
			{"xref-index", pyjson.Obj{{"classes", s.n("xref-index")}, {"edges", s.edges}}},
			{"slots-index", pyjson.Obj{{"classes", s.n("slots-index")}, {"properties", s.props},
				{"actions", s.actions}, {"topics", s.topics}}},
		}},
		{"text_dirs", dirs},
		{"packages", s.packages},
	}
	fmt.Fprintln(e.Out, pyjson.DumpsAny(data, pyjson.Indent2))
}

// statsText is the stats report; the raw HTML directories are counted as it prints them.
func (e *Env) statsText(s *corpusStats) {
	w := func(n int) string { return pycompat.RJust(pycompat.Commas(n), 6, ' ') }
	vid := s.vid
	if vid == "" {
		vid = "unregistered"
	}
	totalBytes := 0
	for _, d := range statsTextDirs {
		totalBytes += s.text[d].bytes
	}
	indexCount := 9
	out := e.Out
	fmt.Fprintf(out, "Niagara Help Navigator - Corpus Statistics\n%s\n", strings.Repeat("=", 55))
	fmt.Fprintf(out, "Niagara version: %s  (%s)\n", vid, s.abs)
	fmt.Fprintf(out, "\nINDEXES (%d):\n\n", indexCount)
	fmt.Fprintf(out, "  class-index.json       %s classes (%d packages)\n", w(s.n("class-index")), s.packages)
	fmt.Fprintf(out, "  devguide-toc.json      %s guides\n", w(s.dgItems))
	fmt.Fprintf(out, "  guides-index.json      %s folders\n", w(s.n("guides-index")))
	fmt.Fprintf(out, "  inheritance.json       %s entries\n", w(s.n("inheritance")))
	fmt.Fprintf(out, "  pdf-index.json         %s PDFs\n", w(s.n("pdf-index")))
	fmt.Fprintf(out, "  method-index.json      %s names (%s entries)\n", w(s.n("method-index")), pycompat.Commas(s.methods))
	fmt.Fprintf(out, "  source-index.json      %s classes (%s lines)\n", w(s.n("source-index")), pycompat.Commas(s.lines))
	fmt.Fprintf(out, "  xref-index.json        %s classes (%s edges)\n", w(s.n("xref-index")), pycompat.Commas(s.edges))
	fmt.Fprintf(out, "  slots-index.json       %s classes (%s props, %d actions, %d topics)\n",
		w(s.n("slots-index")), pycompat.Commas(s.props), s.actions, s.topics)
	fmt.Fprintf(out, "\nTEXT SOURCES (%s MB total):\n\n", mb(totalBytes))
	for _, d := range statsTextDirs {
		fmt.Fprintf(out, "  %s %s files  (%s MB)\n", pycompat.LJust(d+"/", 17, ' '), w(s.text[d].files), mb(s.text[d].bytes))
	}
	fmt.Fprint(out, "\nRAW HTML:\n\n")
	for _, d := range []string{"bajadoc", "devguide", "guides"} {
		fmt.Fprintf(out, "  %s %s HTML\n", pycompat.LJust(d+"/", 17, ' '), w(countFiles(filepath.Join(e.BaseDir, d), ".html").files))
	}
	fmt.Fprintf(out, "\nINDEXES: %d\nCLI: 53 commands + REPL\n", indexCount)
}
