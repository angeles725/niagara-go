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
`~/niagara-research/niagara-help`) — and skips visibly only when there is none. The drift
test `TestEmbeddedRulesMatchCanonical` uses the same lookup; `$NIAGARA_HELP_DIR` pointing
at a folder without the data file fails it, a `$NIAGARA_RESEARCH` without one falls
through to the walk.

| Test | Driver mode | Compares |
|---|---|---|
| `TestParitySyntheticJars` | `classify` | per jar the compact listing JSON, per entry: `is_doc_like`, extract rule, exclude rule + reason, the `(course)` exclude rule, `course_kind`, `is_guide_page`, `is_licence`, `is_source_archive("guides/"+entry)`, `clean_text_name(entry, src)` (with a sibling tree) |
| `TestParityRealJars` | `classify` | the same on five EC-Net 4.3.58.18 jars (`docDeveloper-doc` the biggest doc jar, `docLinuxN4-doc`, `alarm-rt`, `workbench-wb`, `batchJob-ux`) plus `ffmpeg-wb.jar` of 4.15.6.20 (the only source archive); `HELPBUILD_PARITY_JARS` replaces the list; skipped with `-short` |
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
| `TestParityExtractReal` | extract + verify of a copied EC-Net 4.3.58.18 subset (`docDeveloper-doc`, `docDeveloper-ux`, `docDeveloper-rt`, `docLinuxN4-doc`, `docObix-doc`, plus `workbench-wb`, `batchJob-ux`, `alarm-rt`); skipped with `-short` |
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
| `pdftext.go` | `classify`, `sanitize`, `top_folder`, `find_pdfs`, `fix_unmapped_bullets`, `extract_text`, `resolve_name` | one pdftotext run: argv list, no shell, `PDFTimeout` (120 s); a failure is a `*PDFError` whose `Failure` says which of the four cases `extract_text` catches (exit status, timeout, output not UTF-8, could not start) |
| `pdfindex.go` | `pdf_extractor.main()` | `ExtractPDFs`; the early exits, the `-v` probe, keep-last-good, the index save, the prune after a clean run, the summary |
| `procgroup_unix.go`, `procgroup_other.go` | — | the child runs in its own process group; the timeout or a relayed SIGINT/SIGTERM kills the group (a copy of `internal/secaudit`'s relay) |
| `registry.go` | `versions.identify`, `versions.install_dir`, `course.course_dir` | read with pyjson so every malformed entry takes Python's branch: skipped where Python catches, an error where it raises |
| `pyfs.go` | `os.walk`, `os.remove`, `shutil.copyfile`, `json.load` | `pyWalk` lists a symlinked folder without entering it and skips unreadable ones; `removeFile` refuses a directory; `pyCopyfile` refuses a named pipe before opening it; `loadJSON` returns a `*JSONLoadError` (Python's `str(e)`) for what the builders catch |
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
| `TestParityPDFSynthetic` | stdout (data root relabeled, the `Time:` value normalized), exit status and both whole repo trees after each of: unregistered root (SKIP, docs dir `None`), unregistered root with manuals, corrupt index (bad JSON, not an object, not UTF-8) with nothing to rebuild from, install not mounted, a docs dir with no PDF, pdftotext not on PATH, then 13 hand-written PDFs (two pages with an unmapped bullet, a WinAnsi degree sign, short and blank pages, a stem collision, hardware, top-level, nested, non-ASCII and punctuated names, a symlinked folder) with a corrupt PDF, output that is not UTF-8 and a timeout (fake pdftotext, 1 s on both sides): failures keep the last good text and its re-dumped entry, a corrupt index is rebuilt, a clean run prunes |
| `TestParityPDFReal` | the same on five EC-Net 4.3.58.18 manuals (the largest, `Getting Started`, 9.6 MB; `Station Security`, whose bullets come out as U+FFFD; the smallest; a hardware guide; the platform guide); `HELPBUILD_PARITY_PDF_DOCS` converts a whole docs dir instead; skipped with `-short` |
| `TestParityCoursePlanSynthetic` | every `plan` row plus the `(course)` exclude rule id over the test_course_source fixture and 18 more files (nested text, upper-case extension, dev readme.md, .js and doc pages, key material, solutions, PDFs, non-ASCII names, a symlinked folder) |
| `TestParityCourseExtractorSynthetic` | stdout, stderr, exit status and both repo trees: bound by a relative and an absolute `course_dir`, unbound (earlier copies removed; a symlink in their place refused), `course_dir` missing, unregistered root, rebuilt after the course changed, a copy that fails (mode 000 file), a named pipe in the course, a registry that is not JSON |
| `TestParityCourseReal` | plan rows and the copy of `sources/n4-dev-course` (334 files: 20 text, 169 code, 17 PDFs, 128 not copied; `HELPBUILD_PARITY_COURSE` replaces it); skipped with `-short` |

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

- pdftotext 26.10.0 ends **every** page with a form feed, the last one too, so
  `extract_text`'s `count('\f') + 1` reports one page more than the PDF has; its comment
  says the last page has none. `pages` in `pdf-index.json` is therefore N+1 (a blank
  one-page PDF is 0: its text strips to nothing).
- The parity driver shortens the 120 s timeout only for the timeout fixture
  (`HB_PDF_TIMEOUT` replaces `timeout=120` in the one `subprocess.run` call that passes it).

## Shared-helper candidates

`pyJoin` (`os.path.join` without cleaning), `pyAbspath`, `pyBasename`, `rmtreeIgnore`
(`shutil.rmtree(ignore_errors=True)`) and `commas` (`format(n, ",")`) are local to this
package; `pyJoin`/`pyAbspath`/`pyBasename` belong next to `pycompat.PosixJoin` once a
second package needs them.

`relayInterrupts`, `reraise` and the group kill in `procgroup_unix.go` are a copy of
`internal/secaudit/procgroup_unix.go`. Proposed: one `internal/procgroup` package
(`KillGroupOnCancel(cmd)`, `RelayInterrupts(cancel) (stop func())`, `Reraise(sig)` and
an `*InterruptError`) with the forking-child and signal tests once, used by secaudit and
helpbuild; until it exists both packages keep their copy.

## Dead code not ported

- `maintenance.ORPHAN_INDEXES` is `[]` (its cleanup loop never runs).
- `maintenance.BUILDERS` `skip_if` is always `None` (the skip branch is unreachable).
- `extract_version --no-index` has no caller.
- `bajadoc_parser.format_output` exists only as a re-export for `bajadoc_xml_parser`; both
  are retired in Phase 5.
- `doc_jars.fidelity_jar(report=None)` default is used only by tests.
