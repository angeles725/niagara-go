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
// The backends stay the Python tools: a Backend runs one of them and types its
// answer (Raw). execBackend runs `<python> <script> <args...>` exactly as the
// Python front door does; an in-process Backend can replace it per source once
// the Go ports export library subpackages (decision D29).
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
// (one process answers one question); `--version` describes the Go build (D18).
package searchdoor
