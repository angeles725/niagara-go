package helpbuild

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Module examples (no Python twin): the templates the installed module jars
// ship outside their documentation (fts.IsModuleExample: Px views, Handlebars
// and Velocity templates, XML .bog files), such as kitControl's example Px
// views or the Velocity pages of web-rt. module_examples_extractor writes
// each one to module-examples/<jar stem>/<entry>, its own extension kept, as
// a header (TITLE, MODULE, ENTRY) and the template as the jar holds it, so a
// find hit names the module and the entry. A doc jar's template, which the
// doc-jar extraction copies already (destPath), is not read again. A file
// that is not text (a NUL byte or invalid UTF-8; a .bog that is not XML,
// as a zipped or binary bog) writes no page and is a warning, a vendored copy
// is counted as skipped: the log line accounts for every template entry.

const (
	moduleExamplesDir    = "module-examples"
	moduleExamplesScript = "module_examples_extractor"
)

// moduleExampleKinds are the extensions the log line counts, in its order.
var moduleExampleKinds = []string{".px", ".hbs", ".vm", ".bog"}

// isModuleExampleEntry is whether the extractor accounts for this entry of
// the jar jarName (any jar of modules/): a file with a module-examples
// extension, vendored or not, that the doc-jar extraction does not copy.
func isModuleExampleEntry(jarName, entry string, rules *Rules) bool {
	if jarName == javadocJar || !fts.HasModuleExampleExt(entry) {
		return false
	}
	_, mapped := destPath(jarName, entry, rules)
	return !mapped
}

// moduleExamplePage is the module-examples/ page of one template, or why the
// entry has no text to index.
func moduleExamplePage(jarName, entry string, data []byte) (page textPage, why string) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	switch {
	case bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data):
		if strings.HasSuffix(entry, ".bog") {
			return textPage{}, "not XML (binary/zip bog)"
		}
		return textPage{}, "binary, not text (NUL byte or invalid UTF-8)"
	case strings.HasSuffix(entry, ".bog") && !bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("<")):
		return textPage{}, "not XML (binary/zip bog)"
	}
	stem := strings.TrimSuffix(jarName, ".jar")
	header := fmt.Sprintf("TITLE: %s %s\nMODULE: %s\nENTRY: %s\n\n", stem, pyBasename(entry), stem, entry)
	return textPage{stem + "/" + entry, header + string(data)}, ""
}

// moduleExampleCounts are what the extractor met: template entries, pages
// by extension, and the entries without one.
type moduleExampleCounts struct {
	entries, notText, unreadable, vendored int
	byKind                                 map[string]int
}

// moduleExamplesOf reads the template pages of one jar into pages.
func moduleExamplesOf(jar string, rules *Rules, c *jarCensus, n *moduleExampleCounts, res *textResult) (pages []textPage) {
	name := c.count(jar)
	members, err := readMembers(jar, func(entry string) bool { return isModuleExampleEntry(name, entry, rules) })
	if err != nil {
		c.fail(res, jar, "", err)
		return nil
	}
	for _, m := range members {
		n.entries++
		switch {
		case fts.IsVendoredWeb(m.name):
			n.vendored++
		case m.err != nil:
			n.unreadable++
			c.fail(res, name, m.name, m.err)
		default:
			p, why := moduleExamplePage(name, m.name, m.data)
			if why != "" {
				n.notText++
				c.fail(res, name, m.name, errors.New(why))
				continue
			}
			pages = append(pages, p)
			n.byKind[pathExt(m.name)]++
		}
	}
	return pages
}

// pathExt is the extension of a "/"-separated path, dot included.
func pathExt(p string) string {
	if i := strings.LastIndexByte(p, '.'); i > strings.LastIndexByte(p, '/') {
		return p[i:]
	}
	return ""
}

// buildModuleExamplesFrom writes root/module-examples/ from the jars of
// installDir/modules.
func buildModuleExamplesFrom(root, installDir string) (*textResult, error) {
	jars, err := lexiconJars(installDir)
	if err != nil {
		return nil, err
	}
	rules, err := Default()
	if err != nil {
		return nil, err
	}
	res := &textResult{}
	c := &jarCensus{script: moduleExamplesScript}
	n := &moduleExampleCounts{byKind: map[string]int{}}
	var pages []textPage
	for _, jar := range jars {
		pages = append(pages, moduleExamplesOf(jar, rules, c, n, res)...)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].rel < pages[j].rel })
	if err := writeTextSource(root, moduleExamplesDir, pages); err != nil {
		return nil, err
	}
	kinds := make([]string, len(moduleExampleKinds))
	for i, k := range moduleExampleKinds {
		kinds[i] = fmt.Sprintf("%d %s", n.byKind[k], k)
	}
	unreadableJars := len(c.errors) - n.notText - n.unreadable
	res.Files = len(pages)
	res.Log = append(res.Log, fmt.Sprintf("%s/: %d templates (%s) of %d template entries: %d not text, %d unreadable, "+
		"%d vendored (from %s, %d jars, %d unreadable)", moduleExamplesDir, len(pages), strings.Join(kinds, ", "),
		n.entries, n.notText, n.unreadable, n.vendored, pycompat.Join(installDir, "modules"), c.jars, unreadableJars))
	return res, nil
}

// buildModuleExamples is the module_examples_extractor step of rebuild-all:
// module-examples/ from the modules/ of the install registered for root.
var buildModuleExamples = fromInstall(moduleExamplesDir+"/", buildModuleExamplesFrom)
