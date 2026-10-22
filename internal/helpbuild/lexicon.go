package helpbuild

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/helpbuild/registry"
	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Lexicon help text (no Python twin): the *.lexicon files of the
// installed module jars (modules/*.jar, translations in the
// niagaraLexicon<Lang>-rt jars included) hold the description, help and
// tool-tip strings Workbench and the web UI show. lexicon_extractor writes
// one file per lexicon file, lexicon-text/<jar stem>/<entry>.txt, holding
// its help entries as "key = value" lines, so a hit names the module (the
// jar), the lexicon file and the key on the matching line.
//
// The file is a paged source (fulltext.paged): its entries are separated
// by form feeds, so each entry is its own full-text document, scored on
// its own words (a tool-tip that matches ranks like the sentence it is),
// while the results still list one hit per lexicon file (a file scores
// its best entry plus decayed further ones). A whole file as one document
// ranked below every guide (BM25 favours short documents: 7 of 15 sampled
// descriptions stayed out of find's ten results); one result row per key
// would flood the list. Plain UI
// labels ("Station Name") are left out: a page keeps the keys whose last
// segment names help text (description, desc, help, tooltip, summary) and
// any value of lexiconMinWords words or more (messages, prompts, warnings).

const (
	lexiconTextDir  = "lexicon-text"
	lexiconScript   = "lexicon_extractor"
	lexiconMinWords = 5
)

// lexiconHelpKeys are the last key segments (lower-cased) that mark help
// text whatever its length.
var lexiconHelpKeys = map[string]bool{"description": true, "desc": true, "help": true,
	"tooltip": true, "summary": true}

// lexEntry is one key = value pair of a lexicon file.
type lexEntry struct{ key, value string }

// parseLexicon reads a lexicon file (the java.util.Properties syntax
// Niagara uses): comment lines (# or !), "key=value" or "key:value"
// pairs, a trailing backslash continuing the line, the usual escapes and
// \uXXXX. Invalid UTF-8 is replaced, as the jars are read leniently.
func parseLexicon(data []byte) []lexEntry {
	text := strings.ToValidUTF8(strings.ReplaceAll(string(data), "\r\n", "\n"), "�")
	text = strings.TrimPrefix(text, "\uFEFF")
	lines := strings.Split(strings.ReplaceAll(text, "\r", "\n"), "\n")
	var out []lexEntry
	for i := 0; i < len(lines); i++ {
		line := strings.TrimLeft(lines[i], " \t\f")
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		for continued(line) && i+1 < len(lines) {
			i++
			line = line[:len(line)-1] + strings.TrimLeft(lines[i], " \t\f")
		}
		key, value, ok := splitLexiconLine(line)
		if ok {
			out = append(out, lexEntry{unescapeLexicon(key), unescapeLexicon(value)})
		}
	}
	return out
}

// continued is a line ending with an odd number of backslashes.
func continued(line string) bool {
	n := 0
	for i := len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

// splitLexiconLine splits at the first unescaped '=' or ':'.
func splitLexiconLine(line string) (key, value string, ok bool) {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			i++
		case '=', ':':
			return strings.TrimSpace(line[:i]), strings.TrimLeft(line[i+1:], " \t\f"), true
		}
	}
	return "", "", false
}

// unescapeLexicon resolves \uXXXX, \t \n \r \f (as spaces: a page line is
// one entry) and drops the backslash of any other escaped character.
func unescapeLexicon(s string) string {
	if !strings.Contains(s, `\`) {
		return strings.TrimSpace(s)
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 == len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 't', 'n', 'r', 'f':
			b.WriteByte(' ')
		case 'u':
			if i+5 <= len(s) {
				if cp, err := strconv.ParseUint(s[i+1:i+5], 16, 32); err == nil {
					b.WriteRune(rune(cp))
					i += 4
					continue
				}
			}
			b.WriteByte('u')
		default:
			b.WriteByte(s[i])
		}
	}
	return strings.TrimSpace(b.String())
}

// isLexiconHelp is whether an entry is help text (see the package note).
func isLexiconHelp(e lexEntry) bool {
	if e.value == "" {
		return false
	}
	last := e.key
	if i := strings.LastIndexByte(last, '.'); i >= 0 {
		last = last[i+1:]
	}
	return lexiconHelpKeys[textfold.Lower(last)] || len(textfold.Fields(e.value)) >= lexiconMinWords
}

// isLexiconEntry is whether the extractor reads this entry of the jar
// jarName (any jar of modules/): a *.lexicon file the doc-jar extraction
// does not already copy into the corpus (an extracted one keeps its
// coverage there: doc_types excludes it as a translation lexicon).
func isLexiconEntry(jarName, entry string, rules *Rules) bool {
	if jarName == javadocJar || !strings.HasSuffix(textfold.Lower(entry), ".lexicon") {
		return false
	}
	_, mapped := destPath(jarName, entry, rules)
	return !mapped && !strings.HasSuffix(entry, "/")
}

// lexiconPage is the lexicon-text/ page of one lexicon file; ok is false
// when the file holds no help text (labels only), which writes no page.
func lexiconPage(jarName, entry string, data []byte) (page textPage, ok bool) {
	var b strings.Builder
	stem := strings.TrimSuffix(jarName, ".jar")
	title := stem
	if lex := strings.TrimSuffix(pyBasename(entry), ".lexicon"); lex != stem {
		title += " " + lex
	}
	fmt.Fprintf(&b, "TITLE: %s\nMODULE: %s\nLEXICON: %s\n\n", title, stem, entry)
	n := 0
	for _, e := range parseLexicon(data) {
		if isLexiconHelp(e) {
			if n > 0 {
				b.WriteString("\f")
			}
			fmt.Fprintf(&b, "%s = %s\n", e.key, e.value)
			n++
		}
	}
	if n == 0 {
		return textPage{}, false
	}
	return textPage{stem + "/" + entry + ".txt", b.String()}, true
}

// lexiconJars are the jars of modules/, sorted by name.
func lexiconJars(installDir string) ([]string, error) {
	names, err := sortedNames(pycompat.Join(installDir, "modules"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, n := range names {
		if strings.HasSuffix(textfold.Lower(n), ".jar") {
			out = append(out, pycompat.Join(installDir, "modules", n))
		}
	}
	return out, nil
}

// lexiconPagesOf reads one jar's lexicon pages and the number of lexicon
// files it holds.
func lexiconPagesOf(jar string, rules *Rules) (pages []textPage, files int, err error) {
	z, err := zipfile.Open(jar)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", jar, err)
	}
	defer z.Close()
	name := pyBasename(jar)
	for i := range z.Infos {
		x := &z.Infos[i]
		if !isLexiconEntry(name, x.Name, rules) {
			continue
		}
		data, err := z.ReadEntry(x, true)
		if err != nil {
			return nil, files, fmt.Errorf("%s: %s: %w", jar, x.Name, err)
		}
		files++
		if p, ok := lexiconPage(name, x.Name, data); ok {
			pages = append(pages, p)
		}
	}
	return pages, files, nil
}

// buildLexiconText is the lexicon_extractor step of rebuild-all. The
// install comes from the registry (install_dir); a version without one,
// or whose install is not mounted, keeps its last lexicon-text/ (exit 0,
// like the optional course). An unreadable jar is a warning, skipped.
func buildLexiconText(baseDir string) (*textResult, error) {
	res := &textResult{}
	install, err := registry.InstallDir(baseDir)
	if err != nil {
		return res, err
	}
	if install == "" || !pycompat.IsDir(pycompat.Join(install, "modules")) {
		where := "no install registered"
		if install != "" {
			where = "no modules/ folder in " + install
		}
		res.Log = append(res.Log, fmt.Sprintf("%s/: %s; skipped (the last copy is kept)", lexiconTextDir, where))
		return res, nil
	}
	rules, err := Default()
	if err != nil {
		return res, err
	}
	jars, err := lexiconJars(install)
	if err != nil {
		return res, err
	}
	var pages []textPage
	files, withText := 0, 0
	for _, jar := range jars {
		p, n, err := lexiconPagesOf(jar, rules)
		if err != nil {
			res.Warnings = append(res.Warnings, "WARNING: "+lexiconScript+": "+err.Error())
			continue
		}
		files += n
		withText += len(p)
		pages = append(pages, p...)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].rel < pages[j].rel })
	if err := writeTextSource(baseDir, lexiconTextDir, pages); err != nil {
		return res, err
	}
	res.Files = len(pages)
	res.Log = append(res.Log, fmt.Sprintf("%s/: %d lexicon files with help text of %d (from %s/modules, %d jars)",
		lexiconTextDir, withText, files, install, len(jars)))
	return res, nil
}

// openLexiconJar is the jar the sweep reads lexicon entries from: entries
// of one jar are classified one after the other, so one jar stays open.
type openLexiconJar struct {
	path string
	z    *zipfile.File
}

func (s *sweeper) closeLexiconJar() {
	if s.lexJar != nil {
		_ = s.lexJar.z.Close()
		s.lexJar = nil
	}
}

// lexicon is the sweep verdict of a lexicon entry of an installed module
// jar (isLexiconEntry): indexed when its lexicon-text/ page is in the
// full-text index, excluded when it holds UI labels only (no page by
// design), UNCLASSIFIED otherwise. The sweep reads the entry itself (the
// listing cache has no contents), with lexiconPage's rule of what counts
// as help text.
func (s *sweeper) lexicon(jar string, e Entry) (verdict, error) {
	if s.lexJar == nil || s.lexJar.path != jar {
		s.closeLexiconJar()
		z, err := zipfile.Open(jar)
		if err != nil {
			return verdict{statusUnclassified, "lexicon not readable: " + err.Error(), ""}, nil
		}
		s.lexJar = &openLexiconJar{jar, z}
	}
	z := s.lexJar.z
	for i := range z.Infos {
		x := &z.Infos[i]
		if x.Name != e.Name {
			continue
		}
		data, err := z.ReadEntry(x, true)
		if err != nil {
			return verdict{statusUnclassified, "lexicon not readable: " + err.Error(), ""}, nil
		}
		page, ok := lexiconPage(pyBasename(jar), e.Name, data)
		if !ok {
			return verdict{statusExcluded, "translation lexicon with UI labels only (no description, help, " +
				"tool-tip or sentence to index)", "lexicon-labels"}, nil
		}
		key := lexiconTextDir + "/" + page.rel
		docs, err := s.indexed.fulltextDocs()
		if err != nil {
			return verdict{}, err
		}
		if docs[key] {
			return verdict{statusIndexed, key, "lexicon"}, nil
		}
		return verdict{statusUnclassified, "lexicon help text is not indexed (" + key + "): run rebuild-all, then find", ""}, nil
	}
	return verdict{statusUnclassified, "lexicon not readable: entry not found in the jar", ""}, nil
}
