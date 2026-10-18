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

## S2 files

| File | Ports | Notes |
|---|---|---|
| `zipread.go` | `ZipFile.open(info)` + `ZipExtFile.read` | local-header checks in Python's order, inflate, cut at `file_size`, CRC; `whole` = `read()`, else `copyfileobj`'s bounded reads |
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
hook skips the step, which is `extract_and_index(no_index=True)`; that is what the parity
tests compare, since the index builders are later slices. `extract_version --no-index`
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
niagara-help checkout — `$NIAGARA_HELP_DIR`; else `$NIAGARA_RESEARCH/niagara-help` when it
holds the data file (the variable the glossary and version drift tests read); else the
first one met walking up from the test directory as `<dir>/niagara-help` or
`<dir>/niagara-research/niagara-help` (a worktree under `~/niagara-go-worktrees` finds
`~/niagara-research/niagara-help`) — and skips visibly only when there is none. A run
that set any `HELPBUILD_PARITY_*` variable (`make parity` sets four) fails instead of
skipping when `python3`, the checkout, its tools or a real version's `versions/` tree are
missing, the message naming the variable. The interpreter must be CPython 3.15.x
(`html.parser` and the other stdlib behaviour the drivers compare change between minor
releases): another version is a missing reference like the rest (skipped visibly in a
plain run, every parity test failing when a `HELPBUILD_PARITY_*` variable is set), and `-v`
logs the version used (`parity driver python: CPython 3.15.0 (...)`). After Phase 5 retires the Python tools from
niagara-research, the reference lives in a `pre-go-retire` worktree of niagara-research:
point `NIAGARA_HELP_DIR` at its `niagara-help` folder (for example
`NIAGARA_HELP_DIR=~/niagara-research-worktrees/pre-go-retire/niagara-help make parity`). The drift
test `TestEmbeddedRulesMatchCanonical` uses the same lookup; `$NIAGARA_HELP_DIR` pointing
at a folder without the data file fails it, a `$NIAGARA_RESEARCH` without one falls
through to the walk.

| Test | Driver mode | Compares |
|---|---|---|
| `TestParitySyntheticJars` | `classify` | per jar the compact listing JSON, per entry: `is_doc_like`, extract rule, exclude rule + reason, the `(course)` exclude rule, `course_kind`, `is_guide_page`, `is_licence`, `is_source_archive("guides/"+entry)`, `clean_text_name(entry, src)` (with a sibling tree) |
| `TestParityRealJars` | `classify` | the same on five EC-Net 4.3.58.18 jars (`docDeveloper-doc` the biggest doc jar, `docLinuxN4-doc`, `alarm-rt`, `workbench-wb`, `batchJob-ux`) plus `ffmpeg-wb.jar` of 4.15.6.20 (the only source archive); `HELPBUILD_PARITY_JARS` replaces the list; only with `HELPBUILD_PARITY_REAL=1` (see Real-data parity) |
| `TestParityListingCache` | `cache` | a fresh cache written by each side (byte-identical), each side reading the other's cache (0 jars opened), an update of a Python cache by each side (byte-identical, dict order kept) |
| `TestParityCachePath` | `path` | `cache_path` for absolute, relative, `..`, double-slash and non-ASCII install paths |
| `TestParityGlob` | `glob` | 468 pattern/path pairs through `glob_re` |

S2 adds (driver modes `map`, `helpjars`, `docjars`, `setup`, `emptyindexes`; every
Python exception becomes `["raise"]`, compared against a Go error):

| Test | Compares |
|---|---|
| `TestParityExtractSynthetic` | `extract_and_index` result lines and both whole repo trees (path, kind, permission bits, size, sha256; manifests inline with `generated_utc` normalized) after each of: install not mounted, repo-root refusal, verify-only over the repo root, extract, already-extracted refusal, verify after mismatched / EOL-only / missing edits, fill-missing, force |
| `TestParityDocJarCorners` | `extract`, `fill_missing`, `fidelity` (also over what extract left) and the trees (a root one side did not create counts as a difference) on twelve one-jar installs: bad CRC, local name ≠ central name, encrypted, unknown method, overlapped entries, a shared local header, `file_size` shorter than the data, cp437 and NUL names, prepended data, a zip64 end record, not a zip, a folder named like a jar |
| `TestParityDocJarMapping` | `is_doc_jar` … `guide_folder`, `dest_path`, `is_help_entry`, `mapped_entries` over every synthetic jar plus a grid of 27 jar kinds × 32 entry shapes; `help_jars` / `list_doc_jars` of the synthetic install |
| `TestParityExtractReal` | extract + verify of a copied EC-Net 4.3.58.18 subset (`docDeveloper-doc`, `docDeveloper-ux`, `docDeveloper-rt`, `docLinuxN4-doc`, `docObix-doc`, plus `workbench-wb`, `batchJob-ux`, `alarm-rt`); only with `HELPBUILD_PARITY_REAL=1` (see Real-data parity) |
| `TestParityHelpJarsFullInstall` | `help_jars` / `list_doc_jars` of the whole 4.3 install (578 module jars, ~25 s a side); only with `HELPBUILD_PARITY_FULL=1` |
| `TestParityEmptyIndexes` | `_empty_indexes` lines and tree |

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
| `pyfs.go` | `os.walk`, `os.remove`, `shutil.copyfile`, `json.load` | `pyWalk` lists a symlinked folder without entering it and skips unreadable ones; `removeFile` refuses a directory; `pyCopyfile` raises `SameFileError` when source and destination are the same file, refuses a named pipe before opening it and a directory as source (`EISDIR`, Python's `IsADirectoryError`) or destination (the open fails); `loadJSON` returns a `*JSONLoadError` (Python's `str(e)`) for what the builders catch |
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
does without a shell; `PDFOptions.Pdftotext` names another program. The parity tests make
both sides resolve the same binary (pdftotext 26.10.0 of linuxbrew): the test puts a fake
`pdftotext` first on PATH that hands every real PDF to that binary.

S3/S4 parity (`testdata/parity/textdriver.py`, modes `pdf`, `course`, `plan`):

| Test | Compares |
|---|---|
| `TestParityPDFSynthetic` | stdout (data root relabeled, the `Time:` value normalized), exit status and both whole repo trees after each of: unregistered root (SKIP, docs dir `None`), unregistered root with manuals, corrupt index (bad JSON, not an object, not UTF-8) with nothing to rebuild from, install not mounted, a docs dir with no PDF, pdftotext not on PATH, then 13 hand-written PDFs (two pages with an unmapped bullet, a WinAnsi degree sign, short and blank pages, a stem collision, hardware, top-level, nested, non-ASCII and punctuated names, a symlinked folder) with a corrupt PDF, output that is not UTF-8 and a timeout (fake pdftotext, 1 s on both sides for that PDF only): failures keep the last good text and its re-dumped entry, a corrupt index is rebuilt, a clean run prunes |
| `TestParityPDFReal` | the same on five EC-Net 4.3.58.18 manuals (the largest, `Getting Started`, 9.6 MB; `Station Security`, whose bullets come out as U+FFFD; the smallest; a hardware guide; the platform guide); `HELPBUILD_PARITY_PDF_DOCS` converts a whole docs dir instead; only with `HELPBUILD_PARITY_REAL=1` (see Real-data parity) |
| `TestParityCoursePlanSynthetic` | every `plan` row plus the `(course)` exclude rule id over the test_course_source fixture and 18 more files (nested text, upper-case extension, dev readme.md, .js and doc pages, key material, solutions, PDFs, non-ASCII names, a symlinked folder) |
| `TestParityCourseExtractorSynthetic` | stdout, stderr, exit status and both repo trees: bound by a relative and an absolute `course_dir`, unbound (earlier copies removed; a symlink in their place refused), `course_dir` missing, unregistered root, rebuilt after the course changed, a copy that fails (mode 000 file), a named pipe in the course, a registry that is not JSON |
| `TestParityCourseReal` | plan rows and the copy of `sources/n4-dev-course` (334 files: 20 text, 169 code, 17 PDFs, 128 not copied; `HELPBUILD_PARITY_COURSE` replaces it); only with `HELPBUILD_PARITY_REAL=1` (see Real-data parity) |

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
  parity tests compare Python's `pdf-index.json` with `pages` lowered by one wherever
  the entry's `docs-text/<stem>.txt` ends with a form feed (`pyPDFTreeLines`).
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
- **File names that are not UTF-8** (Python's surrogateescape) are not modelled; parity runs
  on Linux with UTF-8 names.

Found while porting (the Python code wins, kept for parity):

- The parity driver shortens the 120 s timeout only for the timeout fixture
  (`HB_PDF_TIMEOUT` replaces `timeout=120` of `extract_text` for a PDF with a
  `<pdf>.sleep` side file); the Go side does the same through the `pdfTimeout` hook.
  Every other PDF keeps 120 s on both sides.

## Shared-helper candidates

`pyJoin` (`os.path.join` without cleaning), `pyAbspath`, `pyBasename`, `rmtreeIgnore`
(`shutil.rmtree(ignore_errors=True)`) and `commas` (`format(n, ",")`) are local to this
package; `pyJoin`/`pyAbspath`/`pyBasename` belong next to `pycompat.PosixJoin` once a
second package needs them.

S8 adds one: `pyTextLines` (`inheritread.go`, a text file read line by line with
TextIOWrapper's chunked strict decoding) belongs in `pycompat` next to `ReadText` once a
second package needs Python's decode-error text. The class index reads `.bajadoc` files
through `bajadocxml.Parse`, as S7b does; S7b's sequential walk (`bajadocXMLClasses`) could
take `parseBajadocFiles` (`classindexxml.go`) for the same concurrency.

## S5 and S6 files

| File | Ports | Notes |
|---|---|---|
| `internal/pyhtml` (shared package, D37) | `html.parser.HTMLParser` (CPython 3.15), `convert_charrefs=True`; `html.unescape`; `name in html.entities.html5` | the tokenizer below (`pyhtml.Parse`, a `pyhtml.Handler`); reports only the callbacks the tools override |
| `htmltext.go` | `html_text.convert` as `HTMLToText(page) (title, text)` | Python's Unicode `\s`, `strip` and `\w` (`pycompat.IsSpace`, `IsWordRune`) |
| `guidetoc.go` | `guide_toc.find_toc`, `parse_toc`, `count_items` | ElementTree is `internal/pyxml` in ETree mode; the warning is returned, not printed |
| `guidesparser.go` | `guides_parser.parse_guides`, `_markdown_title`, `devguide_parser.parse_devguide` | `RebuiltDir`; `TextResult.Log` = stdout lines, `.Warnings` = stderr lines |
| `guidesindex.go` | `build_guides_index` (`topic_name`, `known_jar_names`, `modules_dir_of`, `jar_for_folder`, `previous_jars`, `build`, `main`) | `JarSources` replaces the `_MANIFEST` sentinels; `Unknown` is `jar_names=None`; the walk, `json.load` and registry lookups are S3's (`pyfs.go`, `registry.go`) |
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
`pycompat.HTMLUnescape` can forward to it. Its parity tests stay here, next to their
driver.

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
parity test over all 2,231 names of `html.entities.html5`). The attribute rule
(`_unescape_attrvalue`) needs `name in html5`; Go does not export its table, so
`pyhtml.IsHTML5Entity` reads it through `html.UnescapeString` (an exact name decodes to 1 or 2 code points, a prefix match leaves the rest of the name).

### Parity tests (`testdata/parity/htmldriver.py`)

| Test | Compares |
|---|---|
| `TestParityHTMLCorners` | the parser callbacks and `convert()` of 60 hand-written pages (the Python tests' fixtures, unterminated constructs, raw text, attribute quoting, charrefs) |
| `TestParityHTMLFuzz` | the same on 5,000 seeded random snippets of tag/charref/whitespace fragments, some cut at a random point (400 with `-short`) |
| `TestParityHTMLFeed` | `pyhtml.Feed` (one `feed()` without `close()`, bajadoc's `strip_tags`) against the same on the corner pages plus held-back ends (cut references inside and outside the 34-code-point look-back, raw text without its end tag, a lone `<`) and on 5,000 seeded random snippets (400 with `-short`) |
| `TestParityHTML5Entities` | `pyhtml.IsHTML5Entity` on every `html5` name and its near misses, `pyhtml.Unescape` against `html.unescape` on 46,851 texts |
| `TestParityTocFiles` | `parse_toc` (order, titles, trails), `count_items` and the stderr warning on the synthetic toc.xml files and corners (namespaced, root `tocitem`, empty, undefined entity, BOM + ISO-8859-1 / US-ASCII / UTF-8 / UTF-16 (one real 4.3/4.15 toc.xml is BOM + ISO-8859-1; `pyxml` applies the declared encoding after a BOM as expat does), junk, missing file) |
| `TestParityGuidesSynthetic` | stdout/stderr lines, return value and both whole trees after each builder (and a second run) on a synthetic root: the `test_extraction` pages, toc.xml corners, Markdown, `.txt` beside `.html`, licence texts, a non-UTF-8 page body, CRLF, a broken toc, `TOC.XML` |
| `TestParityGuidesIndexSources` | `guides-index.json` of a registered root with a manifest and a mounted install, an unmounted install, and no manifest (values kept from an earlier index) |
| `TestParityNonUTF8Names` | page and folder names that are not UTF-8 |
| `TestParityGuidesReal` | all four builders on EC-Net 4.3.58.18 (guides/ and devguide/ linked into a temporary registered root): 5,077 files, 15,528,757 bytes; `HELPBUILD_PARITY_415=1` adds 4.15.6.20 (6,640 files, 34,680,193 bytes); only with `HELPBUILD_PARITY_REAL=1` (see Real-data parity) |

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

The `Done in %.1fs` line is the real elapsed time on both sides; the parity tests print it
as `Done in <t>s` (as S3 does with `Time:`).

S7a parity (`testdata/parity/bajadocdriver.py`, mode `run`):

| Test | Compares |
|---|---|
| `TestParseBajadocFixture` | no Python needed: `ParseBajadoc` on `testdata/bajadoc/root/` against the golden `testdata/bajadoc/clean/` and `stdout.txt` (Python's output, regenerated with the driver's `run` mode; the driver's docstring says how) |
| `TestParityBajadocSynthetic` | stdout lines, exit status and both whole trees: the fixture (classes, an interface with subinterfaces, an enum, an annotation type, a generic class name, the `<title>` fallback, skipped pages, `class-use/` folders, no package, CRLF and bytes that are not UTF-8, a 78-code-point wrap, member rows whose fragments `feed()` holds back), then an unreadable page, a dangling link, links to a page and to a folder, a page under its escaped name, an unrelated S7b page, a second run, and a root without `bajadoc/javax/baja` (exit 1); it also fails when the golden files are no longer Python's output |
| `TestParityBajadocReal` | the same on a real version's `bajadoc/` (linked into a temporary data root): 4.14.0.162 with `HELPBUILD_PARITY_REAL=1`, 4.15.6.20 with `HELPBUILD_PARITY_415=1`, or the `HELPBUILD_PARITY_BAJADOC` list; logs both run times |

Measured 2026-10-17 (whole trees identical, Python 3.15 vs Go, wall time of one run):

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
  not modelled; parity runs on Linux with UTF-8 names.

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

S7b parity (`testdata/parity/bajadocdriver.py`, mode `xmlrun`):

| Test | Compares |
|---|---|
| `TestParseBajadocXMLFixture` | no Python needed: `ParseBajadocXML` on `testdata/bajadocxml/root/` against the golden `testdata/bajadocxml/clean/` and `stdout.txt` (Python's output, regenerated with the driver's `xmlrun` mode) |
| `TestParityBajadocXMLSynthetic` | stdout lines, exit status and both whole trees: the fixture (a class with every member kind, varargs, parameterized, wildcard and type-variable types, markup, a comment and CDATA in a description, two `@since` tags, two `<implements>` elements of which only the first is read; an enum, an annotation type, an interface, a generic name, no package; a class with a javadoc page; a class in both trees and in two devguide modules; an index file; an empty and an unclosed file; a forbidden character reference; CRLF; files that are not `.bajadoc`), then a page under its escaped name, S7a's page of the class with a javadoc page, links to a file and to a folder, a second run, a dangling link and an unreadable file (both raise), and a root without `devguide/` or `guides/`; it also fails when the golden files are no longer Python's output |
| `TestParityBajadocXMLReal` | the build's order on a real version (`bajadoc/`, `devguide/`, `guides/` linked into a temporary data root): S7a (`run`) then S7b (`xmlrun`) on each side, the lines of both steps and the whole `bajadoc-clean/` tree; 4.3.58.18 and 4.14.0.162 with `HELPBUILD_PARITY_REAL=1`, 4.15.6.20 with `HELPBUILD_PARITY_415=1`, or the `HELPBUILD_PARITY_BAJADOCXML` list; logs the run times of each step |

Measured 2026-10-17 (S7a then S7b; whole `bajadoc-clean/` trees identical, Python 3.15 vs
Go, wall time of one run of each step):

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

S8 parity (`testdata/s8/indexdriver.py`, modes `class`, `inherit`, `method`; the driver also
prints each builder's own run time):

| Test | Compares |
|---|---|
| `TestBuildClassIndexFixture`, `TestBuildInheritanceFixture`, `TestBuildMethodIndexFixture` | no Python needed: each Go builder on `testdata/s8/root/` against `testdata/s8/golden/` (stdout lines and indexes, Python's output; `go test -run TestParityS8Synthetic -args -update-s8` rewrites them) |
| `TestParityS8Synthetic` | stdout lines, exit status and both whole trees after each builder on the fixture: a javadoc list with spans, CRLF, a byte that is not UTF-8, a duplicate and a non-class title; `.bajadoc` classes of every kind (attributes and categories), modules with and without a runtime profile, a class without a qualified name, nested and namespaced classes, a file fixed by sanitizing (`&#x0;`, `&#0;`, a value beyond any integer), one that stays broken, Latin-1, simple names shared across javax.baja, javadoc and sibling trees; bajadoc-clean pages with every header line, CRLF and lone CR, a BOM, Unicode whitespace, duplicate names (preferred, no package, empty package, twice the same), member sections with generic returns, Unicode names and spaces, field descriptions; then the corners (decode errors in, after and past the header, across a chunk end, a `\r` at a chunk end, a truncated end, a line longer than a chunk, symlinked folders and files) and roots without the folders each builder reads; it also fails when the golden files are no longer Python's output |
| `TestParityS8Unreadable` | stdout lines and exit status with an unreadable `.bajadoc`, an unreadable page and a dangling `.bajadoc` link (class and method index raise, inheritance lists the page under `Errors`) |
| `TestParityS8Real` | the three builders on a real version's `bajadoc/`, `devguide/`, `guides/`, `jdk/` and `bajadoc-clean/` (linked into a temporary data root): 4.14.0.162 with `HELPBUILD_PARITY_REAL=1`, 4.15.6.20 with `HELPBUILD_PARITY_415=1`, or the `HELPBUILD_PARITY_INDEXES` list; logs sizes and both run times |

Measured 2026-10-17 (every file byte-identical, builder time only, CPython 3.15 vs Go, a
shared machine):

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

S9 parity (`testdata/s9/indexdriver.py`, modes `source`, `slots`, `xref`; the driver
prints stdout lines, then stderr lines as `["err", line]`, and each builder's own run
time):

| Test | Compares |
|---|---|
| `TestBuildSourceIndexFixture`, `TestBuildSlotsIndexFixture`, `TestBuildXrefIndexFixture` | no Python needed: each Go builder on `testdata/s9/root/` against `testdata/s9/golden/` (lines exactly; the index with its top-level keys sorted, since source and slots follow the directory order of the file system the golden file was written on); `go test -run TestParityS9Synthetic -args -update-s9` rewrites them |
| `TestParityS9Synthetic` | lines, exit status and both whole trees after each builder on the fixture (multi-line annotations, nested parentheses, literals holding commas, quotes and `=`, comments in and around annotations, unnamed, empty and repeated keys, a closer that does not match, an annotation inside a comment, a package split over two lines, an indented package, a default package, `X.java.java`, non-ASCII names, static, wildcard and self imports, `.bajadoc` slots with and without a source entry, module or package, an index file and a broken file); then the corners written at test time (one class name in three folders of one module and five identical copies with the `#n` warning, both order-dependent; CRLF, a lone CR, a form feed, U+2028, bytes that are not UTF-8, a literal left open by a final backslash, a `\` in a file name, a linked module, a linked folder inside a module, linked and dangling `.java` files); then roots without `source/`, without `devguide-rc/`, a `source` that is a file, and an import ending with `.` (xref raises); it also fails when the golden files are no longer Python's output |
| `TestParityS9Unreadable` | lines and index with an unreadable `.java` (source and slots skip it, xref names it after its file) and an unreadable `.bajadoc` (slots raises) |
| `TestParityS9Real` | the three builders on a real version's `source/`, `devguide-rc/`, `devguide/` and `guides/` (linked into a temporary data root), on the versions of `TestParityS8Real` (`HELPBUILD_PARITY_INDEXES`, or the `REAL` and `415` switches); logs sizes and both run times |

Measured 2026-10-18 (every file byte-identical, builder time only, CPython 3.15 vs Go, a
shared machine):

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
  not write an empty one for a new version (`TestParityEmptyIndexes` removes Python's line
  and file before comparing, and fails once Python stops writing it); nh `stats` neither
  counts nor prints it, even when a stale file is on disk, and nh's search scopes have no
  `calls` scope.

- **File names that are not UTF-8** (Python's surrogateescape, sorted as decoded names)
  are not modelled, as in S7a; parity runs on Linux with UTF-8 names.
- **The `.bajadoc` files are parsed concurrently** (as in S8): after an unreadable file
  Python stops, Go has also read the files after it; nothing printed or written changes.

## Real-data parity

Every test that reads a real install or the real corpus runs only on request, `-short`
or not; without its variable it skips and the message names the variable. A switch
(`REAL`, `FULL`, `415`) counts only when set to `1`: any other value fails the test,
naming the variable and the accepted value. A selection that leaves nothing to compare
(no version for `TestParityGuidesReal`, `TestParityBajadocReal`, `TestParityBajadocXMLReal`, `TestParityS8Real` or `TestParityS9Real`, a `HELPBUILD_PARITY_JARS` naming no jar, a
`HELPBUILD_PARITY_PDF_DOCS` without a PDF, a `HELPBUILD_PARITY_COURSE` without files)
fails too, and an explicit jar list or course never skips as "not mounted". The synthetic,
cache, glob, toc and fuzz parity tests (5,000 snippets) stay on in `go test`.

| Variable | Turns on |
|---|---|
| `HELPBUILD_PARITY_REAL=1` | the umbrella: `TestParityRealJars` (five 4.3 jars + 4.15 `ffmpeg-wb.jar`), `TestParityExtractReal` (the 4.3 subset), `TestParityPDFReal` (five 4.3 manuals), `TestParityCourseReal`, `TestParityGuidesReal` (4.3.58.18), `TestParityBajadocReal` (4.14.0.162), `TestParityBajadocXMLReal` (4.3.58.18 and 4.14.0.162), `TestParityS8Real` and `TestParityS9Real` (4.14.0.162) |
| `HELPBUILD_PARITY_JARS=a.jar:b.jar` | `TestParityRealJars` on that list instead |
| `HELPBUILD_PARITY_FULL=1` | `TestParityHelpJarsFullInstall` (all 578 module jars of 4.3) |
| `HELPBUILD_PARITY_PDF_DOCS=<dir>` | `TestParityPDFReal` over a whole docs folder (the 66-PDF run) |
| `HELPBUILD_PARITY_COURSE=<dir>` | `TestParityCourseReal` on that course tree |
| `HELPBUILD_PARITY_415=1` | `TestParityGuidesReal`, `TestParityBajadocReal`, `TestParityBajadocXMLReal`, `TestParityS8Real` and `TestParityS9Real` on 4.15.6.20 too |
| `HELPBUILD_PARITY_BAJADOC=a:b` | `TestParityBajadocReal` on that list of version ids instead (for example `4.13.2.18`) |
| `HELPBUILD_PARITY_BAJADOCXML=a:b` | `TestParityBajadocXMLReal` on that list of version ids instead |
| `HELPBUILD_PARITY_INDEXES=a:b` | `TestParityS8Real` and `TestParityS9Real` on that list of version ids instead |

`make parity` sets `REAL`, `FULL`, `415` and `INDEXES=4.14.0.162:4.15.6.20:4.3.58.18` and runs every `Parity` test of the package.
Timings on the development laptop: `go test ./internal/helpbuild/` about 20 s without the
variables (about 170 s when the real tests ran by default); `HELPBUILD_PARITY_REAL=1 go test ./internal/helpbuild/` about 60 s; `make parity`
4.5 to 6 min (the full-install sweep 80 s or more over /mnt/c, 4.3 + 4.15 guides about 135 s).

## Dead code not ported

- `maintenance.ORPHAN_INDEXES` is `[]` (its cleanup loop never runs).
- `maintenance.BUILDERS` `skip_if` is always `None` (the skip branch is unreachable).
- `extract_version --no-index` has no caller.
- `bajadoc_parser.format_output` exists only as a re-export for `bajadoc_xml_parser`; both
  are retired in Phase 5.
- `doc_jars.fidelity_jar(report=None)` default is used only by tests.
- `bajadoc_parser._TagStripper.handle_entityref` / `handle_charref` (never called with
  `convert_charrefs`) and `strip_tags`'s regex fallback (`feed()` does not raise).
