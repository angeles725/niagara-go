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

// textSource is one source of the full-text index as stats reports it: the files the
// index reads (fts.IterFiles) and their size, absent when the source is not built, and
// the version it is borrowed from when this one ships none (docs-text, course-text).
type textSource struct {
	fileCount
	absent bool
	shared string // the fts.Shared note with the owner version, "" when not borrowed
}

// textSources counts every source of the full-text index, in fts.Labels order: the one
// source registry find, the fingerprint and the scope report read, so stats can never
// list fewer sources than the index holds.
func textSources(baseDir string) map[string]textSource {
	dirs := fts.SourceDirs(baseDir)
	out := map[string]textSource{}
	for _, label := range fts.Labels {
		dir := fts.DirOf(dirs, label)
		if dir == "" {
			out[label] = textSource{absent: true}
			continue
		}
		var ts textSource
		for _, path := range fts.IterFiles(label, dir) {
			ts.files++
			if st, err := os.Stat(path); err == nil {
				ts.bytes += int(st.Size())
			}
		}
		if note, ok := fts.Shared[label]; ok {
			if _, owner := fts.SharedRoot(baseDir, label); owner != "" {
				ts.shared = fmt.Sprintf(note, owner)
			}
		}
		out[label] = ts
	}
	return out
}

// corpusStats is what cmd_stats prints: the loaded indexes, the text directory counts and
// the totals computed from them.
type corpusStats struct {
	idx                             map[string]*index.Object
	text                            map[string]textSource
	props, actions, topics, methods int
	edges, lines, dgItems, packages int
	fromJars                        int            // source-index records extracted from module jars
	abs, vid                        string         // the absolute data root, its version ("" unregistered)
	mods                            *moduleCounts  // nil when module-index.json is absent
	pals                            *paletteCounts // nil when palette-index.json is absent
	tags                            pyjson.Obj     // nil when tag-index.json is absent
	lexicons                        pyjson.Obj     // nil when lexicon-index.json is absent
}

// tagStats counts tag-index.json, nil when it is absent: its distinct tags
// (dict:name), the records of its Haystack 4 defs files, the rows of every
// other section, the files read and those it could not read.
func (e *Env) tagStats() (pyjson.Obj, error) {
	if !pycompat.IsFile(index.Path(e.BaseDir, tagIndexName)) {
		return nil, nil
	}
	idx, err := e.loadIndex(tagIndexName, false)
	if err != nil {
		return nil, err
	}
	tags, defs := 0, 0
	for _, t := range tagEntries(idx) {
		if t.kind == "tag" {
			tags++
		}
	}
	for _, f := range objList(idx, "files") {
		if getStr(f, "kind", "") == "defs" {
			n, _ := index.Int(get(f, "records", nil))
			defs += int(n)
		}
	}
	c := pyjson.Obj{{"tags", tags}, {"defs", defs}}
	for _, sec := range []string{"protos", "groups", "relations", "rules", "migrations", "files"} {
		c = append(c, pyjson.KV{sec, listLen(idx, sec)})
	}
	return append(c, pyjson.KV{"unreadable", listLen(idx, "errors")}), nil
}

// paletteCounts are the palette-index.json totals stats reports.
type paletteCounts struct{ palettes, entries, unreadable int }

// paletteStats counts palette-index.json, nil when it is absent (it is built
// from the install, so a data root may lack it).
func (e *Env) paletteStats() (*paletteCounts, error) {
	if !pycompat.IsFile(index.Path(e.BaseDir, paletteIndexName)) {
		return nil, nil
	}
	idx, err := e.loadIndex(paletteIndexName, false)
	if err != nil {
		return nil, err
	}
	c := &paletteCounts{unreadable: listLenAny(get(idx, "errors", nil))}
	for _, p := range objList(idx, "palettes") {
		n, _ := index.Int(get(p, "entries", nil))
		c.palettes, c.entries = c.palettes+1, c.entries+int(n)
	}
	return c, nil
}

// moduleCounts are the module-index.json totals stats reports.
type moduleCounts struct{ modules, deps, types, agents, perms, unreadable int }

// moduleStats counts module-index.json, nil when it is absent (it is built from
// the install, so a data root may lack it).
func (e *Env) moduleStats() (*moduleCounts, error) {
	if !pycompat.IsFile(index.Path(e.BaseDir, moduleIndexName)) {
		return nil, nil
	}
	idx, err := e.loadIndex(moduleIndexName, false)
	if err != nil {
		return nil, err
	}
	c := &moduleCounts{unreadable: listLenAny(get(idx, "errors", nil))}
	if mods := idx.Object("modules"); mods != nil {
		for _, k := range mods.Keys() {
			r := mods.Object(k)
			c.modules++
			c.deps += listLen(r, "dependencies")
			c.types += listLen(r, "types")
			c.agents += listLen(r, "agents")
			c.perms += listLen(r, "java_permissions")
		}
	}
	return c, nil
}

func listLenAny(v any) int { l, _ := v.([]any); return len(l) }

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
	s := &corpusStats{idx: map[string]*index.Object{}}
	for _, n := range statsIndexNames {
		o, err := e.loadIndex(n+".json", true)
		if err != nil {
			return nil, err
		}
		s.idx[n] = o
	}
	s.text = textSources(e.BaseDir)
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
	s.fromJars = sum("source-index", func(o *index.Object, k string) int { // vendor source of module jars
		if fromModuleJar(o.Object(k)) {
			return 1
		}
		return 0
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
	var err error
	if s.mods, err = e.moduleStats(); err != nil {
		return nil, err
	}
	if s.pals, err = e.paletteStats(); err != nil {
		return nil, err
	}
	if s.tags, err = e.tagStats(); err != nil {
		return nil, err
	}
	s.lexicons, err = e.lexiconStats()
	return s, err
}

// statsJSON is stats --json: niagara_version is null for an unregistered data root.
func (e *Env) statsJSON(s *corpusStats) {
	var version any
	if s.vid != "" {
		version = s.vid
	}
	dirs := pyjson.Obj{}
	for _, d := range fts.Labels {
		ts := s.text[d]
		row := pyjson.Obj{{"files", ts.files}, {"bytes", ts.bytes}}
		if ts.absent {
			row = append(row, pyjson.KV{"absent", true})
		}
		if ts.shared != "" {
			row = append(row, pyjson.KV{"shared", ts.shared})
		}
		dirs = append(dirs, pyjson.KV{d, row})
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
			{"source-index", sourceStatsJSON(s)},
			{"xref-index", pyjson.Obj{{"classes", s.n("xref-index")}, {"edges", s.edges}}},
			{"slots-index", pyjson.Obj{{"classes", s.n("slots-index")}, {"properties", s.props},
				{"actions", s.actions}, {"topics", s.topics}}},
			{"module-index", s.moduleJSON()},
			{"palette-index", s.paletteJSON()},
			{"tag-index", orAbsent(s.tags)},
			{"lexicon-index", orAbsent(s.lexicons)},
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
	for _, d := range fts.Labels {
		totalBytes += s.text[d].bytes
	}
	indexCount := len(statsIndexNames) + 4 // and module-index.json, palette-index.json, tag-index.json, lexicon-index.json
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
	jars := ""
	if s.fromJars > 0 {
		jars = fmt.Sprintf(", %s from module jars", pycompat.Commas(s.fromJars))
	}
	fmt.Fprintf(out, "  source-index.json      %s classes (%s lines%s)\n", w(s.n("source-index")), pycompat.Commas(s.lines), jars)
	fmt.Fprintf(out, "  xref-index.json        %s classes (%s edges)\n", w(s.n("xref-index")), pycompat.Commas(s.edges))
	fmt.Fprintf(out, "  slots-index.json       %s classes (%s props, %d actions, %d topics)\n",
		w(s.n("slots-index")), pycompat.Commas(s.props), s.actions, s.topics)
	if m := s.mods; m == nil {
		fmt.Fprintln(out, "  module-index.json      absent (rebuild-all builds it from the install's modules/)")
	} else {
		fmt.Fprintf(out, "  module-index.json      %s modules (%s dependencies, %s types, %s agents, %s permissions; %d unreadable)\n",
			w(m.modules), pycompat.Commas(m.deps), pycompat.Commas(m.types), pycompat.Commas(m.agents), pycompat.Commas(m.perms), m.unreadable)
	}
	if p := s.pals; p == nil {
		fmt.Fprintln(out, "  palette-index.json     absent (rebuild-all builds it from the install's modules/)")
	} else {
		fmt.Fprintf(out, "  palette-index.json     %s palettes (%s entries; %d unreadable)\n", w(p.palettes), pycompat.Commas(p.entries), p.unreadable)
	}
	if c := s.tags; c == nil {
		fmt.Fprintln(out, "  tag-index.json         absent (rebuild-all builds it from the install's modules/)")
	} else {
		var parts []string // "N defs", ... in tagStats order, the files and unreadable counts last
		for _, kv := range c[1:] {
			parts = append(parts, pycompat.Commas(kv.Val.(int))+" "+kv.Key)
		}
		k := len(parts) - 2
		fmt.Fprintf(out, "  tag-index.json         %s tags (%s; %s)\n", w(c[0].Val.(int)), strings.Join(parts[:k], ", "), strings.Join(parts[k:], ", "))
	}
	if c := s.lexicons; c == nil {
		fmt.Fprintln(out, "  lexicon-index.json     absent (rebuild-all builds it from the install's modules/)")
	} else {
		fmt.Fprintf(out, "  lexicon-index.json     %s\n", lexiconStatsLine(c, w))
	}
	fmt.Fprintf(out, "\nTEXT SOURCES (%d sources of the full-text index, %s MB total):\n\n", len(fts.Labels), mb(totalBytes))
	for _, d := range fts.Labels {
		ts, name := s.text[d], pycompat.LJust(d+"/", 19, ' ')
		switch {
		case ts.absent:
			fmt.Fprintf(out, "  %s %s\n", name, pycompat.RJust("absent", 6, ' '))
		case ts.shared != "":
			fmt.Fprintf(out, "  %s %s files  (%s MB)  shared: %s\n", name, w(ts.files), mb(ts.bytes), ts.shared)
		default:
			fmt.Fprintf(out, "  %s %s files  (%s MB)\n", name, w(ts.files), mb(ts.bytes))
		}
	}
	fmt.Fprint(out, "\nRAW HTML:\n\n")
	for _, d := range []string{"bajadoc", "devguide", "guides"} {
		fmt.Fprintf(out, "  %s %s HTML\n", pycompat.LJust(d+"/", 19, ' '), w(countFiles(filepath.Join(e.BaseDir, d), ".html").files))
	}
	// The command table (order lists every registered command, in --help order),
	// not a fixed number: retired commands and the REPL exit 2 and are not counted.
	fmt.Fprintf(out, "\nINDEXES: %d\nCLI: %d commands\n", indexCount, len(order))
}

// moduleJSON is the module-index entry of stats --json: {"absent": true} when
// the index is not built.
func (s *corpusStats) moduleJSON() pyjson.Obj {
	m := s.mods
	if m == nil {
		return pyjson.Obj{{"absent", true}}
	}
	return pyjson.Obj{{"modules", m.modules}, {"dependencies", m.deps}, {"types", m.types}, {"agents", m.agents},
		{"java_permissions", m.perms}, {"unreadable", m.unreadable}}
}

// paletteJSON is the palette-index entry of stats --json: {"absent": true}
// when the index is not built.
func (s *corpusStats) paletteJSON() pyjson.Obj {
	if s.pals == nil {
		return pyjson.Obj{{"absent", true}}
	}
	return pyjson.Obj{{"palettes", s.pals.palettes}, {"entries", s.pals.entries}, {"unreadable", s.pals.unreadable}}
}

// orAbsent is an install-built index's entry of stats --json: {"absent": true}
// when the index is not built.
func orAbsent(c pyjson.Obj) pyjson.Obj {
	if c == nil {
		return pyjson.Obj{{"absent", true}}
	}
	return c
}

// sourceStatsJSON is the source-index record of stats --json; module_jar_classes
// (the records extracted from module jars) only when there are some, so a
// docSource-only corpus reports what it always did.
func sourceStatsJSON(s *corpusStats) pyjson.Obj {
	o := pyjson.Obj{{"classes", s.n("source-index")}, {"lines", s.lines}}
	if s.fromJars > 0 {
		o = append(o, pyjson.KV{"module_jar_classes", s.fromJars})
	}
	return o
}
