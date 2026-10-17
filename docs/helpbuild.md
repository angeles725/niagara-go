# internal/helpbuild: the niagara-help builders in Go

`internal/helpbuild` is the Go port of the niagara-help index builders (Phase 4 of the
Go migration). It is a **non-printing library**: every function returns typed values or
errors and writes nothing to stdout or stderr; the CLI layer (`internal/help`, `cmd/nh`)
decides what to print. Each source file holds one responsibility and stays under 500
lines (the size gate enforces it).

Source of truth: niagara-help `tools/` (Python 3.15). Where the Python code and this
document disagree, the code wins and this document is corrected.

## Slice plan

| Slice | Scope | Python source | Size |
|---|---|---|---|
| **S1** (this slice) | atomic writes, jar-listing cache, install rules + embedded `install_docs.json`, the `doc_types` predicates the builders share | `atomic.py`, `jar_listing.py`, `install_rules.py`, `doc_types.py` (part) | ~650 |
| S2 | doc-jar extract + fidelity (dest paths, mapped entries, fill-missing, sha256 + EOL-only class) | `doc_jars.py` | ~700 |
| S3 | PDF text (`pdftotext -layout -enc UTF-8`, `docs-text/`, `pdf-index.json`) | `pdf_extractor.py` | ~500 |
| S4 | developer course (`course-text/`, `devguide-examples/course/`) | `course_extractor.py`, `course.py` | ~350 |
| S5 | `html_text` (html.parser with `convert_charrefs`) + guides/devguide parsers | `html_text.py`, `guides_parser.py`, `guide_toc.py`, `devguide_parser.py` | ~750 |
| S6 | `guides-index.json` + `devguide-toc.json` | `build_guides_index.py`, `build_devguide_toc.py` | ~350 |
| S7 | bajadoc text (`bajadoc-clean/`) | `bajadoc_parser.py`, `bajadoc_xml_parser.py`, `bajadoc_xml.py` | ~800 |
| S8 | class / inheritance / method indexes | `build_class_index.py`, `inheritance_builder.py`, `method_indexer.py` | ~550 |
| S9 | source / slots / xref / call indexes | `source_indexer.py`, `slots_indexer.py`, `xref_builder.py`, `build_call_index.py` | ~800 |
| S10 | orchestration (`extract`, `versions add`, `rebuild-all`, `freshness`) | `extract_version.py`, `version_setup.py`, `version_report.py`, `maintenance.py` | ~750 |
| (S11) | whole-install sweep gate, if it moves to Go | `install_sweep.py` | — |

`fulltext.sqlite` is outside this plan (owned by the full-text port).

## S1 files

| File | Ports | Notes |
|---|---|---|
| `atomic.go` | `atomic.write_bytes/write_text/write_json/rebuilt_dir` | temp file + rename in the same directory; no fsync (atomic.py has none: the guarantee is against an interrupted builder, not power loss); modes 0644 / 0755 like the explicit `chmod` calls; removing the old tree after a good swap ignores errors like `shutil.rmtree(old, ignore_errors=True)` (the rebuild succeeded; a leftover `.old-<pid>` tree is not reported) |
| `zipdir.go` | `zipfile.ZipFile(path).infolist()` (3.15) | `_EndRecData`, `_EndRecData64`, `_RealGetContents`, `ZipInfo._decodeExtra` ported, see below |
| `jarlisting.go` | `jar_listing.cache_path/read_listing/Listings` | the cache file is byte-identical to Python's |
| `rules.go` | `install_rules.glob_re/Rule/Rules/load/default` | `install_docs.json` embedded with `//go:embed` |
| `doctypes.go` | `doc_types.is_source_archive/is_guide_page/is_licence/clean_text_name` | the rest of `doc_types` arrives with S2/S5 |

### Why a zip reader of its own

`archive/zip` lists the same jars differently from Python in the corners: it keeps raw
name bytes where Python decodes cp437 (no UTF-8 flag), cuts names at a NUL and applies
the Info-ZIP Unicode Path extra field (0x7075); and it counts directory records where
Python walks the directory's byte size. The listing is the contract (names feed every
later classification and the cache), so the reader is a direct port, and a jar is
unreadable in exactly the cases where `read_listing` returns `None`.

### The jar-listing cache

`.cache/jar-listings/<sha1(abspath(install))[:16]>.json`, one per install, written with
`json.dumps(separators=(",", ":"))` (`ensure_ascii` on): `{"version":1,"jars":{<abs jar
path>:{"stamp":[st_size,st_mtime_ns],"listing":{"entries":[[name,size,crc]...],
"classes":n}|null}}}`. The jars object keeps Python's dict order (a re-listed jar keeps its
position, a new one is appended), so Go and Python write the same bytes and each reads the
other's cache without opening a jar.

### The rules data file

`internal/helpbuild/install_docs.json` is a **copy** of niagara-help
`tools/niagara_help_lib/install_docs.json`, kept equal by a drift test
(`TestEmbeddedRulesMatchCanonical`, skipped visibly when the checkout is absent). A copy
was chosen over generation because the established pattern (`internal/glossary`) is a
copy plus a drift test, it needs no `go generate` step or build-time access to the
niagara-research checkout, and the drift test fails on the first commit that forgets to
copy. When the data file changes upstream: copy it, run `go test ./internal/helpbuild/`.

## Parity harness

`testdata/parity/driver.py` imports the real niagara-help modules (`python3 -I -B`: isolated, no `__pycache__` written into the checkout; the tools
directory put on `sys.path` explicitly) and prints JSON lines; `parity_test.go` rebuilds
every line from this package and requires an empty diff. It needs `python3` and a
niagara-help checkout — `$NIAGARA_HELP_DIR`, else the first one met walking up from the
test directory as `<dir>/niagara-help` or `<dir>/niagara-research/niagara-help` (a worktree
under `~/niagara-go-worktrees` finds `~/niagara-research/niagara-help`) — and skips visibly
only when there is none. The drift test `TestEmbeddedRulesMatchCanonical` uses the same
lookup; `$NIAGARA_HELP_DIR` pointing at a folder without the data file fails it.

| Test | Driver mode | Compares |
|---|---|---|
| `TestParitySyntheticJars` | `classify` | per jar the compact listing JSON, per entry: `is_doc_like`, extract rule, exclude rule + reason, the `(course)` exclude rule, `course_kind`, `is_guide_page`, `is_licence`, `is_source_archive("guides/"+entry)`, `clean_text_name(entry, src)` (with a sibling tree) |
| `TestParityRealJars` | `classify` | the same on five EC-Net 4.3.58.18 jars (`docDeveloper-doc` the biggest doc jar, `docLinuxN4-doc`, `alarm-rt`, `workbench-wb`, `batchJob-ux`) plus `ffmpeg-wb.jar` of 4.15.6.20 (the only source archive); `HELPBUILD_PARITY_JARS` replaces the list; skipped with `-short` |
| `TestParityListingCache` | `cache` | a fresh cache written by each side (byte-identical), each side reading the other's cache (0 jars opened), an update of a Python cache by each side (byte-identical, dict order kept) |
| `TestParityCachePath` | `path` | `cache_path` for absolute, relative, `..`, double-slash and non-ASCII install paths |
| `TestParityGlob` | `glob` | 468 pattern/path pairs through `glob_re` |

The synthetic jars mirror the `test_doc_jar_completeness` and `test_install_sweep`
fixtures and add the central-directory corners: duplicate names, directory entries, empty
entries, odd UTF-8 names (İ ı K ſ Σ, CJK, emoji, tab, newline), a raw cp437 name holding
all 128 high bytes, a NUL in a name, Unicode-path extra fields (matching CRC, other CRC,
empty, not UTF-8), a zip64 size, zip64 end records (plain, after prepended data, on
several disks, a record offset past or short of the record), a directory walked by its
byte size whatever the entry count says, corrupt extra fields, prepended data, an archive
comment, and unreadable files (not a zip, empty, truncated, a directory).

## Intended differences

All of them are inputs the Python code does not handle; none changes an output Python
produces.

- **Zip version above 6.3**: Python raises `NotImplementedError`, which `read_listing`
  does not catch (the whole tool stops); `ReadListing` reports the jar unreadable
  (`readListing` returns `ErrUnsupportedZip`). Not in the parity set for that reason.
- **Malformed cache content** (a cache whose top level, `jars` or a jar record is not a
  dict, or a listing of another shape): Python fails with `AttributeError`/`TypeError` on
  first use; Go treats it as a miss (the jar is listed again).
- **`rebuilt_dir` cleanup**: atomic.py leaves the temp tree behind when the `chmod` or the
  rename of the old tree aside fails; `RebuiltDir` removes it.
- **A non-list `unless`, `globs` or `course` value**: Python iterates the characters of a
  string; Go returns an error.
- **A non-list `skip_dirs`, a truthy non-dict `globs` or `course`**: Python fails with
  `TypeError`/`AttributeError` (or, for `globs`, only once an `@name` is looked up); Go
  returns an error when parsing. A falsy value of any type is no data on both sides
  (`data.get(...) or {}`).
- **A negative `classes` count in a cached listing**: Python returns it; Go treats the
  record as damaged and lists the jar again.
- **Empty Unicode-path field**: Python emits a `warnings.warn` on stderr; the library
  prints nothing (it keeps the raw name, as Python does).
- **Interrupts**: Python's `except BaseException` also cleans up on Ctrl-C; a Go process
  killed by a signal runs no deferred cleanup unless the caller handles the signal
  (`signal.NotifyContext`) and returns through the library.
- **Windows**: `os.Rename` is `MoveFileEx(MOVEFILE_REPLACE_EXISTING)`, the call behind
  `os.replace`, so replacing an existing file matches; zip names get `\` → `/` like
  `_sanitize_filename` on Windows; `clean_text_name`'s sibling join uses `filepath` there.
  Parity runs only on Linux.
- **Case folding of non-ASCII pattern literals** follows Go; the shipped patterns are
  ASCII and the one ASCII gap (`i` ~ `İ` `ı` in sre) is handled explicitly.

## Dead code not ported

- `maintenance.ORPHAN_INDEXES` is `[]` (its cleanup loop never runs).
- `maintenance.BUILDERS` `skip_if` is always `None` (the skip branch is unreachable).
- `extract_version --no-index` has no caller.
- `bajadoc_parser.format_output` exists only as a re-export for `bajadoc_xml_parser`; both
  are retired in Phase 5.
- `doc_jars.fidelity_jar(report=None)` default is used only by tests.
