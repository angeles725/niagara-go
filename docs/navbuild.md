# internal/navbuild: the module-navigator builders in Go

`internal/navbuild` is the Go port of the module-navigator index builders
(`tools/build_*.py`, Phase 4 of the Go migration, decision D33). It is a **non-printing
library**: each builder returns typed results and errors and writes only its index file;
the summaries the Python builders print belong to the CLI layer (`mnav reindex`, slice B8).
Only the indexes that kept commands read are ported (D32).

Source of truth: module-navigator `tools/` (Python 3.15). Where the Python code and this
document disagree, the code wins and this document is corrected.

## Slice plan

| Slice | Scope | Python source |
|---|---|---|
| B1 | foundations (JSON layouts, `_meta`, atomic write, normalisation, parity driver) + module inventory | `build_module_inventory.py`, `module_nav_lib/build.py` |
| B2 | `declaration_header` / `strip_comments` + class index | `build_class_index.py`, `module_nav_lib/java_text.py` |
| B3 | inheritance + xref | `build_inheritance.py`, `build_xref.py` |
| B4 | token index (SQLite) | `build_token_index.py` |
| B5 | Java member scanner + method index | `build_method_index.py` |
| ~~B6~~ | field index: dropped, no kept reader (D32) | `build_field_index.py` |
| **B7** | call graph | `build_callgraph_index.py` |
| B8 | `mnav reindex` orchestration | `reindex.sh` |

## Foundations

| Piece | Ports | Notes |
|---|---|---|
| `Indented`, `Compact` | `json.dump(indent=2, ensure_ascii=False)`, `json.dump(separators=(",", ":"), ensure_ascii=False)` | `pyjson.DumpOpts` |
| `jsonWriter` | the same layouts, streamed | each member value is `pyjson.Dumps` on its own, re-indented to its depth, so a 33 MB index never exists as one `pyjson` tree |
| `stamped` | the last two `_meta` members of every builder | `build_time_sec` = `round(elapsed, 1)` (ties to even, as Python), `timestamp` = local `%Y-%m-%dT%H:%M:%S` |
| `writeIndex` | `build.write_json` | `helpbuild.WriteText`: temp file + rename; the temp name is `.tmp-*<name>` instead of `<name>.tmp`, and the text is written without newline translation (Python's text mode would write CRLF on Windows) |
| `NormalizeVolatile` | — | replaces `timestamp`, `build_time_sec`, `source` and `source_root` inside the leading `_meta` object only |

Builders take the base directory explicitly and write `<base>/indexes/<index>`; how the
CLI finds the base directory (D24) is B8's concern.

## Module inventory

`ScanInventory(organized)` is `build_inventory`: `core.IterSubmodules`, one entry per
submodule in dict order (a submodule name seen twice keeps its first position and its
last value), `pipeline/fase1-recon.json` read as `utf-8-sig` JSON (any read or decode
error = no recon; the existing-but-unreadable ones are listed for the CLI's `--verbose`
warning), recon values copied as decoded (`class_count` 1.5e2 stays the float 150.0, a big
int stays exact). `scanPackages` walks `vineflower/` as `os.walk` does (symlinked
directories listed but not entered, a dangling `*.java` link counted).

Intended difference: a `fase1-recon.json` that is valid JSON but not an object is an error
here; Python dies with `AttributeError`.

## Class index

`ScanClasses(organized, zkm)` is `build_class_index`: `core.IterJava` order, each file
read with `errors="replace"` and universal newlines and split as `readlines()`
(`pycompat.Lines`: `\f`, `\v`, U+2028 are not line breaks), the package from the first 30
stripped lines, block/line comment skipping by line prefix, `core.MatchDecl` per line, and
`declarationHeader` (`java_text.declaration_header`: up to 40 lines to the first brace
outside comments, `commentSpans`/`stripComments` ported byte-wise, whitespace collapsed
with Python's `str.split()`). Files are parsed by four workers and merged in corpus order.
`LoadZKMModules` reads `<base>/indexes/module-inventory.json` like `load_zkm_modules`
(absent = all flags false and `zkm_source: null`).

Python quirks kept on purpose (they are what the index holds today): an indented
declaration after a top-level one is its inner type even when it is a separate top-level
type at another indent (`\tclass Tabbed` after `class NB`); `implements A extends B` gives
`extends: ""` (a reversed slice) and `implements: ["A extends B"]`; `extends ,` gives
`null`; a header without a brace in 40 lines keeps every `implements` word.

`core.MatchDecl` does not return the match end that `declaration_header` starts from;
`declEnd` recovers it (the name after Modifiers+Kind and whitespace, confirmed by
re-matching the cut line, which skips the same text inside an annotation argument).
Export requests to the owners: `core.MatchDecl`'s end offset (Integrante2) and the
`nav/license` `commentSpans` twin (one shared Java-text helper).

## Inheritance

`readClassIndex` reads `<base>/indexes/class-index.json` with Python `json.load` semantics
(`index.LoadPython`) and flattens `classes` in dict order; a missing index is an error
naming `./reindex.sh` like `build.read_json`, and a member of an unexpected JSON type is
an error (Python would die or key a dict by it). `ComputeInheritance` is
`build_inheritance`: the top-level entries (falsy `outer_class`), children and
implementors deduplicated in first-seen order and then sorted, the first `extends` of a
name wins for the chains, a cycle stops a chain and counts once per walk.

## Xref

`ComputeXref` is `build_xref`: the package, FQCN and class -> modules maps from the
top-level entries, then the import header of each top-level entry's source (the first
200 lines, `content.split("\n", 200)[:200]`; four workers read the files, the merge
keeps entry order). `findImports` is `RE_IMPORT.finditer` with `re.MULTILINE` as a hand
scanner: a match is tried at each line start, `^\s*` may cross blank lines, `static\s+`
is tried first and on failure `static` is read as the start of the name, and only two
lengths of the greedy `[\w.]+` can succeed (the whole run, or all but a final `.` that
starts `.*`), so `import g.h.;` yields `g.h.` as in Python. The reverse and module maps
are key-sorted, matching module-navigator after `fix/deterministic-xref-base-dir`
(before it, their key order followed CPython set order and changed with
`PYTHONHASHSEED`). `avg_imports_per_class` is the int `0` when no class imports
anything, as Python's `else 0`.

## Token index

`BuildTokenIndex(base, organized, minLen)` is `build_token_index.build_index`: the
`corpus.top_level_files` of the class index (`topLevelFiles`: the first entry per path
with a falsy `inner` and an `outer_class` of null, `""` or `"None"`) whose source exists,
each read as `readlines()` with `errors="replace"` (universal newlines), tokenized line by
line (`tokenizeLine`: the runs of `[a-zA-Z0-9_]`, an ASCII class, so every byte of a
non-ASCII character splits; at least two characters; not one of the 60 `STOP_WORDS`; not
`var\d+`), one posting per distinct token of a line of at least `--min-len` characters.
`files.id` is the position in the scan list + 1, so an unreadable or empty file leaves a
gap; `files_with_tokens` counts a file with any token before the `--min-len` filter, as
Python does. Up to four workers (`min(4, GOMAXPROCS)`) tokenize; one goroutine inserts in scan order inside one
transaction (300-row multi-value `INSERT`s), then creates the three indexes, writes `meta`
(`timestamp` as `%Y-%m-%d %H:%M:%S`, `build_time_sec` as `str(round(x, 1))`), runs
`ANALYZE` and `VACUUM`, and renames `<index>.tmp` into place (a stale temp database of a
killed build is removed first; a failed build removes its own and leaves the old index).
Like Python's fixed `.tmp` name, this means two token builds into one base dir at once
break each other; `mnav reindex` runs the builders one at a time.
As in Python, `temp_store=MEMORY` keeps the index sorts in RAM: the full-corpus build
peaks at about 2 GB.

The writer opens SQLite through `modernc.org/sqlite` directly: `internal/index` only opens
databases read-only (as `internal/helpbuild/fulltext` does). The `CREATE TABLE` texts are
Python's triple-quoted strings byte for byte (`sqlite_master` keeps them). The rollback
journal is off during the build (Python uses WAL); the file left behind is in rollback
mode either way.

Parity is logical (`token_test.go` `tokenDump`): `sqlite_master`, `meta` in rowid order
with `timestamp`/`build_time_sec`/`source_root` blanked, `sqlite_stat1` and `sqlite_stat4`
(both SQLite builds have STAT4, so the readers get the same query plans), `files` by id
and `postings` in rowid order, all through `quote()`; the full-corpus run compares the two
streams record by record without holding either.

Intended difference: a scanned file whose entry has `"module": null` is an error naming it
where Python dies with `IntegrityError` (`files.module` is `NOT NULL`).

## Java member scanner

`javamembers.go` ports the line scanner of `build_method_index.py` that the method index
and the call graph share: `countBraces` (`{`/`}` outside `"..."`/`'...'` literals and
before `//`), `dropBlockComments`, `splitParams`, `extractParamType`, `tryParseMethod` and
`parseMethods` (`parse_methods_from_content`). It walks `content.split("\n")` with a class
stack of `(name, body depth)` frames: closing braces are counted on the whole line before
the frame check, a declaration line pushes `core.TypeDeclName` unless it starts with `//`
or an annotation, and a method is tried on a line that starts with a space or tab, does
not start with `//` or `@`, and holds a `(`. Python's quirks are kept because they are
what the indexes hold: a one-line `{ ... }` body at class depth pops the class (later
members fall back to the file's class); the continuation lines of a multi-line parameter
list are skipped, braces included; a line starting with `@` (an `@interface` member, an
annotated method on one line) is never a method; a declaration without return type and
modifiers that is not a constructor is an enum constant and skipped, its continuation
lines scanned again. `\w`, `isalnum`, `isalpha`, `strip` and `split` are Python's
Unicode ones (`pycompat`).

Intended difference: `extract_param_type` raises `IndexError` on an annotation with
arguments and nothing after it (`@A(x y)`); here the type is empty and dropped like any
empty type.

## Method index

`BuildMethodIndex(base, organized)` is `build_method_index`: the top-level files of the
class index (a missing or unreadable one counts in `files_failed`), parsed by four workers
and merged in index order. Each declaration is encoded to its compact JSON entry
(`class, module, signature, line, modifiers, return_type, params`) in the worker, so the
merge holds the 143 MB of entries as strings that do not pin the sources; `class_methods`
holds sorted name lists. The index is streamed to a temp file through a 1 MiB buffer
(`writeIndexStream`, temp + rename, mode 0644), never built as one string.

`_meta.max_methods_class` keeps Python's tie-break without its rescan of every class after
every file: after each file only the classes it touched can exceed the running maximum,
and they are visited in first-seen order, so the result is the first class in first-seen
order whose count is strictly greater than the maximum when that file was merged
(`TestMaxClassTieBreak`). An average over nothing is the int `0`, as Python's `else 0`.

The field index is not ported: no kept command reads `field-index.json` (D32/D33; `stats`
then shows it missing, an accepted difference). The method index stays because
`license-feature`'s miss hints read the `methods` keys.

## Call graph

`BuildCallgraph(base, organized)` is `build_callgraph_index`: `parent_map` from
`class_to_chain` of `inheritance.json` (absent = super calls resolve to the caller's own
class, as Python's warning path), then for each top-level file the method zones of
`parseMethods` (from a declaration line to the next one) scanned by `zoneCalls`
(`extract_calls_from_zone`): comments dropped, string and char literals emptied (the two
`re.sub`), then `this.m(`, `super.m(`, `super(`/`this(` not after a `.`, `new Name(`/`<`
with an upper-case first letter, and `Recv.m(` with an upper-case receiver outside
`SKIP_RECEIVERS`. The regular expressions are hand matchers with Python's Unicode `\w`,
`\s` and `\b`; each is unambiguous, so trying every start position and resuming after a
match is `re.finditer` (a receiver can start mid-word: `Members.this.run(` yields
`this.run`). The import map Python builds is never used and is not ported. Keys and
callee lists are sorted; the index is streamed like the method index.

Intended difference: a `class_to_chain` value that is not a list (Python would take the
first character of a string) or a chain whose first element is not a string is an error.

## Gates

| Gate | Result (2026-10-17) |
|---|---|
| full-corpus `module-inventory.json` normalised byte diff | 0 (365,332 bytes) |
| full-corpus `class-index.json` normalised byte diff | 0 (34,144,621 bytes); Python 32.3 s, Go 3.4 s on a loaded host |
| full-corpus `inheritance.json` normalised byte diff | 0 (3,740,761 bytes); Python 1.6 s, Go 0.5 s |
| full-corpus `xref-index.json` normalised byte diff | 0 (19,290,729 bytes); Python 12.3 s, Go 4.7 s |
| full-corpus `token-index.db` logical dump | 0 differences in 15,581,950 records (53,324 files, 15,528,521 postings; 1,006,706,688 bytes, the same size as Python's); Python 3m13s, Go 2m49s on a loaded host |
| `selftest --recall` on the Go-built index (`TestRecallOnGoBuiltIndex`; the Python oracle via `--base-dir`, since the Go `mnav selftest` is not ported) | 100.00% (55,931/55,931), supertypes 44,369/44,369 |

## Parity harness

`parity_test.go` runs the real Python builder (`python3 -E -B tools/<builder> --base-dir
<temp>`, `NAV_CORPUS_BASE=<corpus>`; `-E` leaves the hash seed random) and the Go builder
into another temp base over the same corpus, then byte-diffs both indexes after
`NormalizeVolatile`. It needs a module-navigator checkout whose builders take
`--base-dir` (branch `fix/deterministic-xref-base-dir`): `$NAVBUILD_PY_NAV`, else
`$NIAGARA_RESEARCH/module-navigator`; it skips visibly otherwise.

| Corpus | When |
|---|---|
| `testdata/organized` (synthetic: Java headers with generics, comments, annotations, records, sealed types, CRLF/CR, invalid UTF-8, U+00A0, `$` names, line-break characters, a late package line; recon files with BOM, invalid JSON and UTF-8 recon files, `{}` recon, float and big-int recon values, a duplicate submodule name, unknown suffix, standalone submodule, a non-submodule dir, the `com.niagaraX` prefix quirk, a non-ASCII package) | always |
| real modules linked into a temp `organized/` (`$NAVBUILD_PARITY_MODULES`, default `alarm,backup,ColdRoomPan,CentralineAhuPx,kitControl`) | not with `-short` |
| the whole corpus (`$NAV_CORPUS_BASE`, else `$NIAGARA_RESEARCH/organized`) | `NAVBUILD_PARITY_FULL=1` |

`TestParityLaterIndexes` builds the chain (inventory, class index, then every later
builder in `laterBuilders`) on both sides; `$NAVBUILD_PARITY_ONLY` (comma-separated index
names) limits the later indexes it checks (inheritance always runs, the call graph reads
it). Each run logs wall time and peak RSS: on the full corpus the Go builder runs in a child
process (the test binary re-executed through `TestMain`), and both sides run under GNU
`/usr/bin/time` when it exists, because Linux keeps a process's RSS high-water mark across
`exec`, so the rusage of a child of the (large) test process would report the parent's peak.

`testdata/golden/<index>.golden` hold the normalised PYTHON outputs of the synthetic corpus;
`TestGoldenSynthetic` checks the Go builders against them without Python. Regenerate with
`go test ./internal/navbuild -run Parity -args -update` (Python and the module-navigator
checkout required), then rerun without `-update`.
