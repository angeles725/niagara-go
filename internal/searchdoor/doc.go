// Package searchdoor is tools/nsearch.py and its alias tools/find3.py (the
// searchdoor package): one question to the three niagara-research sources at
// once, each answering with ranked rows, an honest zero with its scope line,
// or a typed UNAVAILABLE line (never a silent zero).
//
//	blocks  our research blocks   tools/corpus-nav.py find
//	docs    official docs         niagara-help/tools/niagara_help.py find
//	code    decompiled code       module-navigator/tools/module_nav.py token/rank
//	                              (grep when rank cannot search, flagged DEGRADED),
//	                              plus the structure sub-scope (hierarchy,
//	                              implementors, drivers) and the course-code
//	                              sub-scope (in-process BM25 over sources/n4-dev-course)
//
// `license <module|feature>` joins the licensing evidence of the three sources.
// Exit: 0 a source returned rows, 1 every source searched and none matched,
// 2 a source was unavailable and none returned rows, 3 usage.
//
// A Backend runs one backend call and types its answer (Raw). execBackend runs
// `<python> <script> <args...>` exactly as the Python front door does. By
// default inprocBackend answers in process, through the libraries the Go ports
// share (decision D29), the calls it can reproduce byte for byte: blocks
// (corpusnav/blocksearch), docs (help/fts) and the code source's token and grep
// (nav/codesearch) and rank (nav/classrank); it renders the exact text the
// backend CLI prints, so the parsers see the same rows. Everything else runs
// through exec: the structure, modules and license calls, and any call whose
// data is absent, whose full-text index is missing or stale (D21) or whose
// niagara_help miss block belongs to a printing package (or whose rank xref
// index is absent); a call that falls back after trying in process gives exec
// only the budget it has left. NSEARCH_EXEC=1 forces exec for every call; it is
// the parity oracle. The in-process blocks source keeps its own find cache,
// find-<scope>.nsearch.gob, beside corpus-nav's; rank shares module_nav's Go
// cache (indexes/.mnav-cache/class-rank.gob).
//
// Locating the backends (decision D24 style; a binary has no source path):
//
//	root    $NSEARCH_ROOT, else the first directory holding tools/corpus-nav.py
//	        walking up from the executable's directory (symlinks resolved), else
//	        from the working directory, else the executable directory's parent
//	        (the Python layout: the binary installed in <root>/tools/)
//	blocks  $NSEARCH_CORPUS_NAV    else <root>/tools/corpus-nav.py
//	docs    $NSEARCH_NIAGARA_HELP  else <root>/niagara-help/tools/niagara_help.py
//	code    $NSEARCH_MODULE_NAV    else <root>/module-navigator/tools/module_nav.py
//	course  $NSEARCH_COURSE_ROOT   else <root>/sources/n4-dev-course
//	python  $NSEARCH_PYTHON        else python3, else python, from PATH
//	exec    $NSEARCH_EXEC=1        every call through exec (no in-process answer)
//
// The four NSEARCH_* path variables are the same seam the Python front door
// reads (searchdoor/backend.py); an empty value keeps the default. The hermetic
// goldens under testdata/ point them at fake backend scripts.
//
// Intended differences from Python (none golden-covered): the backends run
// under $NSEARCH_PYTHON / PATH's python3 instead of the interpreter running
// nsearch.py, and a missing interpreter is that source's UNAVAILABLE reason; a
// panic inside one source is typed UNAVAILABLE as "panic: <value>" where Python
// named the exception type; the course index is not cached between questions
// (one process answers one question); `--version` describes the Go build (D18);
// an in-process call past its --timeout budget is typed timed out like a killed
// process, but the library work is not interrupted (it ends with the process).
package searchdoor
