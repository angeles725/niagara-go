package inventory

import (
	"database/sql"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/lib/modlist"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// stats (inventory/stats.py cmd_stats): corpus and index summary, the sizes
// and _meta counters of every index, the Help Navigator status and the
// commands the binary runs. Index summaries come from statsCache, so a run
// after the first one reads only the module inventory.

// indexFile is one entry of the INDEXES list.
type indexFile struct {
	name string
	size int64
}

// roundMB is round(size / (1024 * 1024), 1).
func roundMB(size int64) float64 {
	f, _ := strconv.ParseFloat(core.Fixed(float64(size)/(1024*1024), 1), 64)
	return f
}

// metaGet is meta.get(key, 0) as a decoded value (meta may be nil).
func metaGet(meta *index.Object, key string) any {
	if meta != nil {
		if v, ok := meta.Get(key); ok {
			return v
		}
	}
	return int64(0)
}

// metaStr is meta.get(key, "").
func metaStr(meta *index.Object, key string) any {
	if meta != nil {
		if v, ok := meta.Get(key); ok {
			return v
		}
	}
	return ""
}

// pyValue is a decoded JSON value as a Go value pycompat.Format takes.
func pyValue(v any) any {
	switch t := v.(type) {
	case int64, float64, string, bool, nil:
		return t
	case int:
		return int64(t)
	}
	if n, ok := index.Int(v); ok {
		return n
	}
	if s := fmt.Sprint(v); len(s) > 0 && (s[0] == '-' || (s[0] >= '0' && s[0] <= '9')) {
		if b, ok := new(big.Int).SetString(s, 10); ok {
			return b
		}
	}
	if f, ok := index.Float(v); ok {
		return f
	}
	return core.PyStr(v)
}

// format is "{:spec}".format(v).
func format(v any, spec string) string {
	s, err := pycompat.Format(pyValue(v), spec)
	if err != nil {
		return core.PyStr(v)
	}
	return s
}

// dbMeta is _db_meta: the meta table of indexes/<name> (nil when absent or
// unreadable); a readable one joins files with its size.
func dbMeta(dir, name string, files *[]indexFile) map[string]any {
	path := filepath.Join(dir, name)
	if st, err := os.Stat(path); err != nil || !st.Mode().IsRegular() {
		return nil
	}
	db, err := index.OpenSQLite(path)
	if err != nil {
		return nil
	}
	defer db.Close()
	meta, err := readMeta(db)
	if err != nil {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	*files = append(*files, indexFile{name, st.Size()})
	return meta
}

func readMeta(db *sql.DB) (map[string]any, error) {
	rows, err := db.Query("SELECT key, value FROM meta")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	meta := map[string]any{}
	for rows.Next() {
		var k string
		var v any
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		meta[k] = v
	}
	return meta, rows.Err()
}

// dbInt is int(meta.get(key, 0)).
func dbInt(meta map[string]any, key string) int64 {
	switch v := meta[key].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case []byte:
		n, _ := strconv.ParseInt(pycompat.Strip(string(v)), 10, 64)
		return n
	case string:
		n, _ := strconv.ParseInt(pycompat.Strip(v), 10, 64)
		return n
	}
	return 0
}

// section is one "<name>: {...}" block of the stats JSON whose keys read
// their values from an index _meta; it is {} when the _meta is empty and the
// block is conditional.
type section struct {
	name        string
	meta        *index.Object
	conditional bool
	fields      [][3]string // stats key, _meta key, default ("" = 0, "s" = "")
}

func (s section) object() *index.Object {
	o := index.NewObject()
	if s.conditional && (s.meta == nil || s.meta.Len() == 0) {
		return o
	}
	for _, f := range s.fields {
		if f[2] == "s" {
			o.Set(f[0], metaStr(s.meta, f[1]))
		} else {
			o.Set(f[0], metaGet(s.meta, f[1]))
		}
	}
	return o
}

// statsReport is what stats gathers: the stats JSON object and what the text
// view reads beside it.
type statsReport struct {
	stats      *index.Object
	ci         summary // class-index.json, decoded whole
	files      []indexFile
	totalBytes int64
	present    map[string]bool // section name -> its index _meta is non-empty
}

func runStats(env *registry.Env, ns *argparse.Namespace) int {
	reader := openStatsCache(filepath.Join(env.BaseDir, "indexes"))
	r := gatherStats(env, reader)
	defer reader.save()
	if ns.Bool("json") {
		fmt.Fprintln(env.Out, core.Dumps(r.stats, 2))
		return env.ExitCode()
	}
	printStats(env, r, reader)
	return env.ExitCode()
}

// gatherStats builds the stats object: corpus counters from the module
// inventory, then every index's _meta counters, then the index sizes.
func gatherStats(env *registry.Env, reader *statsCache) statsReport {
	dir := filepath.Join(env.BaseDir, "indexes")
	files := jsonIndexFiles(dir)
	var inv *index.Object
	if data := loadInventory(env); data != nil && data.Len() > 0 {
		inv = data.Object("modules")
	}
	corpus := corpusStats(inv)

	ci := reader.summary(filepath.Join(dir, "class-index.json"), true)
	sections := indexSections(reader, dir)
	tk := dbMeta(dir, "token-index.db", &files)
	str := dbMeta(dir, "string-index.db", &files)
	var totalBytes int64
	for _, f := range files {
		totalBytes += f.size
	}

	stats := index.NewObject()
	stats.Set("corpus", corpus)
	stats.Set("class_index", classIndexStats(ci))
	present := map[string]bool{}
	for _, s := range sections {
		stats.Set(s.name, s.object())
		present[s.name] = s.meta != nil && s.meta.Len() > 0
	}
	stats.Set("token_index", dbSection(tk, "total_postings", "unique_tokens", "files_with_tokens", "total_lines"))
	stats.Set("string_index", dbSection(str, "total_strings", "unique_strings", "files_with_strings"))
	stats.Set("indexes", indexList(files))
	stats.Set("total_index_size_mb", roundMB(totalBytes))
	return statsReport{stats, ci, files, totalBytes, present}
}

// jsonIndexFiles is the *.json files of indexes/ in sorted order with their
// sizes (0 when unreadable); a missing directory gives none.
func jsonIndexFiles(dir string) []indexFile {
	var files []indexFile
	if names, err := pycompat.SortedListdir(dir); err == nil {
		for _, n := range names {
			if strings.HasSuffix(n, ".json") {
				var size int64
				if st, err := os.Stat(filepath.Join(dir, n)); err == nil {
					size = st.Size()
				}
				files = append(files, indexFile{n, size})
			}
		}
	}
	return files
}

// corpusStats is the "corpus" block: module counters over the inventory's
// modules object (inv may be nil).
func corpusStats(inv *index.Object) *index.Object {
	var mods []*index.Object
	if inv != nil {
		mods = core.Objects(inv.Values())
	}
	unique := map[string]bool{}
	types := index.NewObject()
	var withCode, java, classCount, zkm int64
	for _, v := range mods {
		unique[core.Str(v, "module")] = true
		if core.Truthy(v, "has_code") {
			withCode++
		}
		java += core.Int(v, "java_files")
		classCount += core.Int(v, "class_count")
		if core.Truthy(v, "zkm") {
			zkm++
		}
		t := modlist.TypeOrStandalone(v)
		n, _ := types.Get(t)
		c, _ := n.(int64)
		types.Set(t, c+1)
	}
	total := int64(len(mods))
	corpus := index.NewObject()
	for _, kv := range []struct {
		k string
		v any
	}{{"submodules", total}, {"unique_modules", int64(len(unique))}, {"with_code", withCode},
		{"without_code", total - withCode}, {"java_files", java}, {"class_files", classCount},
		{"zkm_obfuscated", zkm}, {"types", types}} {
		corpus.Set(kv.k, kv.v)
	}
	return corpus
}

// classIndexStats is the "class_index" block: the decoded class counts, then
// the class-index _meta counters.
func classIndexStats(ci summary) *index.Object {
	ciMeta := ci.meta()
	classIndex := index.NewObject()
	classIndex.Set("total_entries", ci.Entries)
	classIndex.Set("unique_names", ci.Classes)
	for _, f := range [][2]string{{"top_level", "top_level_classes"}, {"inner_classes", "inner_classes"},
		{"interfaces", "interfaces"}, {"enums", "enums"}, {"abstract_classes", "abstract_classes"},
		{"packages", "unique_packages"}} {
		classIndex.Set(f[0], metaGet(ciMeta, f[1]))
	}
	return classIndex
}

// indexSections is the _meta-backed blocks of the stats JSON, in output
// order; each index summary is read in that same order.
func indexSections(reader *statsCache, dir string) []section {
	meta := func(name string) *index.Object { return reader.summary(filepath.Join(dir, name), false).meta() }
	return []section{
		{"swing_index", meta("swing-index.json"), false, [][3]string{{"classes_with_ui", "classes_with_ui"}, {"dialogs", "total_dialogs"},
			{"dimensions", "total_dimensions"}, {"colors", "total_colors"}, {"fonts", "total_fonts"}, {"icons", "total_icons"}}},
		{"inheritance_index", meta("inheritance.json"), false, [][3]string{{"unique_parents", "unique_parents"},
			{"unique_interfaces", "unique_interfaces_implemented"}, {"classes_with_chains", "classes_with_chains"},
			{"max_chain_depth", "max_chain_depth"}, {"avg_chain_depth", "avg_chain_depth"},
			{"with_extends", "with_extends"}, {"with_implements", "with_implements"}}},
		{"xref_index", meta("xref-index.json"), false, [][3]string{{"classes_with_imports", "classes_with_imports"},
			{"classes_imported_by_others", "classes_imported_by_others"},
			{"total_import_statements", "total_import_statements"}, {"resolved_imports", "resolved_imports"},
			{"wildcard_imports", "wildcard_imports"}, {"modules_with_deps", "modules_with_deps"},
			{"modules_depended_on", "modules_depended_on"}, {"total_class_import_edges", "total_class_import_edges"},
			{"total_module_dep_edges", "total_module_dep_edges"}}},
		{"method_index", meta("method-index.json"), false, [][3]string{{"total_definitions", "total_method_definitions"},
			{"unique_names", "unique_method_names"}, {"constructors", "total_constructors"},
			{"classes_with_methods", "classes_with_methods"}, {"avg_methods_per_class", "avg_methods_per_class"},
			{"max_methods_per_class", "max_methods_per_class"}, {"max_methods_class", "max_methods_class", "s"}}},
		{"annotations_index", meta("annotations-index.json"), false, [][3]string{{"niagara_types", "total_niagara_types"},
			{"with_properties", "with_properties"}, {"with_actions", "with_actions"}, {"with_topics", "with_topics"},
			{"total_properties", "total_properties"}, {"total_actions", "total_actions"},
			{"total_topics", "total_topics"}, {"unique_property_types", "unique_property_types"},
			{"unique_action_names", "unique_action_names"}}},
		{"callgraph_index", meta("callgraph-index.json"), true, [][3]string{{"caller_methods", "total_caller_methods"},
			{"unique_callees", "total_unique_callees"}, {"total_edges", "total_edges"},
			{"avg_callees_per_caller", "avg_callees_per_caller"}}},
		{"field_index", meta("field-index.json"), true, [][3]string{{"total_definitions", "total_field_definitions"},
			{"unique_names", "unique_field_names"}, {"static_fields", "total_static"}, {"final_fields", "total_final"},
			{"constants", "total_constants"}, {"with_init", "total_with_init"},
			{"classes_with_fields", "classes_with_fields"}, {"avg_fields_per_class", "avg_fields_per_class"},
			{"max_fields_per_class", "max_fields_per_class"}, {"max_fields_class", "max_fields_class", "s"}}},
		{"exceptions_index", meta("exceptions-index.json"), true, [][3]string{{"throws_declarations", "total_throws_declarations"},
			{"catch_blocks", "total_catch_blocks"}, {"unique_thrown_types", "unique_thrown_types"},
			{"unique_caught_types", "unique_caught_types"}, {"classes_with_throws", "classes_with_throws"},
			{"classes_with_catches", "classes_with_catches"}}},
	}
}

// dbSection is a SQLite index block: int(meta.get(k, 0)) per key, {} when
// the index has no meta.
func dbSection(meta map[string]any, keys ...string) *index.Object {
	o := index.NewObject()
	if len(meta) > 0 {
		for _, k := range keys {
			o.Set(k, dbInt(meta, k))
		}
	}
	return o
}

// indexList is the "indexes" list: name, size_bytes and size_mb per file.
func indexList(files []indexFile) []any {
	list := make([]any, len(files))
	for i, f := range files {
		o := index.NewObject()
		o.Set("name", f.name)
		o.Set("size_bytes", f.size)
		o.Set("size_mb", roundMB(f.size))
		list[i] = o
	}
	return list
}

// readDirNames is os.listdir(dir).
func readDirNames(dir string) ([]string, error) { return pycompat.Listdir(dir) }
