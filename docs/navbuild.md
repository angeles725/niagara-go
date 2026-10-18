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
| **B1** | foundations (JSON layouts, `_meta`, atomic write, normalisation, parity driver) + module inventory | `build_module_inventory.py`, `module_nav_lib/build.py` |
| B2 | `declaration_header` / `strip_comments` + class index | `build_class_index.py`, `module_nav_lib/java_text.py` |
| B3 | inheritance + xref | `build_inheritance.py`, `build_xref.py` |
| B4-B7 | token (SQLite), method, field, callgraph | `build_token_index.py`, ... |
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

## Parity harness

`parity_test.go` runs the real Python builder (`python3 -E -B tools/<builder> --base-dir
<temp>`, `NAV_CORPUS_BASE=<corpus>`; `-E` leaves the hash seed random) and the Go builder
into another temp base over the same corpus, then byte-diffs both indexes after
`NormalizeVolatile`. It needs a module-navigator checkout whose builders take
`--base-dir` (branch `fix/deterministic-xref-base-dir`): `$NAVBUILD_PY_NAV`, else
`$NIAGARA_RESEARCH/module-navigator`; it skips visibly otherwise.

| Corpus | When |
|---|---|
| `testdata/organized` (synthetic: BOM, invalid JSON and UTF-8 recon files, `{}` recon, float and big-int recon values, a duplicate submodule name, unknown suffix, standalone submodule, a non-submodule dir, the `com.niagaraX` prefix quirk, a non-ASCII package) | always |
| real modules linked into a temp `organized/` (`$NAVBUILD_PARITY_MODULES`, default `alarm,backup,ColdRoomPan,CentralineAhuPx,kitControl`) | not with `-short` |
| the whole corpus (`$NAV_CORPUS_BASE`, else `$NIAGARA_RESEARCH/organized`) | `NAVBUILD_PARITY_FULL=1` |
