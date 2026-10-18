// Package navbuild is the Go port of the module-navigator index builders
// (module-navigator tools/build_*.py, Phase 4 of the Go migration, D33).
//
// It is a non-printing library: every builder returns typed results and
// errors and writes only its index file; the summaries the Python builders
// print belong to the CLI layer (`mnav reindex`, a later slice). Only the
// indexes kept commands read are ported (D32): module-inventory, class,
// inheritance, xref, token, method, field and callgraph.
//
// Source of truth: module-navigator tools/ (Python 3.15). Where the Python
// code and this package disagree, the code wins. Each index is written with
// the same bytes as Python except the volatile _meta members (timestamp,
// build_time_sec and the absolute source path; see NormalizeVolatile), which
// the parity tests normalise before a byte diff. See docs/navbuild.md.
package navbuild
