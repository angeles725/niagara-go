package inventory

import (
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// The text view of stats.

// printStats is the text view of cmd_stats.
func printStats(env *registry.Env, stats *index.Object, ci summary, files []indexFile, totalBytes int64,
	present map[string]bool, reader *statsCache) {
	w := env.Out
	obj := func(k string) *index.Object { return stats.Object(k) }
	line := func(label string, v any, spec string) { fmt.Fprintf(w, "    %s%s\n", label, format(v, spec)) }
	rule := strings.Repeat("=", 65)
	fmt.Fprintf(w, "\n%s\n  MODULE NAVIGATOR — CORPUS STATS\n%s\n\n", rule, rule)
	c := obj("corpus")
	fmt.Fprintln(w, "  CORPUS:")
	for _, f := range [][2]string{{"Submodules (JARs):      ", "submodules"}, {"Unique modules:         ", "unique_modules"},
		{"With Java code:         ", "with_code"}, {"Without code:           ", "without_code"},
		{"Java files:             ", "java_files"}, {"Class files:            ", "class_files"},
		{"ZKM obfuscated:         ", "zkm_obfuscated"}} {
		v, _ := c.Get(f[1])
		line(f[0], v, ">7,")
	}
	types := c.Object("types")
	keys := append([]string(nil), types.Keys()...)
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		v, _ := types.Get(k)
		parts[i] = fmt.Sprintf("%s=%v", k, v)
	}
	fmt.Fprintf(w, "\n    Types: %s\n\n", strings.Join(parts, ", "))

	if ci.Entries > 0 {
		fmt.Fprintln(w, "  CLASS INDEX:")
		line("Total entries:          ", ci.Entries, ">7,")
		line("Unique class names:     ", ci.Classes, ">7,")
		cx := obj("class_index")
		for _, f := range [][2]string{{"Top-level classes:      ", "top_level"}, {"Inner classes:          ", "inner_classes"},
			{"Interfaces:             ", "interfaces"}, {"Enums:                  ", "enums"},
			{"Abstract classes:       ", "abstract_classes"}, {"Unique packages:        ", "packages"}} {
			if v, _ := cx.Get(f[1]); index.Truthy(v) {
				line(f[0], v, ">7,")
			}
		}
		fmt.Fprintln(w)
	}
	block := func(title, key string, present bool, rows [][3]string) {
		if !present {
			return
		}
		fmt.Fprintf(w, "  %s:\n", title)
		o := obj(key)
		for _, r := range rows {
			v, _ := o.Get(r[1])
			line(r[0], v, r[2])
		}
		fmt.Fprintln(w)
	}
	nonEmpty := func(key string) bool { return present[key] }
	block("SWING INDEX", "swing_index", nonEmpty("swing_index"), [][3]string{
		{"Classes with UI:        ", "classes_with_ui", ">7,"}, {"Dialogs:                ", "dialogs", ">7,"},
		{"Hardcoded sizes:        ", "dimensions", ">7,"}, {"Hardcoded colors:       ", "colors", ">7,"},
		{"Hardcoded fonts:        ", "fonts", ">7,"}, {"Icons:                  ", "icons", ">7,"}})
	block("INHERITANCE INDEX", "inheritance_index", nonEmpty("inheritance_index"), [][3]string{
		{"With extends:           ", "with_extends", ">7,"}, {"With implements:        ", "with_implements", ">7,"},
		{"Unique parents:         ", "unique_parents", ">7,"}, {"Unique interfaces:      ", "unique_interfaces", ">7,"},
		{"Classes with chains:    ", "classes_with_chains", ">7,"}, {"Max chain depth:        ", "max_chain_depth", ">7"},
		{"Avg chain depth:        ", "avg_chain_depth", ">7.1f"}})
	block("XREF INDEX", "xref_index", nonEmpty("xref_index"), [][3]string{
		{"Classes with imports:    ", "classes_with_imports", ">7,"},
		{"Classes imported by:     ", "classes_imported_by_others", ">7,"},
		{"Import statements:       ", "total_import_statements", ">7,"},
		{"Resolved imports:        ", "resolved_imports", ">7,"}, {"Wildcard imports:        ", "wildcard_imports", ">7,"},
		{"Class import edges:      ", "total_class_import_edges", ">7,"},
		{"Modules with deps:       ", "modules_with_deps", ">7,"},
		{"Modules depended on:     ", "modules_depended_on", ">7,"},
		{"Module dep edges:        ", "total_module_dep_edges", ">7,"}})
	if nonEmpty("method_index") {
		m := obj("method_index")
		fmt.Fprintln(w, "  METHOD INDEX:")
		for _, r := range [][3]string{{"Total definitions:       ", "total_definitions", ">7,"},
			{"Unique method names:     ", "unique_names", ">7,"}, {"Constructors (<init>):   ", "constructors", ">7,"},
			{"Classes with methods:    ", "classes_with_methods", ">7,"},
			{"Avg methods/class:       ", "avg_methods_per_class", ">7.1f"}} {
			v, _ := m.Get(r[1])
			line(r[0], v, r[2])
		}
		maxN, _ := m.Get("max_methods_per_class")
		maxC, _ := m.Get("max_methods_class")
		fmt.Fprintf(w, "    Max methods/class:       %s (%s)\n\n", format(maxN, ">7,"), core.PyStr(maxC))
	}
	block("ANNOTATIONS INDEX", "annotations_index", nonEmpty("annotations_index"), [][3]string{
		{"Niagara types:           ", "niagara_types", ">7,"}, {"With properties:         ", "with_properties", ">7,"},
		{"With actions:            ", "with_actions", ">7,"}, {"With topics:             ", "with_topics", ">7,"},
		{"Total properties:        ", "total_properties", ">7,"}, {"Total actions:           ", "total_actions", ">7,"},
		{"Total topics:            ", "total_topics", ">7,"},
		{"Unique property types:   ", "unique_property_types", ">7,"},
		{"Unique action names:     ", "unique_action_names", ">7,"}})
	block("CALL GRAPH INDEX", "callgraph_index", nonEmpty("callgraph_index"), [][3]string{
		{"Caller methods:          ", "caller_methods", ">7,"}, {"Unique callees:          ", "unique_callees", ">7,"},
		{"Total edges:             ", "total_edges", ">7,"},
		{"Avg callees/caller:      ", "avg_callees_per_caller", ">7.1f"}})
	if nonEmpty("field_index") {
		f := obj("field_index")
		fmt.Fprintln(w, "  FIELD INDEX:")
		for _, r := range [][3]string{{"Total definitions:       ", "total_definitions", ">7,"},
			{"Unique field names:      ", "unique_names", ">7,"}, {"Static fields:           ", "static_fields", ">7,"},
			{"Final fields:            ", "final_fields", ">7,"}, {"Constants (static final):", "constants", ">7,"},
			{"With initial value:      ", "with_init", ">7,"}, {"Classes with fields:     ", "classes_with_fields", ">7,"},
			{"Avg fields/class:        ", "avg_fields_per_class", ">7.1f"}} {
			v, _ := f.Get(r[1])
			line(r[0], v, r[2])
		}
		maxN, _ := f.Get("max_fields_per_class")
		maxC, _ := f.Get("max_fields_class")
		fmt.Fprintf(w, "    Max fields/class:        %s (%s)\n\n", format(maxN, ">7,"), core.PyStr(maxC))
	}
	block("EXCEPTIONS INDEX", "exceptions_index", nonEmpty("exceptions_index"), [][3]string{
		{"Throws declarations:     ", "throws_declarations", ">7,"}, {"Catch blocks:            ", "catch_blocks", ">7,"},
		{"Unique thrown types:      ", "unique_thrown_types", ">7,"},
		{"Unique caught types:      ", "unique_caught_types", ">7,"},
		{"Classes with throws:     ", "classes_with_throws", ">7,"},
		{"Classes with catches:    ", "classes_with_catches", ">7,"}})
	tk := obj("token_index")
	block("TOKEN INDEX (SQLite)", "token_index", tk.Len() > 0, [][3]string{
		{"Total postings:      ", "total_postings", ">11,"}, {"Unique tokens:       ", "unique_tokens", ">11,"},
		{"Files with tokens:   ", "files_with_tokens", ">11,"}, {"Total lines scanned: ", "total_lines", ">11,"}})
	block("STRING INDEX (SQLite)", "string_index", obj("string_index").Len() > 0, [][3]string{
		{"Total strings:       ", "total_strings", ">11,"}, {"Unique strings:      ", "unique_strings", ">11,"},
		{"Files with strings:  ", "files_with_strings", ">11,"}})

	fmt.Fprintln(w, "  INDEXES:")
	for _, f := range files {
		fmt.Fprintf(w, "    %s %s MB\n", core.LJust(f.name, 30), core.RJust(core.Fixed(roundMB(f.size), 1), 6))
	}
	fmt.Fprintf(w, "    %s %s MB\n\n", core.LJust("TOTAL", 30), core.RJust(core.Fixed(roundMB(totalBytes), 1), 6))
	printHelpStatus(w, env.BaseDir, reader)
	commands := statsCommands()
	fmt.Fprintf(w, "  COMMANDS: %d available\n", len(commands))
	fmt.Fprintf(w, "    %s\n\n", strings.Join(commands, ", "))
}

// printHelpStatus is the HELP NAVIGATOR INTEGRATION block (help_bridge
// get_help_nav_status): niagara-help/ next to the base directory.
func printHelpStatus(w io.Writer, baseDir string, reader *statsCache) {
	fmt.Fprintln(w, "  HELP NAVIGATOR INTEGRATION:")
	help := pycompat.Join(hostDirname(baseDir), "niagara-help")
	idx := filepath.Join(help, "indexes")
	if !pycompat.IsDir(help) || !pycompat.IsDir(idx) {
		fmt.Fprintln(w, "    Status:      NOT AVAILABLE")
		fmt.Fprintln(w, "    (niagara-help/ not found as sibling of module-navigator/)")
		fmt.Fprintln(w)
		return
	}
	count := func(name string) int64 {
		s := reader.summary(filepath.Join(idx, name), true)
		if !s.Loaded || !s.Truthy {
			return 0
		}
		return s.Len
	}
	jsons := 0
	if names, err := readDirNames(idx); err == nil {
		for _, n := range names {
			if strings.HasSuffix(n, ".json") {
				jsons++
			}
		}
	}
	fmt.Fprintln(w, "    Status:      CONNECTED")
	fmt.Fprintf(w, "    Path:        %s\n", help)
	fmt.Fprintf(w, "    Classes:     %s\n", core.RJust(core.Commas(count("class-index.json")), 7))
	fmt.Fprintf(w, "    Source files: %s\n", core.RJust(core.Commas(count("source-index.json")), 6))
	fmt.Fprintf(w, "    With slots:  %s\n", core.RJust(core.Commas(count("slots-index.json")), 7))
	fmt.Fprintf(w, "    Indexes:     %s\n\n", core.RJust(strconv.Itoa(jsons), 7))
}

// hostDirname is os.path.dirname for the host: posixDirname off Windows; ntpath.dirname on
// Windows, where either separator splits and the drive (or UNC share) and root are kept.
func hostDirname(p string) string {
	if runtime.GOOS != "windows" {
		return posixDirname(p)
	}
	vol := filepath.VolumeName(p)
	rest, root := p[len(vol):], ""
	if rest != "" && (rest[0] == '/' || rest[0] == '\\') {
		root, rest = rest[:1], rest[1:]
	}
	head := rest[:strings.LastIndexAny(rest, `/\`)+1]
	return vol + root + strings.TrimRight(head, `/\`)
}

// posixDirname is os.path.dirname (posixpath).
func posixDirname(p string) string {
	head := p[:strings.LastIndex(p, "/")+1]
	if head != "" && strings.Trim(head, "/") != "" {
		head = strings.TrimRight(head, "/")
	}
	return head
}
