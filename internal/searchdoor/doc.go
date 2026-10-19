// Package searchdoor is tools/nsearch.py and its alias tools/find3.py (the
// searchdoor package): one question to the three niagara-research sources at
// once, each answering with ranked rows, an honest zero with its scope line,
// or a typed UNAVAILABLE line (never a silent zero).
//
//	blocks  our research blocks   corpus-nav find
//	docs    official docs         niagara_help find
//	code    decompiled code       module_nav token/rank
//	                              (grep when rank cannot search, flagged DEGRADED),
//	                              plus the structure sub-scope (hierarchy,
//	                              implementors, drivers) and the course-code
//	                              sub-scope (in-process BM25 over sources/n4-dev-course)
//
// A query that names a course answer-key item (L10.23, LB.7, RA1.V12, D1.8, T1.1,
// X3, P2: courseitem.go) is led by that item, as `course show <ID>` prints it
// (parsed by course/key, decision D41), before the three sources; a token that
// names no item is ordinary query text.
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
// Locating the backends (decision D24 style; a binary has no source path). No
// backend path is run and none needs a Python shim: each one names its tool's
// checkout, either that directory itself (the defaults) or a script inside it
// (the parent of the script's tools/ folder, as the Python tools find theirs from
// __file__). A path that is neither a directory nor a file is that source's
// "tool not found".
//
//	root    $NSEARCH_ROOT, else the first niagara-research checkout
//	        (locate.ResearchCheckout: CATALOG.md and a bloque file, or the legacy
//	        tools/corpus-nav.py) walking up from the executable's directory
//	        (symlinks resolved), else from the working directory, else the
//	        executable directory's parent (the Python layout: the binary
//	        installed in <root>/tools/)
//	blocks  $NSEARCH_CORPUS_NAV    else <root>
//	docs    $NSEARCH_NIAGARA_HELP  else <root>/niagara-help
//	code    $NSEARCH_MODULE_NAV    else <root>/module-navigator
//	course  $NSEARCH_COURSE_ROOT   else <root>/sources/n4-dev-course
//	keys    $NSEARCH_COURSE_KEYS   else <root> (holds docs/n4-dev-course-solutions)
//
// The first four NSEARCH_* path variables are the variables the Python front door
// read (searchdoor/backend.py), and a script path in them keeps working;
// NSEARCH_COURSE_KEYS is Go only. An empty value
// keeps the default. The root search is locate.Find (decision D44). Every
// variable this package reads (these, and the sources' CORPUS_NAV_ROOT,
// CORPUS_NAV_INDEX_DIR, NIAGARA_HELP_DIR, the docs version and NAV_CORPUS_BASE)
// goes through the command's cli.Stdio.Env, the process environment under
// cli.Run; the libraries the sources call (blocksearch's CORPUS_NAV_FIND_INDEX,
// help/scope's cache directory, nav.RunAt) still read the process. The hermetic
// goldens under testdata/ point the NSEARCH_* paths at fake backend scripts, which the tests
// answer with their Go port (scriptFakes).
//
// Intended differences from Python (none golden-covered): no backend runs as a
// process, so $NSEARCH_PYTHON and $NSEARCH_EXEC are gone; the defaults name the
// checkout directories, not the scripts, so a checkout without the shims still
// answers, and a backend path that exists but is neither a regular file nor a
// directory is "tool not found"; a panic inside one
// source is typed UNAVAILABLE as "panic: <value>" where Python named the
// exception type; the course index is not cached between questions (one process
// answers one question); `--version` describes the Go build (D18); an in-process
// token miss leaves out module_nav's "Similar tokens:" line (a full scan of the
// postings; the code source reads only the miss state); a call past its
// --timeout budget is typed timed out like a killed process, but the library
// work (a fulltext rebuild included, which is atomic) is not interrupted: it ends
// with the process; a query that starts with "-" is searched as text (the Python
// backends' argparse read it as an option and exited 2); a call the door never
// makes is typed UNAVAILABLE "no in-process answer"; the course item lead (D41) is
// Go only, so a query naming an item ID prints one more section and Summary entry.
package searchdoor
