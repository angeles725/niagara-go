// Package helpbuild holds the niagara-help builder foundations ported from
// niagara-help tools/niagara_help_lib: atomic writes (atomic.go), the
// Python-faithful zip central-directory reader and the jar-listing cache
// (zipdir.go, jarlisting.go), the whole-install sweep rules with the
// embedded install_docs.json (rules.go), the doc_types predicates the
// builders share (doctypes.go) and, from S2, the doc-jar extraction:
// entry reading like ZipFile.open (zipread.go), the jar-entry mapping
// (docjars.go), extract / fill-missing / fidelity (extract.go) and the raw
// steps of version_setup.extract_and_index with the version manifest
// (versionsetup.go, manifest.go). Then the builders: PDF text and the
// developer course (S3, S4: pdftext.go, pdfindex.go, course.go), the
// guides/devguide text and their indexes (S5, S6: htmltext.go,
// guidesparser.go, guidesindex.go, devguidetoc.go), bajadoc-clean/ from the
// javadoc pages and the .bajadoc XML (S7a, S7b: bajadocrun.go,
// bajadocxmlrun.go, with internal/bajadocxml) and the class, inheritance
// and method indexes (S8: classindex.go, inheritance.go, methodindex.go).
//
// It is a non-printing library: every function returns typed values or
// errors and writes nothing to stdout or stderr. Behaviour matches the
// Python code byte for byte where its output is observable (listing JSON,
// cache files, rule and predicate results); the *frozen_test.go tests replay
// the output of the retired Python twin, frozen under testdata/frozen, and
// require empty diffs. See
// docs/helpbuild.md for the slice plan and the intended differences.
package helpbuild
