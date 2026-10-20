package htmltext

import (
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// The port of niagara_help_lib/html_text.py: an HTML help page to readable
// plain text, shared by the guide and devguide parsers. Entities decoded,
// <pre> fenced as ``` with its indentation, tables as "| a | b |" rows,
// images as "[image: alt -> src]", lists as "- item", the "Index | Prev |
// Next" navigation line dropped, the title from <title> else the first <h1>.
// Whitespace classes, strip and the \w of the edge rule are Python's Unicode
// ones (textfold.IsSpace, pycompat.IsWordRune).

var (
	htmlBlockTags = map[string]bool{"p": true, "div": true, "br": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "li": true, "tr": true, "dt": true, "dd": true, "ul": true, "ol": true,
		"dl": true, "table": true, "blockquote": true, "section": true, "article": true, "hr": true, "header": true,
		"footer": true, "nav": true, "center": true, "form": true}
	htmlSkipTags = map[string]bool{"script": true, "style": true, "head": true, "noscript": true}
)

// htmlEdge marks an inline tag edge (_EDGE).
const htmlEdge = "\x00"

type htmlTable struct {
	rows [][]string
	cell *strings.Builder // nil: no open cell
}

type htmlConverter struct {
	out     strings.Builder
	skip    int
	pre     int
	tables  []*htmlTable
	title   strings.Builder
	inTitle bool
	h1      strings.Builder
	h1Seen  bool // self.h1, the list of chunks, is non-empty
	inH1    bool
}

// emit appends to the innermost open table cell, else to the page.
func (c *htmlConverter) emit(text string) {
	for i := len(c.tables) - 1; i >= 0; i-- {
		if cell := c.tables[i].cell; cell != nil {
			cell.WriteString(text)
			return
		}
	}
	c.out.WriteString(text)
}

func (c *htmlConverter) boundary() {
	if c.pre == 0 {
		c.emit(htmlEdge)
	}
}

func (c *htmlConverter) StartTag(tag string, attrs []pyhtml.Attr) {
	if tag == "title" {
		c.inTitle = true
	}
	if htmlSkipTags[tag] {
		c.skip++
		return
	}
	if c.skip > 0 {
		return
	}
	if tag == "h1" && !c.h1Seen {
		c.inH1 = true
	}
	switch {
	case tag == "pre":
		c.pre++
		if c.pre == 1 {
			c.emit("\n```\n")
		}
	case tag == "table":
		c.tables = append(c.tables, &htmlTable{})
	case tag == "tr" && len(c.tables) > 0:
		c.closeCell() // HTML4 may omit </td> and </tr>
		t := c.tables[len(c.tables)-1]
		t.rows = append(t.rows, nil)
	case (tag == "td" || tag == "th") && len(c.tables) > 0:
		c.closeCell()
		t := c.tables[len(c.tables)-1]
		if len(t.rows) == 0 {
			t.rows = append(t.rows, nil)
		}
		t.cell = &strings.Builder{}
	case tag == "img":
		alt, src := attrGet(attrs, "alt"), attrGet(attrs, "src")
		label := alt
		switch {
		case alt != "" && src != "":
			label = alt + " -> " + src
		case alt == "":
			label = src
		}
		if label != "" {
			c.emit(" [image: " + label + "] ")
		}
	case tag == "li":
		c.emit("\n- ")
	case htmlBlockTags[tag]:
		c.emit("\n")
	default:
		c.boundary()
	}
}

// attrGet is (dict(attrs).get(name) or "").strip(): the last value wins.
func attrGet(attrs []pyhtml.Attr, name string) string {
	v := ""
	for _, a := range attrs {
		if a.Name == name {
			v = a.Value // None reads as ""
		}
	}
	return pycompat.Strip(v)
}

func (c *htmlConverter) StartEndTag(tag string, attrs []pyhtml.Attr) {
	c.StartTag(tag, attrs)
	if tag != "img" && tag != "br" && tag != "hr" {
		c.EndTag(tag)
	}
}

func (c *htmlConverter) EndTag(tag string) {
	if tag == "title" {
		c.inTitle = false
	}
	if htmlSkipTags[tag] {
		c.skip = max(0, c.skip-1)
		return
	}
	if c.skip > 0 {
		return
	}
	if tag == "h1" {
		c.inH1 = false
	}
	switch {
	case tag == "pre" && c.pre > 0:
		c.pre--
		if c.pre == 0 {
			c.emit("\n```\n")
		}
	case (tag == "td" || tag == "th" || tag == "tr") && len(c.tables) > 0:
		c.closeCell()
	case tag == "table" && len(c.tables) > 0:
		c.closeTable()
	case htmlBlockTags[tag]:
		c.emit("\n")
	default:
		c.boundary()
	}
}

// closeCell commits the open cell of the innermost table to its row.
func (c *htmlConverter) closeCell() {
	t := c.tables[len(c.tables)-1]
	if t.cell == nil {
		return
	}
	text := pycompat.Strip(CollapseSpace(t.cell.String()))
	if len(c.tables) == 1 {
		text = strings.ReplaceAll(text, "|", `\|`)
	}
	last := len(t.rows) - 1
	t.rows[last] = append(t.rows[last], text)
	t.cell = nil
}

// closeTable renders the innermost table, its open cell committed first; a
// nested table flattens into its outer cell as "a / b; c / d".
func (c *htmlConverter) closeTable() {
	c.closeCell()
	t := c.tables[len(c.tables)-1]
	c.tables = c.tables[:len(c.tables)-1]
	var rows [][]string
	for _, r := range t.rows {
		for _, cell := range r {
			if cell != "" {
				rows = append(rows, r)
				break
			}
		}
	}
	if len(c.tables) > 0 {
		parts := make([]string, len(rows))
		for i, r := range rows {
			parts[i] = strings.Join(r, " / ")
		}
		c.emit(" " + strings.Join(parts, "; ") + " ")
		return
	}
	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = "| " + strings.Join(r, " | ") + " |"
	}
	c.emit("\n" + strings.Join(lines, "\n") + "\n")
}

func (c *htmlConverter) Data(data string) {
	if c.inTitle {
		c.title.WriteString(data)
	}
	if c.skip > 0 {
		return
	}
	if c.inH1 {
		c.h1.WriteString(data)
		c.h1Seen = true // an empty chunk (html.unescape("&#1;")) counts too
	}
	if c.pre > 0 {
		c.emit(strings.ReplaceAll(data, "\r\n", "\n"))
	} else {
		c.emit(CollapseSpace(data))
	}
}

// CollapseSpace is re.sub(r"\s+", " ", s).
func CollapseSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if textfold.IsSpace(r) {
			if !space {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// HTMLToText is html_text.convert: the (title, text) of one HTML page.
func HTMLToText(page string) (title, text string) {
	c := &htmlConverter{}
	pyhtml.Parse(page, c)
	for len(c.tables) > 0 { // a table left open at the end of the page
		c.closeTable()
	}
	title = pycompat.Strip(CollapseSpace(c.title.String()))
	if title == "" {
		title = pycompat.Strip(CollapseSpace(c.h1.String()))
	}
	return title, tidyText(c.out.String())
}

// tidyText is _tidy: inline edges resolved, lines stripped outside code
// fences, navigation lines dropped, blank-line runs collapsed.
func tidyText(text string) string {
	text = resolveEdges(text)
	lines := strings.Split(text, "\n")
	out := lines[:0]
	fenced := false
	for _, line := range lines {
		if pycompat.Strip(line) == "```" {
			fenced = !fenced
			out = append(out, "```")
			continue
		}
		if fenced {
			out = append(out, strings.TrimRightFunc(line, textfold.IsSpace))
			continue
		}
		line = pycompat.Strip(line)
		if isNavLine(line) {
			continue
		}
		out = append(out, line)
	}
	text = strings.Join(out, "\n")
	text = squeezeAfterFence(text)
	return pycompat.Strip(squeezeNewlines(text))
}

// resolveEdges is re.sub(r"(?<=\w)\x00+(?=\w)", " ", text) then removing
// every other \x00: a run of edges becomes one space only between two word
// characters of the original text.
func resolveEdges(text string) string {
	if !strings.Contains(text, htmlEdge) {
		return text
	}
	var b strings.Builder
	prev := rune(-1) // the rune before the current position, -1 at the start
	for i := 0; i < len(text); {
		if text[i] != 0 {
			r, size := utf8.DecodeRuneInString(text[i:])
			b.WriteString(text[i : i+size])
			prev, i = r, i+size
			continue
		}
		j := i
		for j < len(text) && text[j] == 0 {
			j++
		}
		if prev >= 0 && pycompat.IsWordRune(prev) && j < len(text) {
			if next, _ := utf8.DecodeRuneInString(text[j:]); pycompat.IsWordRune(next) {
				b.WriteByte(' ')
			}
		}
		prev, i = 0, j
	}
	return b.String()
}

// isNavLine is _NAV.match(line): ^(Index\s*\|\s*)?Prev\s*\|\s*Next$ on a
// line without "\n".
func isNavLine(line string) bool {
	if rest, ok := strings.CutPrefix(line, "Index"); ok {
		if rest, ok := cutBar(rest); ok && isPrevNext(rest) {
			return true
		}
	}
	return isPrevNext(line)
}

func isPrevNext(s string) bool {
	rest, ok := strings.CutPrefix(s, "Prev")
	if !ok {
		return false
	}
	rest, ok = cutBar(rest)
	return ok && rest == "Next"
}

// cutBar consumes \s*\|\s*.
func cutBar(s string) (string, bool) {
	s = strings.TrimLeftFunc(s, textfold.IsSpace)
	rest, ok := strings.CutPrefix(s, "|")
	return strings.TrimLeftFunc(rest, textfold.IsSpace), ok
}

// squeezeAfterFence is re.sub(r"```\n\n+", "```\n", text).
func squeezeAfterFence(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "```\n\n") {
			j := i + 5
			for j < len(text) && text[j] == '\n' {
				j++
			}
			b.WriteString("```\n")
			i = j
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	return b.String()
}

// squeezeNewlines is re.sub(r"\n{3,}", "\n\n", text).
func squeezeNewlines(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		if text[i] != '\n' {
			b.WriteByte(text[i])
			i++
			continue
		}
		j := i
		for j < len(text) && text[j] == '\n' {
			j++
		}
		if j-i >= 3 {
			b.WriteString("\n\n")
		} else {
			b.WriteString(text[i:j])
		}
		i = j
	}
	return b.String()
}
