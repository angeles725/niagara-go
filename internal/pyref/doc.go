// Package pyref locates the frozen Python references the parity tests and the
// golden generators compare the Go ports against (decision D47).
//
// Each reference is a worktree of its repository checked out at the tag
// pre-go-retire, the last commit that still carried the Python
// implementation:
//
//	repo              variable                  default
//	niagara-research  NIAGARA_RESEARCH_PY_REF   ~/niagara-research-worktrees/pre-go-retire-niagara-research
//	module-navigator  MODULE_NAVIGATOR_PY_REF   ~/niagara-research-worktrees/pre-go-retire-module-navigator
//	niagara-help      NIAGARA_HELP_PY_REF       ~/niagara-research-worktrees/pre-go-retire-niagara-help
//
// Create one with `git -C <checkout> worktree add --detach <dir> pre-go-retire`.
// The live checkouts are never the Python reference: their tools/ are exec
// shims of the Go binaries, so a comparison against them compares Go with Go.
// The live checkouts stay the DATA (indexes/, organized/, registry.json,
// versions/): a reference supplies only code.
//
// Dir resolves a reference for a test; a missing one skips the test unless
// PYREF_REQUIRED=1 (`make parity` sets it), then it fails. A reference whose
// HEAD is not the tag's commit, or that has local changes or untracked files,
// always fails: it is not the frozen Python. Python finds python3 under the
// same policy, and skips on every OS but Linux even when required: the
// references write CRLF and hit cp1252 encoding errors on Windows, and `make
// parity` runs on Linux. Env returns the escape variable that makes a tag
// script run its Python instead of the exec shim it may carry (niagara-help's
// niagara_help.py does at the tag; module-navigator's module_nav.py is still
// full Python there, and the variable is harmless).
//
// Python generators (testdata/*.py) run standalone with `python3 -I`, so they
// cannot import a shared module: each carries the `py_ref` snippet of
// docs/goldens.md (the same variable and default, the pinned revision and the
// clean-worktree check), sets the escape variable in os.environ (and in any
// subprocess environment it builds from scratch, with NIAGARA_GO_BIN_DIR
// passed through so a NIAGARA_GO_BIN_DIR=/nonexistent run proves no Go port
// answered), and runs a niagara-research tools/<x>.py, which has no escape,
// through the FROZEN runner documented there.
//
// Contract: a leaf package for tests; it imports only the standard library.
// Owner: Integrante2.
package pyref
