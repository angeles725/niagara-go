package helpbuild

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/pyre"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The page half of the port of tools/bajadoc_parser.py: parse_bajadoc_html
// and the extract_* / parse_*_summary helpers it calls, over one javadoc
// page read with errors="replace" and universal newlines. The regexes are
// Python's, run by internal/pyre on code points (Unicode \s, \S, \w, \b,
// lazy quantifiers and backtracking as sre has them); strip_tags is one
// html.parser feed() without close() (pyhtml.Feed).

// BajadocClass is the dict parse_bajadoc_html returns for a class page.
type BajadocClass struct {
	Name, Kind, Package, Since, Extends, Description      string
	Implements, Subclasses, Fields, Constructors, Methods []string
}

var (
	bdPackage    = pyre.MustCompile(`<div class="subTitle">(javax\.baja[^<]*)</div>`, 0)
	bdTitleH2    = pyre.MustCompile(`<h2[^>]*class="title"[^>]*>\s*(Class|Interface|Enum|Annotation Type)\s+(\S+)\s*</h2>`, 0)
	bdTitleTag   = pyre.MustCompile(`<title>\s*(\S+)\s*\(`, 0)
	bdSince      = pyre.MustCompile(`<span class="simpleTagLabel">Since:</span>\s*</dt>\s*<dd>([^<]+)</dd>`, 0)
	bdPre        = pyre.MustCompile(`<pre>.*?</pre>`, pyre.DOTALL)
	bdExtends    = pyre.MustCompile(`\bextends\s+(?:<a[^>]*>)?([^<\s]+)`, 0)
	bdImplements = pyre.MustCompile(`\bimplements\s+(.*)`, pyre.DOTALL)
	bdImplName   = pyre.MustCompile(`(?:<a[^>]*>)?([A-Z]\w+(?:\.[A-Z]\w+)*)(?:</a>)?`, 0)
	bdSubLists   = []*pyre.Regexp{
		pyre.MustCompile(`<dt>Direct Known Subclasses:</dt>\s*<dd>(.*?)</dd>`, pyre.DOTALL),
		pyre.MustCompile(`<dt>All Known Subinterfaces:</dt>\s*<dd>(.*?)</dd>`, pyre.DOTALL),
		pyre.MustCompile(`<dt>All Known Implementing Classes:</dt>\s*<dd>(.*?)</dd>`, pyre.DOTALL),
	}
	bdSubName  = pyre.MustCompile(`>([A-Z]\w+)</a>`, 0)
	bdBlock    = pyre.MustCompile(`<div class="block">(.*?)</div>`, pyre.DOTALL)
	bdFieldRow = pyre.MustCompile(`<td class="colFirst"><code>(.*?)</code></td>\s*`+
		`<td class="colLast"><code>.*?>([\w.]+)</a>.*?</code>(.*?)</td>`, pyre.DOTALL)
	bdCtorRow   = pyre.MustCompile(`<td class="colOne"><code>.*?<a[^>]*>([\w.]+)</a>.*?\((.*?)\)</code>(.*?)</td>`, pyre.DOTALL)
	bdMethodRow = pyre.MustCompile(`<td class="colFirst"><code>(.*?)</code></td>\s*`+
		`<td class="colLast"><code>.*?<a[^>]*>([\w.]+)</a>.*?\((.*?)\)</code>(.*?)</td>`, pyre.DOTALL)
)

// bdNotClasses are the names parse_bajadoc_html refuses as class names.
var bdNotClasses = map[string]bool{"package-summary": true, "package-tree": true, "package-use": true,
	"package-frame": true, "class-use": true}

// The comment markers that open the three summaries, and the ones that end
// a summary (_find_summary_section; "<!-- ==========" is kept although
// "<!-- ========" always matches first, as in the Python list).
const (
	bdFieldMarker  = "<!-- =========== FIELD SUMMARY =========== -->"
	bdCtorMarker   = "<!-- ======== CONSTRUCTOR SUMMARY ======== -->"
	bdMethodMarker = "<!-- ========== METHOD SUMMARY =========== -->"
)

var bdEndMarkers = []string{"<!-- ========", "<!-- ==========", "</li>\n</ul>\n</li>\n</ul>\n</div>"}

// tagStripper is _TagStripper: the data callbacks joined. Its
// handle_entityref and handle_charref are never called (convert_charrefs).
type tagStripper struct{ strings.Builder }

func (s *tagStripper) StartTag(string, []pyhtml.Attr)    {}
func (s *tagStripper) StartEndTag(string, []pyhtml.Attr) {}
func (s *tagStripper) EndTag(string)                     {}
func (s *tagStripper) Data(text string)                  { s.WriteString(text) }

// stripTags is strip_tags: the data of one feed() without close(). Its
// regex fallback (an exception from feed) is unreachable: feed() raises only
// through its internal asserts, which the tokenizer never trips.
func stripTags(fragment string) string {
	var s tagStripper
	pyhtml.Feed(fragment, &s)
	return s.String()
}

// normalizeWS is re.sub(r"\s+", " ", text).strip(): \s and strip() use the
// same str.isspace() code points as str.split().
func normalizeWS(text string) string { return strings.Join(textfold.Fields(text), " ") }

// classNameAndKind is extract_class_name_and_kind; "" is None.
func classNameAndKind(page []rune) (name, kind string) {
	if m := bdTitleH2.SearchRunes(page, 0); m != nil {
		return m.Group(2), strings.ReplaceAll(strings.ToLower(m.Group(1)), " ", "_")
	}
	if m := bdTitleTag.SearchRunes(page, 0); m != nil {
		name = m.Group(1)
		if strings.HasPrefix(name, "BI") {
			return name, "interface"
		}
		return name, "class"
	}
	return "", ""
}

// implementsFromDecl is extract_implements_from_decl on the declaration.
func implementsFromDecl(decl []rune) []string {
	im := bdImplements.SearchRunes(decl, 0)
	if im == nil {
		return nil
	}
	text := []rune(strings.ReplaceAll(im.Group(1), "</pre>", ""))
	var names []string
	for _, m := range bdImplName.FindIter(text) {
		names = append(names, m.Group(1))
	}
	return names
}

// subclasses is extract_subclasses.
func subclasses(page []rune) []string {
	for _, re := range bdSubLists {
		if dd := re.SearchRunes(page, 0); dd != nil {
			var names []string
			for _, n := range bdSubName.FindIter([]rune(dd.Group(1))) {
				names = append(names, normalizeWS(n.Group(1)))
			}
			return names
		}
	}
	return nil
}

// summarySection is _find_summary_section; "" is None or empty (both
// mean no rows). The end markers are searched from 10 code points past
// the marker.
func summarySection(html, marker string) string {
	idx := strings.Index(html, marker)
	if idx < 0 {
		return ""
	}
	start := idx + len(marker)
	from := start
	for c := 0; c < 10 && from < len(html); c++ {
		_, size := utf8.DecodeRuneInString(html[from:])
		from += size
	}
	end := len(html)
	for _, em := range bdEndMarkers {
		if pos := strings.Index(html[from:], em); pos >= 0 && from+pos < end {
			end = from + pos
		}
	}
	return html[start:end]
}

// summaryRows runs a row regex over a summary section.
func summaryRows(html, marker string, row *pyre.Regexp) []*pyre.Match {
	section := summarySection(html, marker)
	if section == "" {
		return nil
	}
	return row.FindIter([]rune(section))
}

// fieldSummary is parse_field_summary.
func fieldSummary(html string) []string {
	var out []string
	for _, m := range summaryRows(html, bdFieldMarker, bdFieldRow) {
		ftype, name := normalizeWS(stripTags(m.Group(1))), m.Group(2)
		if desc := normalizeWS(stripTags(m.Group(3))); desc != "" {
			out = append(out, "  "+ftype+" "+name+" - "+desc)
		} else {
			out = append(out, "  "+ftype+" "+name)
		}
	}
	return out
}

// constructorSummary is parse_constructor_summary.
func constructorSummary(html string) []string {
	var out []string
	for _, m := range summaryRows(html, bdCtorMarker, bdCtorRow) {
		out = append(out, "  "+m.Group(1)+"("+normalizeWS(stripTags(m.Group(2)))+")")
	}
	return out
}

// methodSummary is parse_method_summary.
func methodSummary(html string) []string {
	var out []string
	for _, m := range summaryRows(html, bdMethodMarker, bdMethodRow) {
		ret, params := normalizeWS(stripTags(m.Group(1))), normalizeWS(stripTags(m.Group(3)))
		out = append(out, "  "+ret+" "+m.Group(2)+"("+params+")")
	}
	return out
}

// parseBajadocPage is parse_bajadoc_html on the page text; nil is None (no
// class name, or a package/class-use page).
func parseBajadocPage(html string) *BajadocClass {
	page := []rune(html)
	name, kind := classNameAndKind(page)
	if name == "" || bdNotClasses[name] {
		return nil
	}
	c := &BajadocClass{Name: name, Kind: kind}
	if m := bdPackage.SearchRunes(page, 0); m != nil {
		c.Package = m.Group(1)
	}
	if m := bdSince.SearchRunes(page, 0); m != nil {
		c.Since = normalizeWS(m.Group(1))
	}
	if pre := bdPre.SearchRunes(page, 0); pre != nil {
		decl := page[pre.Start():pre.End()]
		if em := bdExtends.SearchRunes(decl, 0); em != nil {
			c.Extends = em.Group(1)
		}
		c.Implements = implementsFromDecl(decl)
		if m := bdBlock.SearchRunes(page[pre.End():], 0); m != nil {
			c.Description = normalizeWS(stripTags(m.Group(1)))
		}
	}
	c.Subclasses = subclasses(page)
	c.Fields = fieldSummary(html)
	c.Constructors = constructorSummary(html)
	c.Methods = methodSummary(html)
	return c
}
