// Package bog reads Niagara station BOG files and parses them into a component graph.
//
// It is the Go port of niagara-research tools/lib/bogio.py (ReadBytes, ReadText, ReadError) and
// tools/bognav/model.py (Parse, Load, Bog, Comp, Resolve, LinkRow), behaviour-identical to them:
// testdata/gen_fixtures.py regenerates the parity fixtures from the Python originals.
//
// Reading detects the input by content (a config.bog ZIP, a .dist backup with a nested
// config.bog, or plain BOG XML) and fails with a *ReadError carrying bogio's message, which
// callers map to exit 3 (ExitBadInput, D17). Parsing is the same tolerant line-oriented tag
// reader as model.py, Unicode semantics of Python's re included, so a truncated document yields a
// smaller graph, never an error. A caller that needs well-formedness (bog-cycles, bog-write)
// checks it separately.
//
// Nested composite values (opt-in, Go-only). Like model.py, Parse keeps a composite property (a
// non-self-closing <p> without a handle, such as a setpoint StatusNumeric) as a Slot, with only
// its direct value and status children in Slot.Child; anything deeper, and any other direct child,
// is dropped. ParseOptions and LoadOptions with Options{NestedSlots: true} also fill Comp.Nested
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
