// Package helpbuild holds the niagara-help builder foundations ported from
// niagara-help tools/niagara_help_lib: atomic writes (atomic.go), the
// Python-faithful zip central-directory reader and the jar-listing cache
// (zipdir.go, jarlisting.go), the whole-install sweep rules with the
// embedded install_docs.json (rules.go) and the doc_types predicates the
// builders share (doctypes.go).
//
// It is a non-printing library: every function returns typed values or
// errors and writes nothing to stdout or stderr. Behaviour matches the
// Python code byte for byte where its output is observable (listing JSON,
// cache files, rule and predicate results); parity_test.go runs the Python
// twin (testdata/parity/driver.py) and requires empty diffs. See
// docs/helpbuild.md for the slice plan and the intended differences.
package helpbuild
