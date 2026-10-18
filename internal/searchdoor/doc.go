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
// A Backend runs one backend call and types its answer (Raw). inprocBackend
// answers every call in process; nothing runs Python. Blocks, docs and the code
// source's token, grep and rank go through the libraries the Go ports share
// (decision D29), byte for byte what the backend CLI prints, so the parsers see
// the same rows: blocks (corpusnav/blocksearch), docs (help/fts and help/scope:
// a missing or stale fulltext.sqlite is rebuilt first, atomically, with nh's
// notices, decision D38; a miss or an unresolvable version prints nh's own
// lines), token and grep (nav/codesearch) and rank (nav/classrank). Every other
// module_nav call (structure, drivers and its --help probe, modules,
// license-feature(s)) and every token/grep/rank case those fast paths leave runs
// mnav's own command through nav.RunAt on the script's checkout (decision D42).
// The in-process blocks source keeps its own find cache,
// find-<scope>.nsearch.gob, beside corpus-nav's; rank shares module_nav's Go
// cache (indexes/.mnav-cache/class-rank.gob).
//
// Locating the backends (decision D24 style; a binary has no source path). The
// script paths are not run: each one names its tool's checkout (the parent of
// its tools/ folder), as the Python tools find theirs from __file__, and a path
// that is not a file is that source's "tool not found".
//
//	root    $NSEARCH_ROOT, else the first directory holding tools/corpus-nav.py
//	        walking up from the executable's directory (symlinks resolved), else
//	        from the working directory, else the executable directory's parent
//	        (the Python layout: the binary installed in <root>/tools/)
//	blocks  $NSEARCH_CORPUS_NAV    else <root>/tools/corpus-nav.py
//	docs    $NSEARCH_NIAGARA_HELP  else <root>/niagara-help/tools/niagara_help.py
//	code    $NSEARCH_MODULE_NAV    else <root>/module-navigator/tools/module_nav.py
//	course  $NSEARCH_COURSE_ROOT   else <root>/sources/n4-dev-course
//
// The four NSEARCH_* path variables are the same seam the Python front door
// reads (searchdoor/backend.py); an empty value keeps the default. The hermetic
// goldens under testdata/ point them at fake backend scripts, which the tests
// answer with their Go port (scriptFakes).
//
// Intended differences from Python (none golden-covered): no backend runs as a
// process, so $NSEARCH_PYTHON and $NSEARCH_EXEC are gone and a script path that
// exists but is not a regular file is "tool not found"; a panic inside one
// source is typed UNAVAILABLE as "panic: <value>" where Python named the
// exception type; the course index is not cached between questions (one process
// answers one question); `--version` describes the Go build (D18); an in-process
// token miss leaves out module_nav's "Similar tokens:" line (a full scan of the
// postings; the code source reads only the miss state); a call past its
// --timeout budget is typed timed out like a killed process, but the library
// work (a fulltext rebuild included, which is atomic) is not interrupted: it ends
// with the process; a query that starts with "-" is searched as text (the Python
// backends' argparse read it as an option and exited 2); a call the door never
// makes is typed UNAVAILABLE "no in-process answer".
package searchdoor
