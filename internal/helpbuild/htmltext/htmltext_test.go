package htmltext

import (
	"strings"
	"testing"
)

// Ported from niagara-help tests/test_text_extraction.py (TestHtmlText,
// TestImplicitTableEnds); the method names are cited per case.

// pyTestPage is test_text_extraction.PAGE.
const pyTestPage = `<html><head><title></title><style>p {}</style></head><body>
<p>Index | Prev | Next</p>
<h1>Web &amp; servlets</h1>
<p>Niagara&rsquo;s servlet &#8212; see <b>BWebServlet</b>.</p>
<pre>public void doGet(WebOp op)
{
    op.getHtmlWriter().w("&lt;html&gt;");
}</pre>
<table><tr><th>Property</th><th>Value</th></tr>
<tr><td>Port</td><td>80 | 443</td></tr></table>
<p><img src="graphics/servlet.png" alt="Servlet view"/></p>
</body></html>`

func TestHTMLToTextPythonPage(t *testing.T) {
	title, text := HTMLToText(pyTestPage)
	// test_title_falls_back_to_h1
	if title != "Web & servlets" {
		t.Errorf("title = %q", title)
	}
	for _, want := range []string{
		"Niagara’s servlet — see BWebServlet.",                                               // test_entities_are_decoded
		"```\npublic void doGet(WebOp op)\n{\n    op.getHtmlWriter().w(\"<html>\");\n}\n```", // test_pre_is_fenced_with_indentation
		"| Property | Value |\n| Port | 80 \\| 443 |",                                        // test_table_rows
		"[image: Servlet view -> graphics/servlet.png]",                                      // test_image_marker
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(strings.ReplaceAll(text, "Web & servlets", ""), "&") {
		t.Errorf("an entity is left: %s", text) // test_entities_are_decoded
	}
	if strings.Contains(text, "Prev") {
		t.Errorf("navigation line kept: %s", text) // test_navigation_line_dropped
	}
}

func TestHTMLToTextCases(t *testing.T) {
	for _, c := range []struct{ name, page, want string }{
		{"test_inline_tags_do_not_glue_words", "<p>by expanding<b>Chart</b> palette</p>", "by expanding Chart palette"},
		{"test_unclosed_cells_and_rows", "<table><tr><th>Name<th>Type\n<tr><td>port<td>int\n<tr><td>host<td>String</table>",
			"| Name | Type |\n| port | int |\n| host | String |"},
		{"nested table (test_nested_table_keeps_its_text)",
			"<table><tr><td>obj</td><td>the literal<table><tr><td>a</td><td>b</td></tr></table> end</td></tr></table>",
			"| obj | the literal a / b end |"},
		{"test_cell_open_at_end_of_page", "<p>before</p><table><tr><td>last cell text", "before\n\n| last cell text |"},
		{"edge between word characters only", "a<b>-</b>b<i>c</i>", "a-b c"},
		{"script, style and head are skipped", "<head>h</head><script>s</script><style>p{}</style>body", "body"},
		{"raw text keeps tags", "<script><p>x</script>after", "after"},
		{"list items", "<ul><li>one<li>two</ul>", "- one\n- two"},
		{"image alt only", `<img alt=" A ">`, "[image: A]"},
		{"image src only", `<img src='b.png'>`, "[image: b.png]"},
		{"blank lines after a fence are dropped", "<pre>\n\n\nx</pre>\n\n\n\ny", "```\nx\n```\ny"},
		{"unknown character reference", "&bogus; &amp;", "&bogus; &"},
		{"bare < is text", "a < b", "a < b"},
		{"unterminated tag at the end is dropped", "text <a href='x", "text"},
		{"unterminated comment at the end is dropped", "text <!-- c", "text"},
		{"a bare </ at the end is text", "text </", "text </"},
	} {
		if _, got := HTMLToText(c.page); got != c.want {
			t.Errorf("%s: HTMLToText(%q) = %q, want %q", c.name, c.page, got, c.want)
		}
	}
}

func TestHTMLToTextTitle(t *testing.T) {
	for _, c := range []struct{ page, want string }{
		{"<title> A  &amp;\nB </title><h1>H</h1>", "A & B"},
		{"<title><b>raw</b></title>", "<b>raw</b>"}, // RCDATA: no tags inside
		{"<h1>First</h1><h1>Second</h1>", "First"},
		{"<h1>&#1;</h1><h1>Second</h1>", ""}, // an empty chunk still claims the first <h1>
		{"<p>none</p>", ""},
	} {
		if got, _ := HTMLToText(c.page); got != c.want {
			t.Errorf("title of %q = %q, want %q", c.page, got, c.want)
		}
	}
}

func TestIsNavLine(t *testing.T) {
	for line, want := range map[string]bool{
		"Index | Prev | Next": true, "Prev|Next": true, "Index\u00a0|\u2028Prev | Next": true,
		"Prev | Next more": false, "Index Prev | Next": false, "Index |": false, "": false,
	} {
		if got := isNavLine(line); got != want {
			t.Errorf("isNavLine(%q) = %v", line, got)
		}
	}
}

func TestResolveEdges(t *testing.T) {
	for in, want := range map[string]string{
		"a\x00b": "a b", "a\x00\x00b": "a b", "a\x00 b": "a b", "\x00a": "a", "a\x00": "a",
		"é\x00日": "é 日", "a\x00-\x00b": "a-b", "_\x00²": "_ ²",
	} {
		if got := resolveEdges(in); got != want {
			t.Errorf("resolveEdges(%q) = %q, want %q", in, got, want)
		}
	}
}
