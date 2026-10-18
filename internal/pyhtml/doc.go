// Package pyhtml is Python's html module family as the niagara-help tools use it
// (decision D37; owner Integrante3): the tokenizer of CPython 3.15's
// html.parser.HTMLParser with convert_charrefs=True (Parse: one feed() of the whole
// page, then close()), html.unescape with the full WHATWG html5 table (Unescape) and
// `name in html.entities.html5` (IsHTML5Entity).
//
// Contract: for valid UTF-8 input, the callbacks a Handler receives (start tag,
// start-end tag, end tag, data, in order, with Python's chunk boundaries) and
// Unescape's result are the ones CPython 3.15 produces, byte for byte. Comments,
// declarations, processing instructions and CDATA sections are dropped, as the base
// class's empty handlers drop them. Invalid UTF-8 never panics or hangs (FuzzParse)
// but is outside the contract: the tools decode pages with errors="replace" first.
// The package depends on textfold only (str.lower, str.isspace), so pycompat can
// forward its HTMLUnescape here.
//
// Fixtures: the expected events of parser_test.go and the values of unescape_test.go
// were taken from CPython 3.15 by hand (python3 -c with html.parser / html.unescape).
// The live parity checks run the interpreter and live in internal/helpbuild, next to
// their driver (testdata/parity/htmldriver.py): TestParityHTMLCorners and
// TestParityHTMLFuzz (callbacks over hand-written pages and 5,000 seeded random
// snippets) and TestParityHTML5Entities (every html5 name, its near misses and
// html.unescape over 46,851 texts). See docs/helpbuild.md, "The html.parser port".
package pyhtml
