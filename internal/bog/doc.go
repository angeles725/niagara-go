// Package bog reads Niagara station BOG files and parses them into a component graph.
//
// It is the Go port of niagara-research tools/lib/bogio.py (ReadBytes, ReadText, ReadError) and
// tools/bognav/model.py (Parse, Load, Bog, Comp, Resolve, LinkRow), behaviour-identical to them:
// the fixtures under testdata/ hold the Python originals' output, frozen when D47 retired their
// generator (gen_fixtures.py); they change only from reviewed Go output.
//
// Reading detects the input by content (a config.bog ZIP, a .dist backup with a nested
// config.bog, or plain BOG XML) and fails with a *ReadError carrying bogio's message, which
// callers map to exit 3 (ExitBadInput, D17). Parsing is the same tolerant line-oriented tag
// reader as model.py, Unicode semantics of Python's re included, so Parse on a truncated text
// yields a smaller graph. Load fails closed instead (R8-T29): it reads with ReadCheckedText, whose
// CheckWellFormed turns a truncated or damaged document into a *ReadError
// "<path>: not well-formed XML (<expat message>: line L, column C)". Its verdict and message are
// bog-cycles' (internal/pyxml Mode{}, expat without namespaces; #170): a fast single pass (about
// 20 ms on the 7.1 MB HARBOR file.xml) accepts a text that is certainly well-formed, and anything
// else (a fault, a DOCTYPE, a non-ASCII name) is decided and worded by pyxml. bog-write keeps
// pyxml's ElementTree mode for namespaces.
//
// Attribute values are XML-decoded (UnescapeXML: &amp; &lt; &gt; &quot; &apos; &#N; &#xN;), a
// deliberate divergence from model.py, which kept the raw text ("u:fahrenheit;&#xb0;F"). A
// malformed or unknown reference is kept as written. Names are XML-decoded before the SlotPath
// $-decoding of UnescapeName.
//
// Nested composite values (opt-in, Go-only). Like model.py, Parse keeps a composite property (a
// non-self-closing <p> without a handle, such as a setpoint StatusNumeric) as a Slot, with only
// its direct value and status children in Slot.Child; anything deeper, and any other direct child,
// is dropped. LoadOptions (and parseOptions) with Options{NestedSlots: true} also fill Comp.Nested
// with every value stored at any depth below a non-platform composite slot, keyed by the dot path
// of decoded names from the component: tcpConfig.address.port -> "4911". Flags-only children,
// links and handle components inside the property are not entries; a name containing '.' is not
// escaped. The rest of the graph is identical with or without the option, so Parse and Load
// output (and every tool golden built on them) is unchanged.
//
// No parse cache. model.py caches the parsed graph on disk (tools/lib/bogcache.py) because the
// Python parse of the largest available station (HARBOR HM_BMS backup: 7.1 MB of XML, 36,132
// components, 844 links) takes about 7.9 s. Measured on 2026-10-17 (WSL2, Go 1.27.2), Load of the
// same file takes 100-120 ms in a fresh process and 145-265 ms per call when repeated in one
// process (ReadText about 21 ms, Parse about 105 ms), under the 300 ms threshold that would
// justify a cache, so NIAGARA_BOG_CACHE_MIN_BYTES and NIAGARA_CACHE_DIR are not read here.
// Re-measure with: BOG_BENCH_FILE=<bog> go test -run '^$' -bench . ./internal/bog/
package bog
