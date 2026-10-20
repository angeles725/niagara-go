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
| **S1** (done) | atomic writes, jar-listing cache, install rules + embedded `install_docs.json`, the `doc_types` predicates the builders share | `atomic.py`, `jar_listing.py`, `install_rules.py`, `doc_types.py` (part) | ~650 |
| **S2** (done) | doc-jar extract + fidelity (dest paths, mapped entries, fill-missing, sha256 + EOL-only class), the raw steps of `extract_and_index`, `jar_manifest`, the version manifest, `_empty_indexes` | `doc_jars.py`, `version_setup.py` (part), `extract_version.py` (as library options) | ~1,500 with tests |
| **S3** (done) | PDF text (`pdftotext -layout -enc UTF-8`, `docs-text/`, `pdf-index.json`) | `pdf_extractor.py` (+ the `os`/`shutil`/`json` and registry helpers S4 shares) | ~1,970 with tests |
| **S4** (done) | developer course (`course-text/`, `devguide-examples/course/`); `gate()` waits for S10 (it reads the full-text index) | `course_extractor.py`, `course.py` (part) | ~670 with tests |
| **S5** (done) | `html_text` (html.parser with `convert_charrefs`) + guides/devguide parsers | `html_text.py`, `guides_parser.py`, `guide_toc.py`, `devguide_parser.py` | ~2,100 with tests |
| **S6** (done) | `guides-index.json` + `devguide-toc.json` | `build_guides_index.py`, `build_devguide_toc.py` | ~930 with tests |
| **S7a** (done) | bajadoc text of the javadoc HTML pages (`bajadoc-clean/`) | `bajadoc_parser.py` (+ `bajadoc_xml.format_output`, `write_clean_class`, `doc_types.clean_class_name`) | ~1,000 with tests |
| **S7b** (done) | bajadoc text from `.bajadoc` XML for the classes without an HTML page; the XML reader moved to the shared leaf `internal/bajadocxml` | `bajadoc_xml_parser.py`, `bajadoc_xml.py` (rest) | ~900 with tests |
| **S8** (done) | class / inheritance / method indexes | `build_class_index.py`, `inheritance_builder.py`, `method_indexer.py` (+ the class attributes of `bajadoc_xml.parse_file`) | ~1,900 with tests |
| **S9** (done) | source / slots / xref indexes; `call-index.json` is retired (decision I3-5), not ported | `source_indexer.py`, `slots_indexer.py`, `xref_builder.py` | ~2,000 with tests |
| **S10a** (done) | `rebuild-all` in process (plan, `--only`, `--dry-run`, `--stamp-only`, the corpus manifest), the `build.log` hook of `extract_and_index` | `maintenance.py` (`BUILDERS`, `cmd_rebuild_all`, the fingerprint), `version_setup._rebuild` | ~1,000 with tests |
| S10 | orchestration (`extract`, `versions add`, `freshness`) and the nh wiring of `rebuild-all` | `extract_version.py`, `version_setup.py`, `version_report.py`, `maintenance.py` (rest) | ~750 |
| **S10b-2** (done) | `versions add` (detect, register, extract and index, both gates, the summary, the rollback) and the nh wiring of `versions add` and `versions extract` | `version_setup.py` (rest), `version_report.py`, `extract_version.py` (CLI) | ~1,300 with tests |
| **S10b-1** (done) | the coverage reports: `coverage`, `coverage <module>`, `coverage --doc-jars` (library + printers; the nh wiring is a later slice) | `coverage_report.py`, `doc_coverage.py` (+ `doc_types.classify`, `classify_unmapped`, `content_type`, `example_skip_reason`) | ~1,300 + tests |
| **S11a** (done) | whole-install sweep gate (`coverage --sweep`, a `versions add` gate) as a library + report, on S10b-1's `Classify` and `Indexed`; nh wiring is a later slice | `install_sweep.py`, `course.gate` / `print_gate` | ~1,700 with tests |
| **S11** (done) | nh wiring: `coverage [module]`, `--doc-jars`, `--sweep`, and `rebuild-all` under the build lock (`internal/help/cmds_maint.go`, `internal/help/scope/buildlock.go`) | `cli/cmd_export.py` `run_coverage`, `cli/cmd_admin.py` `run_rebuild_all` | ~600 with tests |

`fulltext.sqlite` is outside this plan (owned by the full-text port).

## S1 files

| File | Ports | Notes |
|---|---|---|
| `internal/atomicfile` (`WriteBytes`, `WriteText`, `RebuiltDir`) and `pyfs/jsonwrite.go` (`WriteJSON`) | `atomic.write_bytes/write_text/write_json/rebuilt_dir` | `atomicfile` is a leaf package (no module imports) so other commands can share it; temp file + rename in the same directory; no fsync (atomic.py has none: the guarantee is against an interrupted builder, not power loss); modes 0644 / 0755 like the explicit `chmod` calls; removing the old tree after a good swap ignores errors like `shutil.rmtree(old, ignore_errors=True)` (the rebuild succeeded; a leftover `.old-<pid>` tree is not reported) |
| `zipfile/zipdir.go` | `zipfile.ZipFile(path).infolist()` (3.15) | `_EndRecData`, `_EndRecData64`, `_RealGetContents`, `ZipInfo._decodeExtra` ported, see below |
| `jarlisting.go` | `jar_listing.cache_path/read_listing/Listings` | the cache file is byte-identical to Python's |
| `rules.go` | `install_rules.glob_re/Rule/Rules/load/default` | `install_docs.json` embedded with `//go:embed` |
| `doctypes.go` | `doc_types.is_source_archive/is_guide_page/is_licence/clean_text_name` | the rest of `doc_types` arrives with S2/S5 |

## S2 files

| File | Ports | Notes |
|---|---|---|
| `zipfile/zipread.go` | `ZipFile.open(info)` + `ZipExtFile.read` | local-header checks in Python's order, inflate, cut at `file_size`, CRC; `whole` = `read()`, else `copyfileobj`'s bounded reads |
| `docjars.go` | `is_doc_jar` … `mapped_entries`, `help_jars`, `list_doc_jars` | the mapping table in the file comment; `*Rules` must be non-nil (`Default()` is Python's `None`) |
| `extract.go` | `extract`, `fill_missing`, `fidelity_jar`, `fidelity` | |
| `versionsetup.go`, `manifest.go` | `extract_and_index` steps 1, 3, 4, `jar_manifest`, the manifest dict, `_empty_indexes` | non-printing: the `out()` lines are `SetupResult.Log` |

API for S10 (the CLI wiring of `extract_version.py`):

```go
res, err := helpbuild.ExtractAndIndex(repo, vid, helpbuild.SetupOptions{
	Force: force, VerifyOnly: verifyOnly, FillMissing: fillMissing,
	Rebuild: runRebuildAll, // writes root/build.log, returns its SKIP/FAIL/Ran lines
})
// print res.Log line by line; res.Error != "" -> "ERROR: " + res.Error, exit max(res.Status, 2);
// err != nil is what Python raises (traceback); else exit res.Status.
```

`Rebuild` is `_rebuild` minus the parts the library keeps: `ExtractAndIndex` writes the
`indexes/pdf-index.json` stub before the hook and the `{}` empty indexes after it. A nil
hook skips the step, which is `extract_and_index(no_index=True)`; that is what
`TestExtractFrozen` runs, since the index builders are later slices. `extract_version --no-index`
has no caller and is not ported as a flag.

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

`internal/helpbuild/install_docs.json` is the **canonical** rules data file (D44). It began
as a byte-for-byte copy of niagara-help `tools/niagara_help_lib/install_docs.json`, kept equal
by a drift test; with the Python tools retired the Go copy is the only one maintained, so the
drift test is gone and a rule changes here, followed by `go test ./internal/helpbuild/`. The
frozen expectations below are Python's output under the tag's rules, so a rule changed here
changes the ones it touches on purpose: regenerate them from Go and name the change in the commit.

## Frozen expectations

Until D47 the tests here compared this package with the niagara-help Python reference (the
`pre-go-retire` worktree) through driver scripts under `testdata/`. The drivers are gone. Each
synthetic comparison whose output is deterministic became a Go-only test against
`testdata/frozen/<area>/` (`frozen_test.go`); every frozen file was checked byte-identical to
the driver output of the same fixture before the drivers were deleted, so it is Python's
output. Paths are relabeled (`<base>`, `<root>`, `<dir>`; any other temporary folder shows as
`<tmp>`), trees are `treeLines` listings (or, per step, the `treeDelta` / `lineDelta` of the
listing), and the frozen tests run on Linux only, as the parity tests did. A deliberate change
is regenerated from Go and reviewed:

```sh
go test ./internal/helpbuild -run <Test> -update-frozen
```

Dropped with the drivers: the real-data parity runs and the cases whose output was not
deterministic (fuzz snippets drawn by Python's RNG, listing-cache bytes that hold jar mtimes,
the file system's directory order, text that depends on the pdftotext build).
`TestCoverageRealGoldens` (S10b-1) is the one real-data check left: it needs no Python.

| Test | Checks |
|---|---|
| `TestClassifyFrozen` | per jar the compact listing JSON, per entry: `is_doc_like`, extract rule, exclude rule + reason, the `(course)` exclude rule, `course_kind`, `is_guide_page`, `is_licence`, `is_source_archive("guides/"+entry)`, `clean_text_name(entry, src)` (with a sibling tree) |
| `TestCachePathFrozen` | `cache_path` for absolute, relative, `..`, double-slash, non-ASCII, `.` and UNC install paths (run in `/`, so the relative ones hash the same paths everywhere) |
| `TestGlobFrozen` | 468 pattern/path pairs through `glob_re` |

S2 adds (a Go error is `["raise"]`, where Python raised):

| Test | Checks |
|---|---|
| `TestExtractFrozen` | `extract_and_index` result lines and the repo tree delta (path, kind, permission bits, size, sha256; manifests inline with `generated_utc` normalized; the jar-listing cache entry masked) after each of: install not mounted, repo-root refusal, verify-only over the repo root, extract, already-extracted refusal, verify after mismatched / EOL-only / missing edits, fill-missing, force |
| `TestDocJarCornersFrozen` | `extract`, `fill_missing`, `fidelity` (also over what extract left) and the trees (or `<absent>`) on twelve one-jar installs: bad CRC, local name ≠ central name, encrypted, unknown method, overlapped entries, a shared local header, `file_size` shorter than the data, cp437 and NUL names, prepended data, a zip64 end record, not a zip, a folder named like a jar |
| `TestDocJarMappingFrozen` | `is_doc_jar` … `guide_folder`, `dest_path`, `is_help_entry`, `mapped_entries` over every synthetic jar plus a grid of 27 jar kinds × 32 entry shapes (one line per jar); `help_jars` / `list_doc_jars` of the synthetic install |
| `TestEmptyIndexesFrozen` | `_empty_indexes` lines and tree |

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
  (`readListing` returns `ErrUnsupportedZip`). Not in the frozen set for that reason.
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
  The frozen tests run only on Linux.
- **Case folding of non-ASCII pattern literals** follows Go; the shipped patterns are
  ASCII and the one ASCII gap (`i` ~ `İ` `ı` in sre) is handled explicitly.

S2 additions (none changes an output Python produces from a well-formed install):

- **bzip2, lzma and zstd entries** (methods 12, 14, 93): Python inflates them; Go returns
  `ErrUnsupportedMethod`. Niagara jars use stored and deflated only.
- **A deflate stream that ends early** while every compressed byte is present: zlib's
  `flush()` hands Python the decoded prefix, then the CRC decides; Go's `flate` may hold
  back the tail of the last block, so a prefix whose CRC would match fails. A short file
  is an error on both sides.
- **Extract's bounded reads**: `copyfileobj` stops at `file_size`; the lookahead zlib does
  past that point inside one chunk is not modelled (Go stops exactly at `file_size`).
  `read()` (fidelity, fill-missing) inflates everything on both sides.
- **A failed entry read during extract** leaves the empty file Python's single-chunk copy
  leaves; for an entry above one copy chunk Python leaves the chunks already written.
- **A mapped path that could leave the data root**: a `..` segment (split on both `/`
  and `\`), a leading `/` or `\`, or a drive letter (`C:`, `C:x`, `C:\x`). Python writes
  wherever `os.path.join` points, also outside the data root, and on Linux writes a
  name such as `..\x` verbatim as one file; `Extract` and `FillMissing` return
  `ErrUnsafeEntry` on every OS, so the same jar is refused wherever the corpus is built
  (safety over parity). The joined path is also checked to stay under the root. A `\`
  in an otherwise safe name (`a\b.txt`) is kept: one file with that name on Linux, as
  Python writes it, the folder `a` and the file `b.txt` on Windows. Fidelity only reads
  and keeps Python's join.
- **A data root that is the repository under another spelling**: Python refuses only
  the literal `data_root` values `.` and `""`, so `./`, `x/..`, the absolute repo path or
  a symlink to the repository would make a `--force` run remove the raw and derived
  folders of the repository itself. Go refuses with the same message whenever the data
  root is the repository once cleaned, made absolute and (when it exists) followed
  through symlinks (`os.SameFile`).
- **A data root above the repository** (`..`, an absolute parent): a `--force` run there
  removes the fixed raw and derived folder names, which deletes the repository when its
  folder has one of them. Go refuses with `refusing to extract over a folder that holds the
  repo; use --verify-only`; Python goes ahead.
- **Registry entries**: a non-string `install_dir`, `data_root` or `label` is an error
  before any work (Python fails later or copies a non-string label); a missing `label`
  is an error up front where Python raises `KeyError` after extraction and fidelity.
- **The overlapped-entries warning** (`warnings.warn`, a duplicate sharing one local
  header) is not printed; the entry is read as in Python.

## S3 and S4 files

| File | Ports | Notes |
|---|---|---|
| `pdftext.go` | `classify`, `sanitize`, `top_folder`, `find_pdfs`, `fix_unmapped_bullets`, `extract_text`, `resolve_name` | one pdftotext run: argv list, no shell, `PDFTimeout` (120 s); a failure is a `*PDFError` whose `Failure` says which of the four cases `extract_text` catches (exit status, timeout, output not UTF-8, could not start); the run goes through `internal/procgroup` (own process group, SIGINT/SIGTERM relay), shared with `internal/secaudit` |
| `pdfindex.go` | `pdf_extractor.main()` | `ExtractPDFs`; the early exits, the `-v` probe, keep-last-good, the index save, the prune after a clean run, the summary |
| `registry.go` | `versions.identify`, `versions.install_dir`, `course.course_dir` | read with pyjson so every malformed entry takes Python's branch: skipped where Python catches, an error where it raises |
| `pyfs/pyfs.go` | `os.walk`, `os.remove`, `shutil.copyfile`, `json.load` | `Walk` lists a symlinked folder without entering it and skips unreadable ones; `RemoveFile` refuses a directory; `Copyfile` raises `SameFileError` when source and destination are the same file, refuses a named pipe before opening it and a directory as source (`EISDIR`, Python's `IsADirectoryError`) or destination (the open fails); `LoadJSON` returns a `*JSONLoadError` (Python's `str(e)`) for what the builders catch |
| `course.go` | `course.course_dir`, `_files`, `_text_dest`, `plan`, `remove_copies`, `build`, `course_extractor.main()` | both trees staged with `RebuiltDir`, nested like Python's `with` (devguide-examples/course/ swaps first) |

`SOFTWARE_HIGH` of `pdf_extractor.py` is never read and is not ported.

API for S10 (`rebuild-all` runs both tools as builders):

```go
res, err := helpbuild.ExtractPDFs(root, helpbuild.PDFOptions{
	DocsDir: docsDir,          // --docs-dir; "" = <install_dir>/docs of the registered version
	Emit:    func(e string) {}, // optional live output; an entry starting with "\r" is a progress write
})
// os.Stdout.WriteString(res.Stdout()) is pdf_extractor's stdout; exit res.Status;
// err != nil is a Python traceback (exit 1), res.Log holds what was printed before it.

out, err := helpbuild.CourseExtractor(root, rules) // rules from helpbuild.Default()
// print out.Stdout to stdout and out.Stderr to stderr, exit out.Status; err != nil is a traceback.
```

`Pdftotext` (`"pdftotext"`) is resolved on PATH when each run starts, as `subprocess.run`
does without a shell; `PDFOptions.Pdftotext` names another program. The tests put a fake
`pdftotext` first on PATH (`fakeBin`) that hands every real PDF to the real binary.

S3/S4 frozen tests:

| Test | Checks |
|---|---|
| `TestPDFIndexStatesFrozen` | stdout (repository relabeled, the `Time:` value normalized), exit status and the repo tree delta after each state that converts no PDF: unregistered root (SKIP, docs dir `None`), unregistered root with manuals, corrupt index (bad JSON, not an object, not UTF-8) with nothing to rebuild from, install not mounted, a docs dir with no PDF, pdftotext not on PATH. The conversions (two pages with an unmapped bullet, a corrupt PDF, output that is not UTF-8, a timeout, keep-last-good, pruning) are `pdfindex_test.go`'s, with the fake pdftotext |
| `TestCoursePlanFrozen` | every `plan` row plus the `(course)` exclude rule id over the test_course_source fixture and 18 more files (nested text, upper-case extension, dev readme.md, .js and doc pages, key material, solutions, PDFs, non-ASCII names, a symlinked folder) |
| `TestCourseExtractorFrozen` | stdout, stderr, exit status and the repo tree delta: bound by a relative and an absolute `course_dir`, unbound (earlier copies removed; a symlink in their place refused), `course_dir` missing, unregistered root, rebuilt after the course changed, a copy that fails (mode 000 file), a named pipe in the course, a registry that is not JSON |

S3/S4 intended differences (none changes an output Python produces):

- **The pdftotext child runs in its own process group** and the timeout kills the group;
  `subprocess.run` kills only the direct child (a wrapper that forks would leave the real
  program running). The group does not get the terminal's Ctrl-C, so while it runs
  SIGINT and SIGTERM to the extractor are relayed: the group is killed, then the signal
  is raised again so the process dies of it (130/143), as Python dies of an uncaught
  `KeyboardInterrupt`; a caller that catches the signal itself gets the interrupt back
  from `ExtractPDFs` (no index saved, nothing pruned). On SIGTERM Python dies at once
  and leaves pdftotext running; Go kills it. On Windows the direct child is killed and
  the console's Ctrl-C reaches it, as in Python.
- **A pdftotext run that succeeds as its deadline passes** keeps its output; it is a
  timeout only when the run failed and the deadline is past (`subprocess.run` raises
  `TimeoutExpired` only while it is still waiting).
- **Keep-last-good matches the source PDF too**: a failed PDF keeps its previous text and
  index entry only when that entry's `source_pdf` is this PDF. Python matches the stem
  only, so once `a b.pdf` is removed, a failing `a_b.pdf` that takes its stem `a_b`
  would be indexed with the removed manual's text (safety over parity). The stale
  `.txt` stays until the next clean run prunes it, as in Python.
- **Page count**: pdftotext 26.10.0 ends **every** page with a form feed, the last one
  too, so `extract_text`'s `count('\f') + 1` reports one page more than the PDF has (its
  comment says the last page has none; niagara-help keeps it, team decision D34). Go
  counts `count('\f') + (0 if text.endswith('\f') else 1)`, right for both pdftotext
  behaviours; text that is only whitespace is 0 pages on both sides. Measured: 4.14
  `MtgWiring/12997(LON)-C.pdf` is 4 pages in `pdfinfo`, 5 in Python's index, 4 in Go's;
  on EC-Net 4.3.58.18 every one of the 66 manuals drops by exactly 1 (3407 → 3341). The
  parity tests compared Python's `pdf-index.json` with `pages` lowered by one wherever
  the entry's `docs-text/<stem>.txt` ends with a form feed; the frozen files hold Go's counts.
- **stdin** of pdftotext is the null device; Python lets it inherit stdin. pdftotext never
  reads it with `-` as the output file. stderr is dropped on both sides.
- **`PDFOptions.DocsDir` ""** means "not given" (registry lookup); `--docs-dir ''` in Python
  skips the lookup and prints `''` where the unset value prints `None`.
- **Registry shapes**: where Python raises `AttributeError`/`TypeError` (a top level that is
  not an object, a `versions` value that is a number, a non-string `install_dir` or
  `course_dir`), Go returns an error; where Python catches the error (a version without a
  string `data_root`) both skip the entry.
- **`os.path.normpath` keeps a leading `//`**, `filepath.Clean` does not; registry paths
  never start with two slashes.
- **File names that are not UTF-8** (Python's surrogateescape) are not modelled; the tests
  run on Linux with UTF-8 names.

Found while porting (the Python code wins):

- A test shortens the 120 s timeout only for its timeout fixture (a PDF with a
  `<pdf>.sleep` side file) through the `pdfTimeout` hook; the parity driver did the same
  with `HB_PDF_TIMEOUT`. Every other PDF keeps 120 s.

## Shared-helper candidates

`pyAbspath`, `pyBasename`, `rmtreeIgnore` (`shutil.rmtree(ignore_errors=True)`) and
`commas` (`format(n, ",")`) are local to this package; `pyAbspath`/`pyBasename` belong
next to `pycompat.Join` once a second package needs them. `os.path.join`,
`os.path.isdir` and `os.path.isfile` already come from `pycompat` (`Join`, `IsDir`,
`IsFile`).

S8 adds one: `pyTextLines` (`inheritread.go`, a text file read line by line with
TextIOWrapper's chunked strict decoding) belongs in `pycompat` next to `ReadText` once a
second package needs Python's decode-error text. The class index reads `.bajadoc` files
through `bajadocxml.Parse`, as S7b does; S7b's sequential walk (`bajadocXMLClasses`) could
take `parseBajadocFiles` (`classindexxml.go`) for the same concurrency.

## S5 and S6 files

| File | Ports | Notes |
|---|---|---|
| `internal/pyhtml` (shared package, D37) | `html.parser.HTMLParser` (CPython 3.15), `convert_charrefs=True`; `html.unescape`; `name in html.entities.html5` | the tokenizer below (`pyhtml.Parse`, a `pyhtml.Handler`); reports only the callbacks the tools override |
| `htmltext.go` | `html_text.convert` as `HTMLToText(page) (title, text)` | Python's Unicode `\s`, `strip` and `\w` (`textfold.IsSpace`, `pycompat.IsWordRune`) |
| `guidetoc.go` | `guide_toc.find_toc`, `parse_toc`, `count_items` | ElementTree is `internal/pyxml` in ETree mode; the warning is returned, not printed |
| `guidesparser.go` | `guides_parser.parse_guides`, `_markdown_title`, `devguide_parser.parse_devguide` | `RebuiltDir`; `TextResult.Log` = stdout lines, `.Warnings` = stderr lines |
| `guidesindex.go` | `build_guides_index` (`topic_name`, `known_jar_names`, `modules_dir_of`, `jar_for_folder`, `previous_jars`, `build`, `main`) | `JarSources` replaces the `_MANIFEST` sentinels; `Unknown` is `jar_names=None`; the walk, `json.load` and registry lookups are S3's (`pyfs/pyfs.go`, `registry.go`) |
| `devguidetoc.go` | `build_devguide_toc.build`, `main` | its regexes run through `internal/pyre` (Python semantics, code-point offsets) |

API for S10 (`maintenance.BUILDERS` runs these as subprocesses; the Go orchestration calls
them in process and prints `Log`, then `Warnings` on stderr):

```go
res, err := helpbuild.ParseGuides(root)        // guides-clean/
res, err := helpbuild.ParseDevguide(root)      // devguide-clean/
res, err := helpbuild.BuildGuidesIndex(root)   // indexes/guides-index.json
res, err := helpbuild.BuildDevguideTOC(root)   // indexes/devguide-toc.json
// err != nil is where Python raises (traceback, non-zero exit).
```

Both indexes are `json.dumps(indent=2)` with `ensure_ascii` on (`DumpIndent2`).

### The html.parser port

The port lives in the shared `internal/pyhtml` package (decision D37, owner Integrante3;
its `doc.go` holds the contract), which depends on `textfold` only so that
`pycompat.HTMLUnescape` can forward to it. Its frozen tests stay here
(`htmlfrozen_test.go`).

One `feed()` of the whole page and `close()` parse exactly like one `goahead(end=1)`: each
early stop of the feed pass leaves the state unchanged and `close()` resumes there, and
the text after the last `<` is handed over whole either way, so the `&` look-back that
waits for a cut reference changes nothing. With `convert_charrefs` the `&` branches of
`goahead` are unreachable; data is unescaped chunk by chunk except in the raw-text
elements (`script`, `style`, `xmp`, `iframe`, `noembed`, `noframes`, `plaintext`;
`title` and `textarea` are RCDATA, unescaped). Chunk boundaries matter (`html_text`
collapses whitespace per chunk) and are the same: data before a tag, a malformed start tag
reported as text, a lone `<`, a trailing `</`. Unterminated constructs at the end of the
page are dropped (`<a href='x`, `</a`, `<!--`), as in Python.

`pyhtml.Feed` is one `feed()` alone (`goahead(end=0)`), which bajadoc's `strip_tags` uses
(it never calls `close()`): what would wait for more input is dropped, that is an
unterminated construct or a lone `<` at the end with everything after it, raw text
without its end tag, and the data after the last `<` when an `&` in its last 34 code
points has no `[\t\n\r\f ;]` after it (the window counts code points, as Python's
`n-34` does).

`locatetagend`, `tagfind_tolerant` and `attrfind_tolerant` are hand-ported: every part
after the tag name is optional, so sre's first path is the greedy one, except the value
indicator, which backtracks one space before a quote that is never closed (`x= 'y` is
`x=""` plus an attribute `'y`). Tag and attribute names use Python's `str.lower`; the
CDATA end tag folds ASCII letters only (`re.ASCII`). Offsets are byte offsets into valid
UTF-8: every delimiter is ASCII and every class excludes ASCII bytes only.

`html.unescape` is `pyhtml.Unescape` (`pycompat.HTMLUnescape` forwards to it): Go's
`html` entity table plus `&nGt;` and `&nLt;`, the two WHATWG names it lacks (found by the
parity test over all 2,231 names of `html.entities.html5`, now `TestHTML5EntitiesFrozen`). The attribute rule
(`_unescape_attrvalue`) needs `name in html5`; Go does not export its table, so
`pyhtml.IsHTML5Entity` reads it through `html.UnescapeString` (an exact name decodes to 1 or 2 code points, a prefix match leaves the rest of the name).

### Frozen tests

| Test | Checks |
|---|---|
| `TestHTMLCornersFrozen` | the parser callbacks and `convert()` of 60 hand-written pages (the Python tests' fixtures, unterminated constructs, raw text, attribute quoting, charrefs); `FuzzHTMLToText` fuzzes `convert()` natively (the parity's 5,000 random snippets were drawn by Python's RNG and are gone) |
| `TestHTMLFeedCornersFrozen` | `pyhtml.Feed` (one `feed()` without `close()`, bajadoc's `strip_tags`) on the corner pages plus held-back ends (cut references inside and outside the 34-code-point look-back, raw text without its end tag, a lone `<`) |
| `TestHTML5EntitiesFrozen` | `pyhtml.IsHTML5Entity` on every name of CPython 3.15's `html.entities.html5` and its near misses, `pyhtml.Unescape` of `a&NAME b`, `&NAME=` and `&NAME&NAME` against `html.unescape` (`testdata/frozen/html/entities.txt`, one line per name) |
| `TestTocFilesFrozen` | `parse_toc` (order, titles, trails), `count_items` and the stderr warning on the synthetic toc.xml files and corners (namespaced, root `tocitem`, empty, undefined entity, BOM + ISO-8859-1 / US-ASCII / UTF-8 / UTF-16 (one real 4.3/4.15 toc.xml is BOM + ISO-8859-1; `pyxml` applies the declared encoding after a BOM as expat does), junk, missing file) |
| `TestGuidesFrozen` | stdout/stderr lines, return value and the tree delta after each builder (and a second run) on a synthetic root: the `test_extraction` pages, toc.xml corners, Markdown, `.txt` beside `.html`, licence texts, a non-UTF-8 page body, CRLF, a broken toc, `TOC.XML` |
| `TestGuidesIndexSourcesFrozen` | `guides-index.json` of a registered root with a manifest and a mounted install, an unmounted install, and no manifest (values kept from an earlier index) |
| `TestGuidesNonUTF8NamesFrozen` | page and folder names that are not UTF-8 |

### S5/S6 intended differences

- **A page or folder name that is not UTF-8**: `os.walk` decodes it with
  surrogateescape and `write_text` raises `UnicodeEncodeError` on the header, so
  `parse_guides` / `parse_devguide` fail and the old clean tree is kept. Go returns an
  error at the same point (same outcome, an error instead of a traceback);
  `guides-index.json` spells such names as Python does (`\udcXX`, sorted as decoded
  names). A licence text with such a name is printed raw in the `excluded` line. On
  Windows the only such name is an unpaired surrogate (UTF-16 names; Go reads it as
  WTF-8, CPython as `\udcXX`), and both sides fail the same way.
- **The `excluded (licence text)` line** is `os.path.relpath` unchanged, so it keeps the
  host separator (`devguide\sub\lgpl-2.1.txt` on Windows) while `SOURCE:` headers use `/`.

## S7a files

| File | Ports | Notes |
|---|---|---|
| `bajadoc.go` | `parse_bajadoc_html` and its `extract_*` / `parse_*_summary` helpers, `strip_tags`, `normalize_ws` | the regexes are Python's, run by `internal/pyre` on code points (Unicode `\s` `\S` `\w` `\b`, lazy quantifiers, backtracking); `_find_summary_section` searches the end markers from 10 code points past the marker; `strip_tags` is `pyhtml.Feed` (one `feed()`, never `close()`) |
| `bajadocrun.go` | `bajadoc_parser.main`, `bajadoc_xml.write_clean_class`, `doc_types.clean_class_name` (`format_output` is `bajadocxml.FormatCleanInfo`, see S7b) | the walk skips the index pages and the files of a folder whose path holds `/class-use/` (so `class-use/` itself is read, its pages skipped as no class); pages are read with `errors="replace"` and universal newlines; `bajadoc-clean/` is written in place with atomic writes (it also holds what S7b writes), a page left under its escaped name (`Cursor&lt;E&gt;.txt`) is removed |

API for S10 (`maintenance.BUILDERS` runs `bajadoc_parser.py <data root>`):

```go
res, err := helpbuild.ParseBajadoc(root)
// print res.Log line by line, exit res.Status (1: bajadoc/javax/baja missing);
// err != nil is a Python traceback (exit 1), res.Log holds what was printed before it.
```

The `Done in %.1fs` line is the real elapsed time; the tests show it as `Done in <t>s` (as
S3 does with `Time:`).

S7a tests:

| Test | Checks |
|---|---|
| `TestParseBajadocFixture` | `ParseBajadoc` on `testdata/bajadoc/root/` (classes, an interface with subinterfaces, an enum, an annotation type, a generic class name, the `<title>` fallback, skipped pages, `class-use/` folders, no package, CRLF and bytes that are not UTF-8, a 78-code-point wrap, member rows whose fragments `feed()` holds back) against the golden `testdata/bajadoc/clean/` and `stdout.txt` (Python's output, frozen) |
| `TestBajadocCornersFrozen` | stdout lines, exit status and the whole tree after an unreadable page, a dangling link, links to a page and to a folder, a page under its escaped name, an unrelated S7b page, a second run, and a root without `bajadoc/javax/baja` (exit 1) |

Measured 2026-10-17 by the parity run (whole trees identical, Python 3.15 vs Go, wall time of
one run):

| Version | Pages parsed | Files written (bytes) | Python | Go |
|---|---|---|---|---|
| 4.14.0.162 (data root `.`) | 2,761 (none skipped) | 2,759 (2,953,078): two classes are under both `webChart/ux/` and `webchart/ux/` with one package name, the later page wins on both sides | 5.4 s | 5.0 s |
| 4.15.6.20 | 2,839 | 2,839 (3,129,292) | 7.2 s | 5.4 s |
| 4.13.2.18 | 2,714 | 2,714 (2,886,701) | 12.8 s | 5.5 s |

### S7a intended differences

- **A class name that would put its page outside `bajadoc-clean/`** (absolute, or `..`
  segments that climb above it): Python writes wherever `os.path.join` points;
  `ParseBajadoc` stops with `ErrUnsafeClassName` before writing (safety over parity, as
  S2's `ErrUnsafeEntry`). Javadoc class names hold no `/`.
- **Page names that are not UTF-8** (Python's surrogateescape, sorted as decoded names) are
  not modelled; the tests run on Linux with UTF-8 names.

## S7b files

The `.bajadoc` reader is `internal/bajadocxml` (owner Integrante3): a leaf package (standard
library and `internal/pycompat` only, never `internal/help` or `internal/helpbuild`) that nh's
`class --jdk` and `selftest --recall` read the model from and S7b renders pages with. It
moved there unchanged from `internal/help/bajadocxml.go`; its `doc.go` holds the contract.

| File | Ports | Notes |
|---|---|---|
| `internal/bajadocxml/bajadocxml.go` | `bajadoc_xml.render_type`, `parse_class`, `_sanitize`, `parse_file` (`ParseFile`; `Parse` on bytes already read), `_simple`, `to_clean_info`, `format_output` | `encoding/xml` builds an ElementTree-like tree (an element's text is the character data before its first child; comments and processing instructions dropped); the declared encoding is decoded as expat does for UTF-8, ISO-8859-1 and US-ASCII (a `CharsetReader`); a document that does not parse is parsed again with the character references XML forbids replaced by `?`; `CleanInfo.Subclasses` is the caller's to fill, `FormatCleanInfo` prints `SUBCLASSES:` when it is not empty |
| `bajadocxmlrun.go` | `bajadoc_xml_parser.build`, `main`, `has_html_page`, `bajadoc_xml.iter_classes` | walks `devguide/` then `guides/` (os.walk order, links to folders not entered, links to files read), skips `module-index.bajadoc`, a class already seen (the first walked wins) and a class whose `bajadoc/<package path>/<Name>.html` is a file; `SUBCLASSES` lists the classes whose simplified `extends` is the class name, so a `com.vendor.BBase` superclass (kept qualified by `_simple`) or a parameterized one never matches; every page is written after the whole walk with S7a's `writeCleanClass` |

API for S10 (`maintenance.BUILDERS` runs `bajadoc_xml_parser.py <data root>` after
`bajadoc_parser.py`):

```go
res, err := helpbuild.ParseBajadocXML(root)
// print res.Log (one line); err != nil is a Python traceback (exit 1): a .bajadoc file
// that cannot be read (ET.parse raises the OSError, nothing is written) or a failed write.
```

S7b tests:

| Test | Checks |
|---|---|
| `TestParseBajadocXMLFixture` | `ParseBajadocXML` on `testdata/bajadocxml/root/` (a class with every member kind, varargs, parameterized, wildcard and type-variable types, markup, a comment and CDATA in a description, two `@since` tags, two `<implements>` elements of which only the first is read; an enum, an annotation type, an interface, a generic name, no package; a class with a javadoc page; a class in both trees and in two devguide modules; an index file; an empty and an unclosed file; a forbidden character reference; CRLF; files that are not `.bajadoc`) against the golden `testdata/bajadocxml/clean/` and `stdout.txt` (Python's output, frozen) |
| `TestBajadocXMLCornersFrozen` | stdout lines, exit status and the whole tree after a page under its escaped name, S7a's page of the class with a javadoc page, links to a file and to a folder, a second run, a dangling link and an unreadable file (both raise), and a root without `devguide/` or `guides/` |

Measured 2026-10-17 by the parity run (S7a then S7b; whole `bajadoc-clean/` trees identical,
Python 3.15 vs Go, wall time of one run of each step):

| Version | S7a pages | S7b classes | Files (bytes) | S7a Python / Go | S7b Python / Go |
|---|---|---|---|---|---|
| 4.3.58.18 (XML only: S7a prints its ERROR line, exit 1, on both sides) | — | 5,104 | 5,104 (3,417,247) | 0.0 / 0.0 s | 2.1 / 1.6 s |
| 4.14.0.162 (data root `.`) | 2,761 | 3,551 | 6,310 (5,629,497) | 2.5 / 3.3 s | 4.5 / 3.0 s |
| 4.15.6.20 | 2,839 | 2,840 | 5,679 (4,623,322) | 3.7 / 2.7 s | 2.1 / 1.8 s |
| 4.13.2.18 (`HELPBUILD_PARITY_BAJADOCXML`) | 2,714 | 2,796 | 5,510 (4,355,293) | 7.7 / 4.4 s | 2.5 / 1.7 s |

### S7b intended differences

- **A class name that would put its page outside `bajadoc-clean/`**: `ErrUnsafeClassName`,
  as in S7a.
- **Encodings other than UTF-8, ISO-8859-1 and US-ASCII** (`encoding="windows-1252"`,
  `"latin1"`): expat hands them to Python's codecs, which decode any single-byte codec and
  raise `LookupError` for an unknown name; `encoding/xml` has no reader for them, so such a
  file counts as unparseable. Every `.bajadoc` of the four registered versions declares
  UTF-8.

## S8 files

| File | Ports | Notes |
|---|---|---|
| `classindex.go` | `build_class_index` (`html_entries`, `xml_entries`, `_rank`, `merge`, `build`, `build_jdk`, `count_classes`, `_write`, `main`) | the `_LI` regex runs through `internal/pyre`; `allclasses-noframe.html` is read with `errors="replace"` and universal newlines; `IndexResult` is the result type of the three builders |
| `classindexxml.go` | `bajadoc_xml.iter_classes`; `parse_file` is `bajadocxml.Parse` (S7b's shared reader) | the file is read here, so one that cannot be read is the error `ET.parse` raises; one that stays unparseable after sanitizing is a `ParseFailures` entry (`PARSE_FAILURES`); the files are parsed on half the CPUs and taken in walk order |
| `inheritance.go` | `inheritance_builder.parse_txt_file`, `main`, `verify_samples` | the header ends at the first empty line; `sort_keys=True` is done by building the object in sorted order (same bytes) |
| `inheritread.go` | `for line in open(p, encoding="utf-8")` | `pyTextLines`: strict UTF-8 decoded 8,192 bytes at a time as TextIOWrapper does, universal newlines (a `\r` that ends a chunk waits for the next one), stopping when the caller breaks; a decode error is raised when its chunk is read and names the position inside that chunk, so `inheritance.json`'s error lines match (an invalid byte after the header in the same chunk is an error, one in a later chunk is not) |
| `methodindex.go` | `method_indexer.parse_members`, `build_method_index` | `_METHOD_NAME` (`[^(]*?(\w+)\s*\(`) is hand-ported: the name is the whole `\w` run before the first `(` (whitespace allowed in between); `\w`, `\s`, `strip()` and `split()` are Python's Unicode classes (`pycompat.IsWordRune`, `IsSpace`) |

API for S10 (`maintenance.BUILDERS` runs the three scripts with the data root as argument):

```go
res, err := helpbuild.BuildClassIndex(root)  // indexes/class-index.json (+ jdk-class-index.json when jdk/ exists)
res, err := helpbuild.BuildInheritance(root) // indexes/inheritance.json; res.Status 1 without bajadoc-clean/
res, err := helpbuild.BuildMethodIndex(root) // indexes/method-index.json
// print res.Log line by line, exit res.Status; err != nil is a Python traceback (exit 1),
// res.Log holds what was printed before it.
```

`class-index.json` and `jdk-class-index.json` are `json.dumps(indent=2)` (ASCII);
`inheritance.json` adds `sort_keys=True, ensure_ascii=False`; `method-index.json` is
`indent=2, ensure_ascii=False` in first-seen key order (`DumpIndent2Unicode`).

S8 tests:

| Test | Checks |
|---|---|
| `TestBuildClassIndexFixture`, `TestBuildInheritanceFixture`, `TestBuildMethodIndexFixture` | each Go builder on `testdata/s8/root/` (a javadoc list with spans, CRLF, a byte that is not UTF-8, a duplicate and a non-class title; `.bajadoc` classes of every kind, modules with and without a runtime profile, a class without a qualified name, nested and namespaced classes, a file fixed by sanitizing, one that stays broken, Latin-1, shared simple names; bajadoc-clean pages with every header line, CRLF and lone CR, a BOM, Unicode whitespace, duplicate names, member sections) against `testdata/s8/golden/` (stdout lines and indexes, Python's output; `-update-s8` rewrites them from Go) |
| `TestS8CornersFrozen` | stdout lines, exit status and `indexes/` after each builder, then the whole tree, on the corners (decode errors in, after and past the header, across a chunk end, a `\r` at a chunk end, a truncated end, a line longer than a chunk, symlinked folders and files) and on roots without the folders each builder reads |
| `TestS8UnreadableFrozen` | stdout lines and exit status with an unreadable `.bajadoc`, an unreadable page and a dangling `.bajadoc` link (class and method index raise, inheritance lists the page under `Errors`) |
| `TestS8UnreadableInputKeepsThePreviousIndexes` | an unreadable `.bajadoc` (class-index) or `bajadoc-clean/` page (method-index) is an error and the previous index files stay as they were, no temp file left |

Measured 2026-10-17 by the parity run (every file byte-identical, builder time only, CPython
3.15 vs Go, a shared machine):

| Version | `class-index.json` | `jdk-class-index.json` | `inheritance.json` | `method-index.json` | class: Python / Go | inherit | method |
|---|---|---|---|---|---|---|---|
| 4.14.0.162 | 1,268,139 | 721,539 | 1,134,565 | 17,784,122 | 4.2 s / 1.2 s | 0.30 / 0.20 s | 1.9 / 2.0 s |
| 4.15.6.20 | 1,127,995 | 721,539 | 1,044,591 | 12,767,069 | 8.8 s / 1.3 s | 0.21 / 0.17 s | 0.71 / 0.78 s |
| 4.3.58.18 | 1,070,984 | 696,449 | 905,081 | 10,185,670 | 4.6 s / 1.1 s | 0.15 / 0.18 s | 0.80 / 0.51 s |

The Go indexes of 4.14.0.162, 4.15.6.20, 4.13.2.18 and 4.3.58.18 are also byte-identical to
the committed `indexes/` of niagara-help, and `internal/help`'s `TestGoldenReal` passes on a
copy of the corpus whose four indexes were rebuilt by Go, except `freshness` and
`freshness-all`, which fail the same way on a copy that only links the original files (the
baseline manifests are recorded for the real path).

`method-index.json` time is mostly `pyjson.Dumps` (about 1.2 s of 17.8 MB); an ASCII fast
path in `pyjson.EncodeString` is the place to win it back.

### S8 intended differences

- **The `.bajadoc` files are parsed concurrently**: after an unreadable file Python stops,
  Go has also read the files after it; the entries, `ParseFailures` and the error are the
  walk-order ones, so nothing printed or written changes.
- **Encoding declarations** other than UTF-8, ISO-8859-1 and US-ASCII follow
  `bajadocxml` (see the S7b intended differences): such a file is a parse failure. Every
  one of the 27,804 `.bajadoc` declarations in the corpus is UTF-8.

## S9 files

| File | Ports | Notes |
|---|---|---|
| `javasrc.go` | `source_indexer.java_package`, `java_declaration`, `iter_java_roots`, `RC_DIR` | `_PACKAGE` and `_DECLARATION` run through `internal/pyre` (Unicode `\s` `\w`, the lookahead and its backtracking); `.java` files are read with `errors="replace"` (`pycompat.DecodeReplace`) and universal newlines; a root without `source/` gets the warning line on stderr (`IndexResult.Stderr`) |
| `pyfs.go` | `os.walk` | `pyWalkDirOrder` lists each folder in the file system's order (`os.scandir`), for the scripts that do not sort what `os.walk` yields |
| `sourceindex.go` | `source_indexer.index_source` | folders sorted, files in directory order (the script sorts only `dirs`): that order fixes the key order and which same-name file gets the plain key; a later same-name file is `Name (module)`, overwriting an earlier one of that key in place; the package is matched on the raw line (an indented `package` does not count); `path` is `os.path.relpath` from `source/` with `\` turned into `/` |
| `slotsparse.go` | `slots_indexer.annotation_bodies`, `_skip_literal_or_comment`, `_scan`, `_strip_comments`, `_split_top_level`, `_parse_annotation_body` | code points, with Python's slice clamping (an unclosed literal or group runs past the end; a closer that does not match ends the group one code point early, as `content[start:end - 1]` does); `@Name\s*\(` matches anywhere, comments included, as in Python |
| `slotsindex.go` | `slots_indexer.parse_annotations`, `collision_key`, `build_slots_index`, `merge_bajadoc_slots`, `_xml_slot` | the module folders sorted, everything under them in directory order (the script sorts nothing there); the class line search uses `str.splitlines` boundaries and stripped lines (an indented `package` counts here); a source class without a package has no `package` key and never matches a `.bajadoc` class (Python's `None` against `""`); the `.bajadoc` classes come from S8's `bajadocWalk` and `parseBajadocFiles` (devguide/ then guides/), the types from `bajadocxml.Simple` |
| `xrefindex.go` | `xref_builder.extract_imports`, `build_xref` | sorted walk; every file read before the imports are filtered, so the graph does not depend on the walk order; the package is the last `package` line before the declaration (`None` when it does not match); `str.isupper()` of the first code point is `pycompat.IsUpper` |

API for S10 (`maintenance.BUILDERS` runs the three scripts with the data root as argument):

```go
res, err := helpbuild.BuildSourceIndex(root) // indexes/source-index.json
res, err := helpbuild.BuildSlotsIndex(root)  // indexes/slots-index.json; err without source/
res, err := helpbuild.BuildXrefIndex(root)   // indexes/xref-index.json
// print res.Log on stdout and res.Stderr on stderr line by line, exit res.Status (always 0);
// err != nil is a Python traceback (exit 1), res.Log holds what was printed before it.
```

The three files are `json.dumps(indent=2, ensure_ascii=False)` in insertion order
(`DumpIndent2Unicode`); xref's keys are sorted by construction.

S9 tests:

| Test | Checks |
|---|---|
| `TestBuildSourceIndexFixture`, `TestBuildSlotsIndexFixture`, `TestBuildXrefIndexFixture` | each Go builder on `testdata/s9/root/` (multi-line annotations, nested parentheses, literals holding commas, quotes and `=`, comments in and around annotations, unnamed, empty and repeated keys, a closer that does not match, an annotation inside a comment, a package split over two lines, an indented package, a default package, `X.java.java`, non-ASCII names, static, wildcard and self imports, `.bajadoc` slots with and without a source entry, module or package) against `testdata/s9/golden/`, byte for byte: the fixture holds at most one `.java` per folder and one annotated file per module, so its output does not depend on the file system's directory order (`checkS9Fixture` fails otherwise); `-update-s9` rewrites them from Go |
| `TestS9CornersFrozen` | lines, exit status and `indexes/` after each builder, then the whole tree, on the corners written at test time (CRLF, a lone CR, a form feed, U+2028, bytes that are not UTF-8, a literal left open by a final backslash, a `\` in a file name, a linked module, a linked folder inside a module, linked and dangling `.java` files), then on roots without `source/`, without `devguide-rc/`, a `source` that is a file, and an import ending with `.` (xref raises). The parity's order-dependent corners (one class in three folders of a module, five copies with the `#n` warning) cannot be frozen and are gone |
| `TestS9UnreadableFrozen` | lines and index with an unreadable `.java` (source and slots skip it, xref names it after its file) and an unreadable `.bajadoc` (slots raises) |

Measured 2026-10-18 by the parity run (every file byte-identical, builder time only, CPython
3.15 vs Go, a shared machine):

| Version | `source-index.json` | `slots-index.json` | `xref-index.json` | source: Python / Go | slots | xref |
|---|---|---|---|---|---|---|
| 4.14.0.162 | 465,484 | 1,465,953 | 1,199,822 | 0.34 / 0.16 s | 3.7 / 1.4 s | 0.23 / 0.26 s |
| 4.15.6.20 | 479,473 | 1,396,710 | 1,268,867 | 0.20 / 0.11 s | 2.0 / 0.96 s | 0.19 / 0.24 s |
| 4.3.58.18 | 470,439 | 1,005,772 | 935,445 | 0.16 / 0.12 s | 2.5 / 1.2 s | 0.16 / 0.21 s |
| 4.13.2.18 | 459,842 | 1,305,528 | 1,148,605 | 0.22 / 0.10 s | 2.9 / 2.5 s | 0.54 / 1.4 s |

The slots time is mostly the `.bajadoc` XML of `devguide/` and `guides/`; xref's is the
`_DECLARATION` regex (`internal/pyre`) run on every line before a file's type declaration.

### S9 intended differences

- **`call-index.json` is retired** (decision I3-5): `build_call_index.py` is not ported,
  `BuilderIndexes` no longer lists it, so `EmptyIndexes` (Python's `_empty_indexes`) does
  not write an empty one for a new version (`TestEmptyIndexesFrozen` holds Python's output
  without that line and file); nh `stats` neither
  counts nor prints it, even when a stale file is on disk, and nh's search scopes have no
  `calls` scope.

- **File names that are not UTF-8** (Python's surrogateescape, sorted as decoded names)
  are not modelled, as in S7a; the tests run on Linux with UTF-8 names.
- **The `.bajadoc` files are parsed concurrently** (as in S8): after an unreadable file
  Python stops, Go has also read the files after it; nothing printed or written changes.

## S10b-1 files: the coverage reports

| File | Ports | Notes |
|---|---|---|
| `coveragereport.go` | `coverage_report.guide_coverage`, `cmd_guide_coverage`, `cmd_coverage` (entry), `index_cache.load_index` for the four report indexes, `module_export.module_classes` | `MissingIndexError` has Python's message; the four indexes (class, source, method, slots) are loaded before anything prints, in Python's order |
| `coveragereportmodule.go` | the module and overview halves of `cmd_coverage` | `Total methods` counts each method-index entry once against the module's class set (Python's per-class loop over distinct keys gives the same number) |
| `doccoverageclassify.go` | `doc_types.classify`, `classify_unmapped`, `content_type`, `example_skip_reason`, `REASONS`, `_GUIDE_JUNK` | extensions lower-cased with Python's full Unicode `lower()` (`textfold.Lower`) |
| `doccoverage.go` | `doc_coverage.Indexed` | lazy views, each index loaded on first use (so an index error surfaces at the same entry); `FulltextDocsAt` is `FtsIndex.docs()` of one database |
| `doccoveragereport.go` | `_entries`, `_verdict`, `scan`, `cmd_doc_jar_coverage`, `_print_report` | the table is `format()` through `pycompat.Format` (`{:34s}`, `{:>7,}`: code-point widths, comma groups); the listing cache is saved before the scan, as Python does |

API for the nh wiring (`cli/cmd_export.py` `coverage`):

```go
// coverage [module]: print to stdout; exit 1 when res.GuideGaps > 0 (overview only).
res, err := helpbuild.Coverage(os.Stdout, baseDir, module) // module "" = overview
// res.Miss: the module matched no class, nothing printed: print
// scope.report_miss(baseDir, "coverage", module, "modules") (internal/help/scope).
// err: *helpbuild.MissingIndexError -> the CLI's "Cannot search:" exit 2
// (internal/help's MissingIndexError has the same text); anything else is a traceback.

// coverage --doc-jars: status 0 PASS, 1 FAIL, 2 install not available.
status, err := helpbuild.DocJarCoverage(os.Stdout, repoRoot, baseDir, versionID, docs)
// docs is a helpbuild.FulltextDocs: open_index(root).docs() -- the up-to-date
// fulltext.sqlite (rebuilt first when stale, as scope.OpenFulltext does) read
// with helpbuild.FulltextDocsAt(path). It is only called when an example entry
// (devguide-examples/) is checked; nil then is an error, never a silent gap.

// coverage --sweep --doc-jars: DocJarCoverage, a blank line, exit 2 when it
// returned 2, else max(status, sweep) (install_sweep, ported separately).
```

`PrintGuideCoverage(w, baseDir)` is `cmd_guide_coverage` alone (`GuideCoverage` its rows);
`ScanDocJars(install, root, jars, docs)` is `scan` (rows, reasons in first-met order, gaps).

S10b-1 tests (a run is two lines: `["out", <stdout>]`, then `["status", <return value>]`,
`["missing", str(e)]` for a `MissingIndexError` or `["raise"]`; `scope.report_miss` is a
`["miss", ...]` line, the scope block being `internal/help`'s):

| Test | Checks |
|---|---|
| `TestClassify`, `TestPyNormpath`, `TestModuleClasses`, `TestScanDocJarsNeedsFulltextForExamples` | one row per classification branch, `posixpath.normpath`, the module match, the full-text reader requirement |
| `TestCoverageReportGolden`, `TestDocJarCoverageGolden` | the Go reports of the synthetic fixtures against `testdata/cov/golden/*.jsonl` (Python's output; `-update-cov` rewrites them from Go) |
| `TestDocJarCoverageFrozen` | the gate on the synthetic install (11 jars: every index kind indexed, not indexed and not extracted, every exclusion reason, unmapped and unknown entries, duplicates, an unreadable jar, a module help jar, a jar name cut at 34 code points around non-ASCII letters, a content type cut at 20) by version id and identified from the data root, an unregistered version, no `install_dir`, no registry, more than 25 gaps, a PASS, a FAIL from a guide folder alone, no indexes, a malformed index |
| `TestCoverageMalformedIndexes`, `TestCoverageMalformedFrozen` | indexes of an unexpected shape (see the intended differences): the Go outcome of each, and the frozen report where Python read no bad value |
| `TestCoverageReportFrozen` | the overview (ties on the percentage, a class without module, non-ASCII modules, `str.replace` of every `.txt`), module reports (`alarm-rt`, `ALARM`, `alarm-ux`, `zeta-rt`, `école`, `rt`), a miss, the guide coverage alone, an empty source index, each missing index |
| `TestCoverageRealGoldens` | with `NIAGARA_HELP_DIR` (`make realdata`; skipped with `-short`): the SHA-256 and exit status of `coverage`, `coverage alarm-rt` and, with the install mounted, `coverage --doc-jars` on 4.14.0.162 against the `coverage`, `coverage-module` and `coverage-doc-jars` cases of niagara-help `tests/golden/cli.json` at `pre-go-retire`, copied into `testdata/cov/cli-goldens.json`; the full-text index must be up to date (the test never writes the corpus) and the listing cache goes to a temporary repository |

Measured 2026-10-18 by the parity run (byte-identical; wall time, Python's driver start-up
included):

| Version | `coverage` | `coverage alarm-rt` | `--doc-jars` (report bytes) | `--doc-jars` Python / Go |
|---|---|---|---|---|
| 4.14.0.162 | 0.49 / 0.17 s | 1.11 / 0.20 s | 97,462 | 20 to 26 s / 15 to 26 s |
| 4.15.6.20 | 0.59 / 0.25 s | 1.24 / 0.15 s | 65,314 | 32 to 127 s / 15 to 18 s |

The `--doc-jars` times are two runs each: both sides read every jar's central directory
over /mnt/c, so the page cache (cold or warm) decides most of them.

### S10b-1 intended differences

- **An index of an unexpected shape** (a top level that is not an object in any of the four
  report indexes, a class, source or method record of another type, a non-string module)
  is an `*IndexShapeError` before any output. Python raises after printing the lines before
  the failing lookup (the module report up to its methods line, the overview up to its
  header) or before any line (a source-index problem in a module report); where it reads
  no bad value it prints the whole report: a class or method index the overview does not
  read, a non-string module whose percentage ties with no other (it prints `str()` of it).
  `TestCoverageMalformedIndexes` pins the Go side; the `python` field of `covMalformed`
  records what Python did in each case. Empty string or object method entries count no method on both sides. A
  non-string `package` prints Python's `str()` of it. In the doc-jar gate the scan finishes
  before anything prints, so a bad index there prints nothing on both sides.
- **A jar whose central directory has an invalid UTF-8 name under the UTF-8 flag**:
  `_entries` catches only `BadZipFile`/`OSError`, so Python stops with a
  `UnicodeDecodeError` traceback; Go counts the jar unreadable (one gap row), as
  `read_listing` does.
- **A registry of an unexpected shape** (`versions` missing or not an object, an entry or
  `install_dir` of another type) is an error, where Python raises or, for a non-string
  `install_dir`, calls `os.path.isdir` on it.

## S11a files

The whole-install sweep (`coverage --sweep`, and with `coverage --doc-jars` a required
gate of `versions add`, niagara-help TOOLING-GUIDELINES.md): every documentation-like
entry of every jar of the install, and every file of its `docs/`, is indexed, excluded
with a reason, or UNCLASSIFIED; a directory the walk cannot list is UNCLASSIFIED too.

| File | Ports | Notes |
|---|---|---|
| `sweep.go` | `install_sweep.find_jars`, `location`, `is_installed`, `Sweep` (`installed_entry`, `_mapped`, `classify`, `_shadowed`, `shadowed_entries`, `classify_loose_doc`), `scan` | the walks are `os.walk(onerror=...)` (`pyWalkFn`, sorted folders and files); in-jar collisions use `MappedEntries`, the extractor's rule |
| `sweepreport.go` | `install_sweep.cmd_sweep`, `print_report`, `_print_pdf_line`; `course.gate`, `print_gate`; the read-only side of `open_index` (`FreshFulltextPath`, `FreshFulltextDocs`) | widths and comma groups through `pyFmt` (code points, as `str.format`); reasons sort by (-count, rule, reason) |

One rule per concept, shared with the S10b-1 coverage gate: `Classify` and `Reasons`
(`doccoverageclassify.go`), `doc_coverage.Indexed` (`indexedViews.check`,
`doccoverage.go`), the `FulltextDocs` reader type and `FulltextDocsAt`,
`loadReportIndex` / `IndexShapeError` for the index files, and `registeredInstall`, the
`reg["versions"].get(vid, {}).get("install_dir", "")` read both `cmd_sweep` and
`cmd_doc_jar_coverage` make.

API for the wiring slice (nh `coverage --sweep`, and the `versions add` gate of S10):

```go
status, err := helpbuild.RunSweep(repo, baseDir, versionID, os.Stdout, helpbuild.SweepOptions{
	Fulltext: rebuildingDocs, // a FulltextDocs; nil: FreshFulltextDocs (read only, *fts.StaleIndexError when stale)
})
// status: helpbuild.SweepPass (0), SweepFail (1, at least one UNCLASSIFIED entry),
// SweepUnavailable (2, one "INSTALL SWEEP <vid>: install not available (...)" line).
// The report ends with the RESULT line: "RESULT: PASS - every documentation-like entry is
// indexed or excluded with a reason" or "RESULT: FAIL - <n> unclassified entr{y,ies}".
// err != nil is what Python raises (a traceback, exit 1); the lines printed before it
// were written to w.
```

`versionID` "" is identified from `baseDir` (`versions.identify`; an unregistered root
prints `None`). The jar listings go through `<repo>/.cache/jar-listings/` (the S1 cache,
byte-identical with Python's). `SweepOptions.Rules` replaces `install_rules.default()`
everywhere `cmd_sweep` reads it (scan, course gate, the documentation-like line). The
lower-level pieces are exported for `coverage --doc-jars` and tests: `Scan` (the `scan`
dict as `*SweepResult`), `SweepReport` (the report lines), `GateCourse`, `FindJars`,
`FreshFulltextDocs`.

`SweepOptions.Fulltext` exists because Python's `open_index` **builds** a missing or
stale full-text index (in `indexes/`, then the user cache, then memory) before the course
gate or an `example` check reads it, printing notes on stderr. The library prints nothing
and does not build: the default `FreshFulltextDocs` reads the up-to-date database or
returns `*fts.StaleIndexError`. nh should pass the same rebuilding `FulltextDocs` it
gives `DocJarCoverage` (backed by `internal/help/scope`'s opener: a path-returning export
of its `ensure`, then `FulltextDocsAt`), which keeps Python's behaviour and adds the build
lock. The index is opened only when a course is bound or an `example` entry has
its corpus file, as in Python.

S11a tests:

| Test | Checks |
|---|---|
| `TestGateCourseBoundCourse`, `TestRunSweepRegistryErrorExitsOne` | the COURSE gate on a bound, an unbound and a missing course (every Dest prefix cut, PDF pairing, not-indexed rows); an unreadable registry exits 1 with nothing printed |
| `TestSweepClassifiesEveryEntry`, `TestRunSweepReport`, `TestSweepListingCache`, `TestSweepCorners`, `TestSweepShadowedEntries`, `TestFreshFulltextDocs`, `TestIndexedViewsCheck` (every `check` kind) | the assertions of niagara-help `test_install_sweep.py` (12 tests) and the sweep cases of `test_t3a_advisories.py` / `test_n3_advisories.py` on a Go-extracted fixture, plus the whole report of that fixture |
| `TestSweepFrozen` | the report lines and exit status, cold and (as a `lineDelta`) warm, on the fixture the parity driver built, now built by `sweepFrozenFixture` with the Go ports of its steps (extract, rebuild-all, the full-text index): every content kind, installed copies identical and changed, in-jar duplicates identical and different, empty, corrupt, upper-case and non-ASCII jars, a root jar, a location longer than its column, more than 25 gaps, loose PDFs in and out of `pdf-index.json` in any case, unreadable folders in the install and under `docs/`, the course row; for version 1.0 with the shipped and the test rules, an identified version, a version sharing the install but never extracted (PDF fallback and course note), an install that is not registered, an unknown version and an unregistered data root |

Measured 2026-10-18 by the parity run (report and cache byte-identical; RESULT PASS on both
versions; the gate's own time, a shared laptop, installs over /mnt/c):

| Version | Jar-listing cache | Python | Go |
|---|---|---|---|
| 4.15.6.20 | cold (every jar listed) | 255.5 s | 67.3 s |
| 4.15.6.20 | warm (niagara-help's cache) | 24 to 33 s | 25 to 35 s |
| 4.3.58.18 | cold | 36.3 s | 13.7 s |
| 4.3.58.18 | warm | 15 to 20 s | 12 to 13 s |

A warm sweep is bound by the walk and the per-jar `stat` over /mnt/c, so both sides take
about the same time; Go wins on listing the jars. `indexes/fulltext.sqlite` of 4.3.58.18
(and 4.13.2.18) is stale in niagara-help (Python's fingerprint agrees), which is why the
real test builds a temporary one.

### S11a intended differences

- **A stale or missing full-text index** with the default finder is an error (exit 1)
  where Python rebuilds it; with nh's rebuilding finder the behaviour is Python's except
  the in-memory last resort (see `scope.UnwritableIndexError`).
- **Registry and index shapes Python trips over**: as in S10b-1 (the shared
  `registeredInstall` and `loadReportIndex`), plus a `pdf-index.json` row that is not an
  object or a non-string truthy `source_pdf`, which are an error naming the file.
- **File names that are not UTF-8** are not modelled (as in S7a and S9).

## S11: nh coverage and rebuild-all wiring

`internal/help/cmds_maint.go` is `run_coverage`: `Coverage` (exit 1 on a guide folder with
unindexed pages; a module that matches no class prints `scope.Miss(..., "coverage", module,
"modules")` and exits 1), `DocJarCoverage` for `--doc-jars` (its status), and for `--sweep`
the doc-jar gate first when `--doc-jars` is given (then a blank line, exit 2 at once when the
install is not available) and `RunSweep`, exiting with the worse status. `--sweep` ignores the
module, as Python does. Both gates get nh's rebuilding `FulltextDocs`: `scope.OpenFulltext`
(open_index's notice on stderr, the build lock next to the database), then
`FreshFulltextDocs`. `helpbuild.MissingIndexError` is the one missing-index type; nh's
`MissingIndexError` is an alias of it.

Until D47, `TestParityMaintReal` ran `coverage --help`, `coverage`, `coverage alarm-rt`, a
module miss, `--doc-jars`, `--sweep` and `--sweep --doc-jars` through the pinned
`niagara_help.py` and through nh's `Main`, on a temporary checkout whose registry held the
version with its absolute data root, and compared stdout, stderr, the exit status and the
jar-listing cache (both seeded from niagara-help's). Measured 2026-10-18 on
4.3.58.18 (all seven identical, warm cache): `--doc-jars` 25.4 / 21.9 s, `--sweep` 42.7 /
45.5 s, both 64.6 / 74.5 s (Python / Go); the reports alone take under 1.1 s on either side.

`rebuild-all` is `run_rebuild_all`: `RebuildAll` on the data root with nh's streams, exit 0
(failed builders included) and `ERROR: ...` with exit 2 on an error. A build or
`--stamp-only` runs inside `scope.WithBuildLock`: the lock file of the full-text build
(`indexes/fulltext.sqlite.lock`), so a search that must build the index waits for the
rebuild (up to the lock's wait, two minutes, then it builds anyway as before). A heartbeat
refreshes the lock's mtime every lockStale/4 while the rebuild runs, so a rebuild longer than
lockStale is not taken over; a crashed holder's lock still is. A live lock still held after
the wait is `*scope.LockBusyError` and the rebuild does not run. A dry run writes nothing
and takes no lock.

Until D47, `TestParityRebuildReal` ran `rebuild-all --help`, `--dry-run`, `--only build_
--dry-run` and `--dry-run --only zzz` on the live data root, then a real `--only
build_class` and `--stamp-only` on a copy of the data root's `devguide/` and `indexes/`,
through the pinned `niagara_help.py` and nh's `Main`, with elapsed and stamp times folded and
Python's `build_call_index.py` row dropped (I3-5), and compared stdout, stderr, exit status
and the rewritten `class-index.json`, `jdk-class-index.json` and manifest. Measured 2026-10-18 on 4.3.58.18: all six identical (3.8 s in all).

### S11 intended differences

- **The hints name nh** (D34): a missing index says `rebuild the indexes with: nh
  rebuild-all`, a stale full-text index `rebuild it with: nh [--niagara-version <id> ]find
  index`, an unextracted version `Run: nh versions extract <id>` (and `freshness --all`
  `NOT EXTRACTED (nh versions extract <id>)`). The frozen coverage reports hold Go's hint.
- **An index of an unexpected shape or a registry Python trips over**, and a rebuild-all
  error (a stream or the manifest that cannot be written), is nh's `ERROR:` line and exit
  2, where Python prints a traceback and exits 1 (the nh convention for a malformed
  registry).
- **The build lock** has no Python counterpart: a rebuild-all while another process holds
  the lock waits for it, and fails with `*scope.LockBusyError` (exit 2) when it is still
  held after two minutes; Python would run both at once.

## S10a: rebuild-all

| File | Ports | Notes |
|---|---|---|
| `rebuildall.go` | `maintenance.BUILDERS`, `OPTIONAL_ARTIFACTS`, `cmd_rebuild_all`; `version_setup._rebuild`'s build.log part (`RebuildToLog`) | the plan is `Builders()` (14 rows, `call-index.json` retired); each builder is the Go entry point of its slice, called in process with the data root, and what the script prints is written to `Stdout`/`Stderr` after it returns (pdf_extractor streams through `PDFOptions.Emit`, progress entries included); the exit status is the script's, an error or a panic of the builder is exit 1 |
| `rebuildall_stamp.go` | `SOURCE_DIRS`, `SOURCE_EXT`, `_fingerprint_dir`, `compute_fingerprint`, `_write_manifest` | `CorpusFingerprint` walks the five folders concurrently (`os.walk`: links to folders listed, not entered; a file whose size cannot be read not counted; `docs/` counts `*.pdf` by `str.lower()`); `StampManifest` takes the time first, then the fingerprint, as the dict literal does, and writes `json.dumps(indent=2)` atomically; `stamped_utc` is `isoformat()` (microseconds only when not zero) |

API for the nh wiring (`internal/help` routes `rebuild-all` to it under the full-text scope
lock; helpbuild never imports `internal/help` and takes no lock itself):

```go
res, err := helpbuild.RebuildAll(baseDir, helpbuild.RebuildOptions{
	Only: only, DryRun: dryRun, StampOnly: stampOnly, // --only, --dry-run, --stamp-only
	Stdout: os.Stdout, Stderr: os.Stderr,             // nil discards
})
// err != nil is where Python raises (a stream that cannot be written, a manifest that
// cannot be saved): exit 1. Otherwise exit 0, failed builders included: run_rebuild_all
// never exits non-zero; res.Failed counts them and res.Stamped is "" (manifest kept).

res, err := helpbuild.ExtractAndIndex(repo, vid, helpbuild.SetupOptions{
	Rebuild: helpbuild.RebuildToLog(os.Stderr), // stdout -> <root>/build.log, returns its tail
})
```

What it prints, in order (stdout unless noted):

- `--stamp-only`: `Manifest stamped at <stamped_utc> (baseline = current corpus).` and nothing else.
- `REBUILD-ALL ` (with `(dry run)` after the space on a dry run), then 60 `=`.
- per builder of the plan whose script starts with `--only`: `  SKIP  <label>  (source '<dir>/' absent)`
  when its source folder is not a folder; else `  PLAN  <label>` on a dry run; else
  `  RUN   <label>`, the builder's own stdout (and its stderr on stderr), and
  `  FAIL  <script> (exit <n>)` when it exited non-zero. `<label>` is `{:26s} -> {}` of the
  script and its artifacts (`jdk-class-index.json (when jdk/ exists)`).
- a dry run: an empty line, `Dry run — nothing executed. Drop --dry-run to build.`
- otherwise an empty line, `Ran <n>, skipped <n>, failed <n>.`, then
  `Manifest re-stamped. Run 'freshness' to confirm FRESH.` (stamped after the line is
  printed) or `Builders failed — manifest NOT updated (still reflects last good build).`

The manifest Go writes is the bytes Python writes (`stamped_utc` aside), so nh `freshness`
reads it as it reads Python's.

`fulltext.sqlite` is not a rebuild-all builder on either side: the full-text index is
built on demand by the search scope when its own fingerprint changes (Integrante2's
`internal/helpbuild/fulltext`, called by nh), not by `maintenance.BUILDERS`.

### What a concurrent reader sees

nh calls RebuildAll under the full-text build lock (`scope.WithBuildLock`), while other nh commands may read the
data root. Every index file is replaced by rename, never truncated in place:

| Output | Written by | A reader during the run sees |
|---|---|---|
| `indexes/*.json` (class, jdk-class, inheritance, method, source, slots, xref, guides, devguide-toc, pdf) and `indexes/.manifest.json` | `WriteJSON` / `atomicfile.WriteText` (temp file + rename in the same folder) | the old file or the new one |
| `guides-clean/`, `devguide-clean/`, `course-text/`, `devguide-examples/course/` | `atomicfile.RebuiltDir` (a sibling temp tree swapped in) | the old tree or the new one, and for the moment between the two renames (old aside, new in) no folder at all, as Python's `rebuilt_dir` |
| `bajadoc-clean/`, `docs-text/` | page by page with `atomicfile.WriteText` (both are shared by two builders or kept page by page on failure) | whole pages only, old and new mixed while the builder runs; `docs-text/` drops stale pages after a clean run |
| `indexes/pdf-index.json` stub (`ExtractAndIndex` only, when the file is absent) | `atomicfile.WriteText` (Python: a plain `open("w")`) | no file or `{}` |
| `build.log` (`RebuildToLog` only) | plain truncating write, as Python | a growing log; nh reads none |

### S10a tests

`TestRebuildPlanFrozen` and `TestRebuildFrozen` check the plan and each scenario against
`testdata/frozen/rebuild/`, checked against `testdata/rebuild/driver.py` before it was
deleted (it ran the tag's tools, Python's builders as child processes, on the same data-root
path, with `build_call_index.py` taken out of `maintenance.BUILDERS` and the parent's stdout
line-buffered; see the intended differences). Per step: stdout and stderr with the
`Done in`/`Time:` values, the stamp time and a traceback against Go's `ERROR: <script>
raised:` line (both `<raised>`) normalized; the tree delta; and the manifest (`stamped_utc`
checked to be `isoformat()`, then normalized) and `build.log` texts.

| Test | Checks |
|---|---|
| `TestRebuildPlanFrozen` | the plan rows (script, artifacts, source, skip-if `None`) and `OPTIONAL_ARTIFACTS`: `BUILDERS` minus exactly `build_call_index.py` |
| `TestRebuildFrozen` | the S7a, S7b, S8 and S9 fixtures merged with the S5 guide pages: a dry run, `--dry-run --only build_`, a full run, `--stamp-only`, a second run, the `build.log` hook (`_rebuild`: stub, log, tail, EMPTY lines); a root with no corpus folder (dry run, run, hook); a stamp, then a run where `bajadoc_parser` exits 1 and `xref_builder` raises (manifest kept), then the hook; `--only zzz` (nothing runs, the manifest is stamped) |

Measured 2026-10-18 by the parity run (dry run then full run; stdout, stderr and the whole
data-root tree identical after both steps, CPython 3.15 with the builders as child processes vs Go in
process, wall time of the two steps, PDFs converted from the installs under /mnt/c):

| Version | Lines compared (both steps) | Python | Go |
|---|---|---|---|
| 4.3.58.18 | 69,754 | 137.9 s | 89.2 s |
| 4.14.0.162 (data root `.`) | 115,415 | 760.5 s | 623.1 s |

Most of the time is pdftotext on the manuals (the same binary on both sides).

### S10a intended differences

- **`call-index.json` is retired** (decision I3-5): the plan has no `build_call_index.py`
  row, so `--only build_c` runs `build_class_index.py` only, the `Ran`/`skipped` counts of a
  version with `source/` are one lower than Python's, and no `call-index.json` is written
  or stamped empty.
- **A builder that raises**: Python's child prints a traceback on stderr; Go prints one line,
  `ERROR: <script> raised: <error>`. A panic of a Go builder is caught the same way (in
  Python the child process dies alone). Both are `  FAIL  <script> (exit 1)` and the run
  goes on.
- **Output order**: Go writes the orchestrator's lines and each builder's output in run
  order on any stream. Python's own lines are line-buffered only on a terminal: to a pipe
  or a file (`nh rebuild-all | tee`, the `build.log` of `extract_version`) they wait in
  the block buffer and come after all the builders' output. The `build.log` tail
  (`SKIP`/`FAIL`/`Ran` lines) is the same either way; the parity driver line-buffered to
  compare the terminal order.
- **stdout and stderr of one builder** are written when it returns, stdout first
  (pdf_extractor's stdout streams); on a terminal Python's child interleaves them as it
  prints. Each stream on its own is byte-identical.
- **The `pdf-index.json` stub** of `ExtractAndIndex` is an atomic write with mode 0644
  (Python's `open("w")` is not atomic and takes 0666 less the umask; the same bytes and
  mode under the usual umask 022).

## S10b-2: versions add and versions extract

| File | Ports | Notes |
|---|---|---|
| `versionsetup_add.go` | `valid_version_id`, `detect_version`, `register`, `_restore_registry`, `_rollback`, `add_version` | `AddVersion(repo, install, w, AddOptions)` writes add_version's lines to `w` and returns its status (0 ready, 1 a gate or the fidelity check failed, 2 cannot add); an error is what Python raises, after the rollback when extraction raised; `\d`/`\s` are Unicode classes as in Python's `re` |
| `versionreport.go` | `run_gates`, `_result_line`, `_counts`, `print_summary`, `KINDS`, `GATES_LOG` | `RunGates` runs `DocJarCoverage` then `RunSweep` into buffers of their own, writes both to `<data root>/gates.log`; `PrintSummary` counts the full-text documents per source through the `FulltextDocs` it is given |

API for nh (`internal/help/cmds_maint_versions.go`):

```go
status, err := helpbuild.AddVersion(repoRoot, install, os.Stdout, helpbuild.AddOptions{
	Label: label, Default: makeDefault, Force: force, ID: idOrNil, // nil: detected
	Rebuild:  helpbuild.RebuildToLog(os.Stderr), // the extraction's rebuild-all
	Fulltext: rebuildingDocs,                    // open_index(root).docs(), building under scope's lock
})
```

`versions extract <id> [--force] [--verify-only] [--fill-missing] [--no-index]` is
`extract_version.py` (which had no nh entry) over `ExtractAndIndex`: an empty id, an
unknown id (resolved against the registry only, no `NIAGARA_HELP_VERSION`, no default) and
a refusal print `ERROR: ...` on stderr with exit 2; otherwise exit 0 verified, 1 the
fidelity check failed. Its cases are Go-only (`internal/help/testdata/go/golden`, D46).

S10b-2 tests: `TestVersionsAddFrozen` checks `versions add` against
`testdata/frozen/versions/`, checked against `testdata/versions/driver.py` (`add_version` as
`cli/cmd_versions.py` calls it, `build_call_index.py` taken out of `BUILDERS`, stdout
line-buffered) before it was deleted. Each scenario runs in one repository path under `$HOME`
(never `/tmp`); per step: stdout, the exit status and the change of the observed repository
(`lineDelta`): the whole tree (registry, version manifest, every data-root file, `gates.log`)
with `generated_utc` of the version manifest, `stamped_utc` of `indexes/.manifest.json`, the
`Done in`/`Time:` run times of `build.log` and, in `indexes/fulltext.sqlite` (listed as its
`docs` and `meta` rows), the newest-mtime and SHA-1 fields of each fingerprint source
normalized; `gates.log` and `versions/registry.json` are listed as text with the folder as
`<dir>` and the jar-listing cache entry is masked, so the expectation holds under any `$HOME`.
stderr is not recorded.

| Test | Checks |
|---|---|
| `TestValidVersionID`, `TestDetectVersion`, `TestRegister`, `TestAddVersionRefusals`, `TestAddVersionHappyPath`, `TestAddVersionRollsBackARefusal`, `TestAddVersionRollsBackAnError`, `TestResultLine` | every refusal line, detection order and fallbacks, the registry bytes, a full add, the rollbacks |
| `TestVersionsAddFrozen` | the synthetic install plus a `baja.jar`: the refusals before registering (no install, a bad `--id`, an empty `--id`, Unicode digits), an add, the same add again, a forced re-add with `--default` and a non-ASCII `--label`, a second install spelling with `--id`, a data root that already holds a raw folder (rollback of a registry the add did not create) |

Measured 2026-10-18 by the parity run on the EC-Net 4.3.58.18 subset: 21,287 observed lines
identical; Python
68.2 s, Go 17.0 s (both including the PDF conversion and the gates).

### S10b-2 intended differences

- **The registry** is written in one atomic write with its trailing newline (Python writes the
  JSON atomically, then appends the newline).
- **A `baja.jar` whose `module.xml` cannot be inflated** falls back to the folder name like a
  bad zip (Python raises `zlib.error`, which `detect_version` does not catch). An encrypted
  entry or an unsupported method is an error on both sides.
- **A registry of an unexpected shape** (top level or `versions` not an object, an entry or its
  `install_dir`/`label` of another type) is an error, as in S10b-1.
- **What Python raises** (a traceback, exit 1) is nh's `ERROR: ...` with exit 2, as for every
  nh command; the rollback runs first in both.
- **Output timing**: the lines of the extraction step are written when it returns (Python
  prints them as they come), the same bytes in the same order.
- **`gates.log`** is LF on every OS (Python's text mode writes CRLF on Windows), as `build.log`.
- **A full-text index no location can hold** is `scope.UnwritableIndexError` (exit 2), as in S11a.

## Real data

`TestCoverageRealGoldens` (S10b-1) is the package's one real-data test:
`NIAGARA_HELP_DIR=~/niagara-research/niagara-help go test ./internal/helpbuild -run
TestCoverageRealGoldens`, part of `make realdata`; it skips with `-short` or without the
variable. The `HELPBUILD_PARITY_*` variables and the real-version parity runs retired with
D47; their last measurements stay in the sections above. `go test ./internal/helpbuild/`
takes about 20 s on the development laptop.

## Dead code not ported

- `maintenance.ORPHAN_INDEXES` is `[]` (its cleanup loop never runs).
- `maintenance.BUILDERS` `skip_if` is always `None` (the skip branch is unreachable).
- `bajadoc_parser.format_output` exists only as a re-export for `bajadoc_xml_parser`; both
  are retired in Phase 5.
- `doc_jars.fidelity_jar(report=None)` default is used only by tests.
- `bajadoc_parser._TagStripper.handle_entityref` / `handle_charref` (never called with
  `convert_charrefs`) and `strip_tags`'s regex fallback (`feed()` does not raise).
