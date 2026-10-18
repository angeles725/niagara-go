package drivers

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The doc module of a driver (drivers.py _doc_target): the module its lexicon
// names in help.guide.base, else the doc submodule named like its module
// directory.

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}

// docIndex holds _doc_modules ({lower-case core: doc submodule}, in inventory
// order) and _doc_submodules ({doc module name: its submodule}).
type docIndex struct {
	cores    []string
	byCore   map[string]string
	byModule map[string]string
}

func newDocIndex(inventory *index.Object) *docIndex {
	d := &docIndex{byCore: map[string]string{}, byModule: map[string]string{}}
	if inventory == nil {
		return d
	}
	for i, sub := range inventory.Keys() {
		info, _ := inventory.Values()[i].(*index.Object)
		if info == nil || core.Str(info, "type") != "doc" {
			continue
		}
		module, _ := info.String("module")
		if strings.HasPrefix(module, "doc") && module != "docSource" {
			key := textfold.Lower(module[3:])
			if _, seen := d.byCore[key]; !seen {
				d.cores = append(d.cores, key)
			}
			d.byCore[key] = sub
		}
		if core.Truthy(info, "module") {
			d.byModule[core.Str(info, "module")] = sub
		}
	}
	return d
}

// docTarget is the doc fields of a row (the dict _doc_target returns).
type docTarget struct {
	base   *string  // help_guide_base
	others []string // help_guide_base_others
	target *string  // help_doc_module
	exists *bool    // help_doc_module_exists
	doc    *string  // doc_module
	via    *string  // doc_module_via
}

func strp(s string) *string { return &s }

// target is _doc_target for the network in submodule sub of module directory dir.
func (d *docIndex) target(organized, dir, sub string) docTarget {
	bases := helpGuideBases(organized, dir, sub)
	t := docTarget{others: []string{}}
	if len(bases) > 0 {
		t.base, t.others = &bases[0], bases[1:]
	}
	if t.base != nil && strings.HasPrefix(*t.base, "module://") {
		rest := (*t.base)[len("module://"):]
		if name, _, _ := strings.Cut(rest, "/"); name != "" {
			t.target = &name
		}
	}
	if t.target != nil {
		exists := pycompat.IsDir(filepath.Join(organized, *t.target))
		t.exists = &exists
		if doc, ok := d.byModule[*t.target]; exists && ok {
			t.doc = &doc
		}
		t.via = strp("help.guide.base")
	} else if doc, ok := d.docModule(dir); ok {
		t.doc, t.via = &doc, strp("by name")
	}
	return t
}

// docModule is _doc_module: the doc submodule named like the module directory:
// equal, else the longest doc name the directory starts with, else the
// shortest one starting with the directory (stable on ties, inventory order).
func (d *docIndex) docModule(dir string) (string, bool) {
	f := textfold.Lower(dir)
	if doc, ok := d.byCore[f]; ok {
		return doc, true
	}
	n := utf8.RuneCountInString
	var longer, shorter []string
	for _, c := range d.cores {
		if n(c) >= 3 && strings.HasPrefix(f, c) {
			longer = append(longer, c)
		}
		if n(f) >= 3 && strings.HasPrefix(c, f) {
			shorter = append(shorter, c)
		}
	}
	sort.SliceStable(longer, func(a, b int) bool { return n(longer[a]) > n(longer[b]) })
	if len(longer) > 0 {
		return d.byCore[longer[0]], true
	}
	sort.SliceStable(shorter, func(a, b int) bool { return n(shorter[a]) < n(shorter[b]) })
	if len(shorter) > 0 {
		return d.byCore[shorter[0]], true
	}
	return "", false
}

// sortedDir is sorted(os.listdir(dir)); ok=false when it cannot be listed.
func sortedDir(dir string) ([]string, bool) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, false
	}
	sort.Strings(names)
	return names, true
}

// moduleLexicons is _module_lexicons: organized/<dir>/<sub>/extracted/<dir>*.lexicon
// without a language variant, the network's own submodule first.
func moduleLexicons(organized, dir, sub string) []string {
	root := filepath.Join(organized, dir)
	subs, ok := sortedDir(root)
	if !ok {
		return nil
	}
	sort.SliceStable(subs, func(a, b int) bool { return subs[a] == sub && subs[b] != sub })
	var out []string
	for _, s := range subs {
		ext := filepath.Join(root, s, "extracted")
		names, ok := sortedDir(ext)
		if !ok {
			continue
		}
		for _, n := range names {
			if strings.HasSuffix(n, ".lexicon") && strings.HasPrefix(n, dir) &&
				!strings.Contains(n[len(dir):], "_") {
				out = append(out, filepath.Join(ext, n))
			}
		}
	}
	return out
}

var helpGuideBase = regexp.MustCompile(`(?m)^[ \t]*help\.guide\.base[ \t]*[=:][ \t]*(.*?)[ \t\r]*$`)

// unescape is re.sub(r'\\(.)', r'\1', s): properties escapes removed.
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] != '\n' {
			i++
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		b.WriteRune(r)
		i += n
	}
	return b.String()
}

// helpGuideBases is help_guide_bases: every distinct help.guide.base value of
// the module lexicons, the network's own submodule first; within one file the
// last definition wins. A file that is not UTF-8 is read as Latin-1.
func helpGuideBases(organized, dir, sub string) []string {
	var values []string
	for _, p := range moduleLexicons(organized, dir, sub) {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		text := string(raw)
		if !utf8.Valid(raw) {
			runes := make([]rune, len(raw))
			for i, c := range raw {
				runes[i] = rune(c)
			}
			text = string(runes)
		}
		found := helpGuideBase.FindAllStringSubmatch(text, -1)
		if len(found) == 0 {
			continue
		}
		if v := unescape(found[len(found)-1][1]); !contains(values, v) {
			values = append(values, v)
		}
	}
	return values
}
