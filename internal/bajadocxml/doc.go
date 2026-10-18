// Package bajadocxml reads Niagara .bajadoc XML (niagara-help
// tools/niagara_help_lib/bajadoc_xml.py): the class and member model
// (ParseFile -> Class, Member), the bajadoc-clean text of a class
// (ToCleanInfo, FormatCleanInfo) and the helpers they share.
//
// Contract: a leaf package. It imports only the standard library and the
// Python-compatibility helpers (internal/pycompat), never internal/help or
// internal/helpbuild, so both the query side (nh: class --jdk, selftest
// --recall) and the builder (helpbuild S7b) can use it. It writes nothing
// to stdout or stderr. Output matches the Python code byte for byte.
//
// Owner: Integrante3. Its behaviour is pinned by the nh goldens
// (internal/help testdata/golden and golden-real) and by the helpbuild
// parity tests (docs/helpbuild.md).
package bajadocxml
