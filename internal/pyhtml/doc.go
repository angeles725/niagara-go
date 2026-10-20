// Package pyhtml is Python's html module family as the niagara-help tools use it
// (decision D37; owner Integrante3): the tokenizer of CPython 3.15's
// html.parser.HTMLParser with convert_charrefs=True (Parse: one feed() of the whole
// page, then close(); Feed: one feed() alone, as bajadoc_parser.strip_tags calls it),
// html.unescape with the full WHATWG html5 table (Unescape) and
// `name in html.entities.html5` (IsHTML5Entity).
//
// Contract: for valid UTF-8 input, the callbacks a Handler receives (start tag,
// start-end tag, end tag, data, in order, with Python's chunk boundaries) and
// Unescape's result are the ones CPython 3.15 produces, byte for byte. Comments,
// declarations, processing instructions and CDATA sections are dropped, as the base
// class's empty handlers drop them. Invalid UTF-8 never panics or hangs (FuzzParse)
// but is outside the contract: the tools decode pages with errors="replace" first.
// The package depends on textfold only (str.lower, str.isspace); callers use
// Unescape directly.
//
// Fixtures: the expected events of parser_test.go and the values of unescape_test.go
// were taken from CPython 3.15 by hand (python3 -c with html.parser / html.unescape).
// The frozen parity checks live in internal/helpbuild (htmlfrozen_test.go), replaying the
// output of the retired Python driver: TestHTMLCornersFrozen (callbacks over hand-written
// pages), TestHTMLFeedCornersFrozen (the same for Feed) and TestHTML5EntitiesFrozen (every
// html5 name, its near misses and html.unescape around each). See docs/helpbuild.md, "The
// html.parser port".
package pyhtml
