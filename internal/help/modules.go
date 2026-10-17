package help

// Module cross-reference (module_search.py cmd_module): the classes, guide
// folders, Developer Guide entries and PDF manuals whose names mention a
// module or topic.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
)

// moduleSuffix is r"-(rt|ux|wb|se|doc)$"; Python's $ also matches before a
// final newline, which the optional group keeps.
var moduleSuffix = regexp.MustCompile(`-(?:rt|ux|wb|se|doc)(\n?)$`)

// moduleKey is _module_key: "alarm-rt" -> "alarm", "docAlarms" -> "alarm".
func moduleKey(name string) string {
	clean := moduleSuffix.ReplaceAllString(lowerStr(name), "$1")
	clean = strings.TrimPrefix(clean, "doc")
	return strings.TrimRight(clean, "s")
}

type namedEntry struct {
	name string
	data *index.Object
}

// moduleClassMatches is module_class_matches: every class variant whose
// module equals the name or contains the key, or whose name or package
// contains the key.
func moduleClassMatches(idx *index.Object, module string) []namedEntry {
	clean, exact := moduleKey(module), lowerStr(module)
	var out []namedEntry
	for _, name := range idx.Keys() {
		entry := idx.Object(name)
		variants := append([]*index.Object{entry}, objList(entry, "others")...)
		for _, d := range variants {
			mod, pkg := lowerStr(getStr(d, "module", "")), lowerStr(getStr(d, "package", ""))
			if mod == exact || (clean != "" && strings.Contains(mod, clean)) ||
				strings.Contains(lowerStr(name), clean) || strings.Contains(pkg, clean) {
				out = append(out, namedEntry{name, d})
			}
		}
	}
	return out
}

func sortByName(es []namedEntry) {
	sort.SliceStable(es, func(i, j int) bool { return es[i].name < es[j].name })
}

// Module is cmd_module. Each index is read when its section prints, so a
// missing one stops the output where Python's traceback would.
func (e *Env) Module(module string) error {
	clean := moduleKey(module)
	fmt.Fprintf(e.Out, "Module summary for '%s' (search key: '%s')\n%s\n", module, clean, strings.Repeat("=", 60))

	classes, err := e.loadIndex("class-index.json", true)
	if err != nil {
		return err
	}
	cm := moduleClassMatches(classes, module)
	fmt.Fprintf(e.Out, "\nCLASSES (%d found):\n", len(cm))
	sortByName(cm)
	for _, c := range cm[:min(len(cm), 20)] {
		fmt.Fprintf(e.Out, "  %s (%s) - %s\n", c.name, getStr(c.data, "type", ""), getStr(c.data, "package", ""))
	}
	moreOrNone(e, len(cm), 20)

	guides, err := e.loadIndex("guides-index.json", true)
	if err != nil {
		return err
	}
	var gm []namedEntry
	for _, folder := range guides.Keys() {
		d := guides.Object(folder)
		if strings.Contains(lowerStr(folder), clean) || strings.Contains(lowerStr(truthyStr(d, "source_jar")), clean) {
			gm = append(gm, namedEntry{folder, d})
		}
	}
	fmt.Fprintf(e.Out, "\nGUIDES (%d found):\n", len(gm))
	sortByName(gm)
	for _, g := range gm {
		fmt.Fprintf(e.Out, "  %s (%s topics) - %s\n", g.name, topicCount(g.data, 0), orStr(g.data, "source_jar", "", "?"))
	}
	moreOrNone(e, len(gm), len(gm))

	toc, err := e.loadIndex("devguide-toc.json", true)
	if err != nil {
		return err
	}
	var dm []string
	for _, section := range objList(toc, "sections") {
		for _, item := range objList(section, "items") {
			if strings.Contains(lowerStr(getStr(item, "title", "")), clean) ||
				strings.Contains(lowerStr(getStr(item, "description", "")), clean) {
				dm = append(dm, fmt.Sprintf("  [%s] %s - devguide/%s", getStr(section, "title", ""),
					getStr(item, "title", ""), getStr(item, "file", "")))
			}
		}
	}
	fmt.Fprintf(e.Out, "\nDEVGUIDE (%d found):\n", len(dm))
	for _, line := range dm[:min(len(dm), 10)] {
		fmt.Fprintln(e.Out, line)
	}
	if len(dm) == 0 {
		fmt.Fprintln(e.Out, "  (none)")
	}

	pdfs, err := e.loadIndex("pdf-index.json", true)
	if err != nil {
		return err
	}
	var pm []namedEntry
	for _, name := range pdfs.Keys() {
		d := pdfs.Object(name)
		if strings.Contains(lowerStr(strings.ReplaceAll(name, "_", " ")), clean) ||
			strings.Contains(lowerStr(getStr(d, "folder", "")), clean) ||
			strings.Contains(lowerStr(getStr(d, "source_pdf", "")), clean) {
			pm = append(pm, namedEntry{name, d})
		}
	}
	fmt.Fprintf(e.Out, "\nPDFs (%d found):\n", len(pm))
	sortByName(pm)
	for _, p := range pm[:min(len(pm), 15)] {
		fmt.Fprintf(e.Out, "  %s (%s pages, %s)\n", strings.ReplaceAll(p.name, "_", " "),
			getStr(p.data, "pages", "?"), getStr(p.data, "category", "?"))
	}
	moreOrNone(e, len(pm), 15)

	if len(cm)+len(gm)+len(dm)+len(pm) == 0 {
		fmt.Fprintln(e.Out)
		return e.reportMiss("module", module, "classes")
	}
	return nil
}

// moreOrNone closes a section of n matches that listed the first shown:
// "  ... and N more" past it, "  (none)" when empty.
func moreOrNone(e *Env, n, shown int) {
	switch {
	case n == 0:
		fmt.Fprintln(e.Out, "  (none)")
	case n > shown:
		fmt.Fprintf(e.Out, "  ... and %d more\n", n-shown)
	}
}
