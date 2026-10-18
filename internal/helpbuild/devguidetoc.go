package helpbuild

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// The port of tools/build_devguide_toc.py: indexes/devguide-toc.json from
// devguide/index.html. Each <h1 id='...'>Title</h1> opens a section; each
// following <a href='file'>Title</a> description</li> is an item (external
// http(s) links skipped); a heading with no item becomes one entry of the
// previous section. The regexes are Python's, run by internal/pyre.

var (
	dgComment = pyre.MustCompile(`<!--.*?-->`, pyre.DOTALL)
	dgH1      = pyre.MustCompile(`<h1 id=['"]([^'"]+)['"]\s*>(.*?)</h1>`, pyre.DOTALL)
	dgHref    = pyre.MustCompile(`<a href=['"]([^'"]+)['"]`, 0)
	dgItem    = pyre.MustCompile(`<a href=['"]([^'"]+)['"]>(.*?)</a>(.*?)</li>`, pyre.DOTALL)
	dgTag     = pyre.MustCompile(`<[^>]+>`, 0)
)

// pyreSub is pattern.sub(repl, text) for a literal repl.
func pyreSub(re *pyre.Regexp, text []rune, repl string) []rune {
	var out []rune
	last := 0
	for _, m := range re.FindIter(text) {
		out = append(out, text[last:m.Start()]...)
		out = append(out, []rune(repl)...)
		last = m.End()
	}
	return append(out, text[last:]...)
}

// dgClean is _clean: tags dropped, entities decoded, whitespace collapsed,
// leading ":,.; " and one trailing "." removed.
func dgClean(text string) string {
	text = strings.Join(pycompat.Fields(pyhtml.Unescape(string(pyreSub(dgTag, []rune(text), "")))), " ")
	text = strings.TrimLeft(text, ":,.; ")
	return strings.TrimSuffix(text, ".")
}

func isExternal(href string) bool {
	return strings.HasPrefix(href, "http:") || strings.HasPrefix(href, "https:")
}

func dgEntry(file, title, desc string) pyjson.Value {
	e := pyjson.NewObject()
	e.Set("file", pyjson.String(file))
	e.Set("title", pyjson.String(dgClean(title)))
	e.Set("description", pyjson.String(dgClean(desc)))
	return e
}

// DevguideTOC is build_devguide_toc.build(baseDir): {"sections": [...]}.
func DevguideTOC(baseDir string) (pyjson.Value, error) {
	raw, err := readPage(pycompat.Join(baseDir, "devguide", "index.html"))
	if err != nil {
		return pyjson.Value{}, err
	}
	page := pyreSub(dgComment, []rune(raw), "")
	heads := dgH1.FindIter(page)
	var sections []pyjson.Value
	for i, h := range heads {
		end := len(page)
		if i+1 < len(heads) {
			end = heads[i+1].Start()
		}
		body := page[h.End():end]
		var items []pyjson.Value
		for _, m := range dgItem.FindIter(body) {
			if href := m.Group(1); !isExternal(href) {
				items = append(items, dgEntry(href, m.Group(2), m.Group(3)))
			}
		}
		if len(items) > 0 {
			s := pyjson.NewObject()
			s.Set("id", pyjson.String(h.Group(1)))
			s.Set("title", pyjson.String(dgClean(h.Group(2))))
			s.Set("items", pyjson.Value{Kind: pyjson.Array, A: items})
			sections = append(sections, s)
			continue
		}
		// A heading with only a paragraph becomes one entry of the
		// previous section, as in the original index.
		for _, href := range dgHref.FindAll(string(body)) {
			if isExternal(href) {
				continue
			}
			if len(sections) > 0 {
				prev := sections[len(sections)-1]
				it := prev.M["items"]
				it.A = append(it.A, dgEntry(href, h.Group(2), string(body)))
				prev.M["items"] = it
			}
			break
		}
	}
	toc := pyjson.NewObject()
	toc.Set("sections", pyjson.Value{Kind: pyjson.Array, A: append([]pyjson.Value{}, sections...)})
	return toc, nil
}

// BuildDevguideTOC is build_devguide_toc.main(): baseDir/indexes/
// devguide-toc.json written with indent=2.
func BuildDevguideTOC(baseDir string) (*TextResult, error) {
	toc, err := DevguideTOC(baseDir)
	if err != nil {
		return nil, err
	}
	out := pycompat.Join(baseDir, "indexes", "devguide-toc.json")
	if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
		return nil, err
	}
	if err := WriteJSON(out, toc, DumpIndent2); err != nil {
		return nil, err
	}
	n := len(toc.M["sections"].A)
	return &TextResult{Files: n, Log: []string{fmt.Sprintf("devguide-toc.json: %d sections -> %s", n, out)}}, nil
}
