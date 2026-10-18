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
	pyWalkFiles(dir, func(path string) {
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

// Stats is cmd_stats.
func (e *Env) Stats(asJSON bool) error {
	idx := map[string]*index.Object{}
	for _, n := range []string{"class-index", "devguide-toc", "guides-index", "inheritance",
		"pdf-index", "method-index", "source-index", "xref-index", "slots-index"} {
		o, err := e.loadIndex(n+".json", true)
		if err != nil {
			return err
		}
		idx[n] = o
	}
	text := map[string]fileCount{}
	textDirs := []string{"bajadoc-clean", "devguide-clean", "guides-clean", "docs-text", "source"}
	for _, d := range textDirs {
		ext := ".txt"
		if d == "source" {
			ext = ".java"
		}
		text[d] = countFiles(filepath.Join(e.BaseDir, d), ext)
	}
	sum := func(name string, f func(o *index.Object, k string) int) int {
		n, o := 0, idx[name]
		for _, k := range o.Keys() {
			n += f(o, k)
		}
		return n
	}
	listOf := func(key string) func(*index.Object, string) int {
		return func(o *index.Object, k string) int { return listLen(o.Object(k), key) }
	}
	props, actions, topics := sum("slots-index", listOf("properties")), sum("slots-index", listOf("actions")), sum("slots-index", listOf("topics"))
	methods := sum("method-index", func(o *index.Object, k string) int { v, _ := o.Get(k); return pyLen(v) })
	edges := sum("xref-index", listOf("uses"))
	lines := sum("source-index", func(o *index.Object, k string) int {
		n, _ := index.Int(get(o.Object(k), "lines", nil))
		return int(n)
	})
	dgItems := 0
	for _, s := range objList(idx["devguide-toc"], "sections") {
		dgItems += listLen(s, "items")
	}
	packages := map[string]bool{}
	for _, k := range idx["class-index"].Keys() {
		if p := truthyStr(idx["class-index"].Object(k), "package"); p != "" {
			packages[p] = true
		}
	}
	abs, _ := filepath.Abs(e.BaseDir)
	vid := scope.Identify(e.BaseDir)
	n := func(name string) int { return idx[name].Len() }
	if asJSON {
		var version any
		if vid != "" {
			version = vid
		}
		dirs := pyjson.Obj{}
		for _, d := range textDirs {
			dirs = append(dirs, pyjson.KV{d, pyjson.Obj{{"files", text[d].files}, {"bytes", text[d].bytes}}})
		}
		data := pyjson.Obj{
			{"niagara_version", version},
			{"base_dir", abs},
			{"indexes", pyjson.Obj{
				{"class-index", n("class-index")},
				{"devguide-toc", dgItems},
				{"guides-index", n("guides-index")},
				{"inheritance", n("inheritance")},
				{"pdf-index", n("pdf-index")},
				{"method-index", pyjson.Obj{{"names", n("method-index")}, {"entries", methods}}},
				{"source-index", pyjson.Obj{{"classes", n("source-index")}, {"lines", lines}}},
				{"xref-index", pyjson.Obj{{"classes", n("xref-index")}, {"edges", edges}}},
				{"slots-index", pyjson.Obj{{"classes", n("slots-index")}, {"properties", props},
					{"actions", actions}, {"topics", topics}}},
			}},
			{"text_dirs", dirs},
			{"packages", len(packages)},
		}
		fmt.Fprintln(e.Out, pyjson.DumpsAny(data, pyjson.Indent2))
		return nil
	}
	w := func(n int) string { return pycompat.RJust(pycompat.Commas(n), 6, ' ') }
	if vid == "" {
		vid = "unregistered"
	}
	totalBytes := 0
	for _, d := range textDirs {
		totalBytes += text[d].bytes
	}
	indexCount := 9
	out := e.Out
	fmt.Fprintf(out, "Niagara Help Navigator - Corpus Statistics\n%s\n", strings.Repeat("=", 55))
	fmt.Fprintf(out, "Niagara version: %s  (%s)\n", vid, abs)
	fmt.Fprintf(out, "\nINDEXES (%d):\n\n", indexCount)
	fmt.Fprintf(out, "  class-index.json       %s classes (%d packages)\n", w(n("class-index")), len(packages))
	fmt.Fprintf(out, "  devguide-toc.json      %s guides\n", w(dgItems))
	fmt.Fprintf(out, "  guides-index.json      %s folders\n", w(n("guides-index")))
	fmt.Fprintf(out, "  inheritance.json       %s entries\n", w(n("inheritance")))
	fmt.Fprintf(out, "  pdf-index.json         %s PDFs\n", w(n("pdf-index")))
	fmt.Fprintf(out, "  method-index.json      %s names (%s entries)\n", w(n("method-index")), pycompat.Commas(methods))
	fmt.Fprintf(out, "  source-index.json      %s classes (%s lines)\n", w(n("source-index")), pycompat.Commas(lines))
	fmt.Fprintf(out, "  xref-index.json        %s classes (%s edges)\n", w(n("xref-index")), pycompat.Commas(edges))
	fmt.Fprintf(out, "  slots-index.json       %s classes (%s props, %d actions, %d topics)\n",
		w(n("slots-index")), pycompat.Commas(props), actions, topics)
	fmt.Fprintf(out, "\nTEXT SOURCES (%s MB total):\n\n", mb(totalBytes))
	for _, d := range textDirs {
		fmt.Fprintf(out, "  %s %s files  (%s MB)\n", pycompat.LJust(d+"/", 17, ' '), w(text[d].files), mb(text[d].bytes))
	}
	fmt.Fprint(out, "\nRAW HTML:\n\n")
	for _, d := range []string{"bajadoc", "devguide", "guides"} {
		fmt.Fprintf(out, "  %s %s HTML\n", pycompat.LJust(d+"/", 17, ' '), w(countFiles(filepath.Join(e.BaseDir, d), ".html").files))
	}
	fmt.Fprintf(out, "\nINDEXES: %d\nCLI: 53 commands + REPL\n", indexCount)
	return nil
}
