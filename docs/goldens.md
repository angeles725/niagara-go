# Golden CLI cases

A golden case is one frozen invocation of the Python tool: its exact stdout, stderr and exit
code. The Go port must reproduce those bytes. The runner is `internal/golden`
(decision D7 in niagara-research `odd/tasks/go-migration.md`).

## Layout

One directory per case under `<package>/testdata/golden/<case>/`:

| File | Contents |
|---|---|
| `cmd.json` | `{"args": [...], "stdin_file"?, "env"?, "cwd_fixture"?, "requires"?}`; unknown fields are an error |
| `stdout` | expected standard output, exact bytes (required) |
| `stderr` | expected standard error, exact bytes (missing file = empty) |
| `exit` | expected exit code as decimal text |
| `intended.json` | optional intended difference (below) |

- `stdin_file` and `cwd_fixture` are relative to `testdata/`. The command runs in `testdata/`,
  or in `testdata/<cwd_fixture>`.
- Every case gets `COLUMNS=100` and `NO_COLOR=1`; the case's `env` overrides them.
- `{tmp}` in `args` or `env` values becomes a fresh temporary directory per case. That path
  in the command's stdout/stderr is folded back to `{tmp}` before comparing.
- `"requires": {"machine_id_sha256": "<hex>"}` binds a case to the capture host (SHA-256 of the
  trimmed `/etc/machine-id`). On any other host the case is skipped with a reason, never passed.
- `"requires": {"goos": ["linux", ...]}` binds a case to the listed operating systems
  (`runtime.GOOS` values). On any other OS the case is skipped with a reason, never passed.
  Use it for a case whose frozen bytes cannot hold on that OS (for example `make wintest` on
  Windows) instead of a skip in the package test. An empty list or an unknown GOOS name fails
  the load. Both requirements may be combined; each must hold.
  Bind a case only after checking that the port's output on that OS is what the frozen Python
  tool printed there (the cases bound so far were checked against Windows CPython on the
  pre-switch script, niagara-research `e21fe8825^`, before D47 retired the Python); a
  difference there is a bug to fix in the port, not a case to bind. Text-mode
  `print` writes CRLF on Windows and the ports write LF; that difference is not checked.

Cases run sequentially, sorted by directory name. A package wires them with one test:

```go
func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }
```

## Intended differences: `intended.json`

`stdout`, `stderr` and `exit` always hold what Python printed. When the port deliberately
differs (for example one error line where Python crashed with a traceback), the case declares
it next to the fixture:

```json
{"reason": "Python raises TypeError on a null bytecode; the port prints the whole block",
 "stdout": "...", "stderr": "", "exit": 0}
```

- `reason` is mandatory (a blank reason fails the load).
- At least one of `stdout`, `stderr`, `exit` must be present; each replaces that expectation.
- Unknown fields fail the load. The reason is logged (`go test -v`) on every run.

## Exec mode: `GOLDEN_BIN`

With `GOLDEN_BIN` set, every case runs that binary instead of the in-process `Main`, with
`os.Environ()` plus the case env and a 60 s limit per case:

```sh
make build
GOLDEN_BIN=$PWD/bin/hdbread go test ./internal/hdbread -run TestGolden
```

Pass an absolute path: `go test` runs in the package directory, so a relative path such as
`bin/hdbread` resolves against `internal/hdbread/`. A bare name is looked up on `PATH`.

The nsearch/find3 goldens (internal/searchdoor) skip in exec mode: their backends are the Go
port of the fake scripts (`scriptFakes`), which exist only in process. The nsearch binary is
checked on the real corpus by `make realdata` (`TestSearchQuality`).

## Freezing and regenerating

The Python goldens were frozen by generators next to them that ran the Python tool from a
worktree of the source repo's `pre-go-retire` tag (D47). With the Go ports the only
implementation, the generators, `internal/pyref` (which found those worktrees) and `make
pyparity` (the Go-vs-Python suites) are retired: the committed goldens are the permanent oracle
and no test runs the Python tools. The tag stays in each source repo (niagara-research
`c846ce1e5`, module-navigator `f897268`, niagara-help `8410d7451`) for a rollback or a
question about the old behaviour, and this repository's history keeps the generators.

A golden changes only on purpose, from the Go side:

1. Make the change and run the package's goldens: each failing case shows the new bytes.
2. Regenerate the expectation from Go. A case under `testdata/golden/<case>/` holds Python's
   bytes and `golden.Run` never rewrites it: run the Go binary (`make build`) with the case's
   `cmd.json` arguments in `testdata/` (or `testdata/<cwd_fixture>`), with `COLUMNS=100`,
   `NO_COLOR=1` and the case `env`, and write its `stdout`, `stderr` and `exit`; when the case
   has an `intended.json`, rewrite that file's fields instead and keep the Python bytes. A
   Go-only case is rewritten by `GOLDEN_UPDATE=1` (below). Test-data goldens have their own
   update flags: helpbuild `-update-frozen` (`testdata/frozen/`), `-update-s8`, `-update-s9`
   and `-update-cov`, navbuild and helpbuild/fulltext `-update`. A fixture a retired
   generator wrote (the `testdata/*.json` of the shared packages) is edited from the Go
   output.
3. Review the diff and list each intended change, with its reason, in the commit message.

A golden never changes only because Go now prints something else: a difference no one
intended is a bug in the port. A fixture once cut from live indexes stays frozen input
(`internal/nav/testdata/fixture/nav/`, whose `_meta` timestamps the goldens print).

The capture scripts ran a CLI at the tag through `FROZEN` (the switch block that exec'd the
Go port cut out) or exported the frozen tools from niagara-research at the parent of the
switch commit (`git archive`); a case that needs state git cannot hold (an empty root, a
mode-000 file, a CATALOG.md state) lives in its own `testdata/<set>/golden/` and a package
test stages it in a temporary directory before calling `golden.Run` on it. A Go test that only
checks a value the Python once recorded (a hash, an exit status) keeps a copy of it in its own
testdata with the tag it came from: helpbuild's `testdata/cov/cli-goldens.json` holds the
coverage cases of niagara-help `tests/golden/cli.json`.

In the Python suites of the source repos (niagara-research `tools/tests/goldencli.py`, deleted
in Phase 5 and kept at the `pre-go-retire` tag), `GOLDEN_UPDATE=1` rewrote the Python goldens,
and `GOLDEN_BIN_<TOOL>` (script name without `.py`, upper-cased, `-` -> `_`, e.g.
`GOLDEN_BIN_BOG_NAV`) ran a tool's Python golden suite against the Go binary; `GOLDEN_UPDATE`
refused to run with such an override so the port's output was never frozen as the expectation.

## Go-only cases (D34)

Behaviour added to a port after its Python tool froze has no Python output to freeze. Its cases
live in a second set, `<package>/testdata/go/golden/<case>/`, with the same layout, run by
`golden.RunGo(t, Main)`, but `stdout`, `stderr` and `exit` hold the reviewed Go output. Their inputs are synthetic, built by a Go test helper and committed under
`testdata/go/inputs/`; a test fails when a committed input differs from what the helper builds.
A changed Go-only output is regenerated, then reviewed in the diff before it is committed:

```sh
GOLDEN_UPDATE=1 go test ./internal/hdbread -run 'TestGoInputs|TestGoldenGo'
go test ./internal/hdbread -run 'TestGoInputs|TestGoldenGo'
```

With `GOLDEN_UPDATE` set (any non-empty value), `golden.RunGo` first runs each case in process,
each in its own subtest so its working directory and env are restored, and rewrites its
`stdout`, `stderr` and `exit` (a new case needs only its `cmd.json`; an empty stderr removes the
file); then it checks the cases as usual. Update mode logs one line saying it is on and one
`golden: rewrote <path>` per file whose bytes changed (`go test -v` shows them), so a
`GOLDEN_UPDATE` left exported is visible; review that diff before committing. The package's input helper rewrites
`testdata/go/inputs/` under the same variable (`golden.Updating()`, hdbread `TestGoInputs`).
Nothing outside `testdata/go/` is written: `golden.Run` never rewrites a Python case. A Python case
whose output the new behaviour changes keeps its Python bytes and declares the Go output in
`intended.json` with a D34 reason (hdbread `records-audit-type`, `usage-help`).

## Search quality: `make realdata`

`make realdata` runs `TestSearchQuality` (internal/searchdoor/quality_test.go), the search quality
gate. It runs the search in process on the real corpus (`NIAGARA_RESEARCH`, default
`~/niagara-research`) and measures hit@1/hit@5 per source over
`testdata/quality/golden-queries.jsonl` (99 questions, docs pinned to 4.14.0.162) and the warm
p50/p95 latency. The review that set each question's expected locations, with one evidence line
per added or removed location, is `testdata/quality/golden-queries-review.md`, kept next to the
fixture. Each `qualitySets` range has
its own floor (q01-q81 the S3.1 baseline, q82-q99 the D41 course rows), so new rows cannot hide a
regression of the earlier ones. A hit count below a floor fails; a higher one is logged so the
floor can be raised. Without
`NSEARCH_QUALITY_ROOT` the test skips. `TestQualityFixtureMatchesResearch` fails when the copied
question set drifts from the niagara-research one (skipped when that file is absent).

## Real history data: hdbread

`TestRealHarborHistory` (internal/hdbread) replaces the deleted Python `RealHarborHistoryTest`
(niagara-research `pre-go-retire:tools/tests/test_t6c_hdb_trend.py`): it decodes a real .hdb
history named by `HDBREAD_REAL_HDB` and checks the HARBOR UMA14 Ret history (682 records, the
first at 2018-09-26T17:30:00.008Z, ascending timestamps, a 15-minute median step, every value in
(0, 60) degC). The client data never enters this repo. Unset, the test skips; `make realdata` sets
it to that file in the checkout (`make realdata HDBREAD_REAL_HDB=` skips it), and a missing file fails.

make expands `$` in an overriding value, whether it comes from the command line or the
environment (`make -e` too): `UMA14$20Ret.hdb` reaches the test as `UMA140Ret.hdb`. Write each `$`
of a path as `$$` in either form (checked with GNU Make 4.3); `go test` takes the plain path:

```sh
make realdata HDBREAD_REAL_HDB='/path/to/seg1/UMA14$$20Ret.hdb'
HDBREAD_REAL_HDB='/path/to/seg1/UMA14$20Ret.hdb' go test -run TestRealHarborHistory ./internal/hdbread
```

## Real-corpus goldens: mnav and nh keep sets

`internal/nav/testdata/golden-real/` and `internal/help/testdata/golden-real/` hold the
keep-set cases (decision D32) of the two hashed Python golden suites, module-navigator and
niagara-help `tests/golden_cli.py`, as real bytes: 70 mnav cases (48 command cases, 21
`help-<command>`, `help`) and 61 nh cases (42 command cases, 17 `<command>--help`, `help`,
`no-command`). Retired-command ids are dropped. `help` (and nh `no-command`) keep Python's
output and declare the Go one in `intended.json` (reason D32: only kept commands are listed);
nh `stats` and `stats-json` do the same with reason I3-5 (`call-index.json` is retired: nh
counts 14 indexes and prints no call-index line). mnav `stats` (golden-real and the hermetic
`nav-stats`, `nav-stats-sparse`) lists the registry's runnable commands instead of Python's
hand-kept list (D32), and golden-real counts the 16 files of niagara-help `indexes/` (I3-5, NH3-NH5, NH7b, NH9).

`palette-lexicon-agents` follows niagara-research `tools/palette-lexicon-agents.py`, not
`module_nav.py` (D39 e), so its cases come from that script: the kept
`palette-lexicon-agents` case and the four `palette-lexicon-agents-*` cases hold its output,
and `help-palette-lexicon-agents` keeps `module_nav.py`'s bytes with the Go help in
`intended.json` (the script's own `--help` names another program). The retired
`gen_palette_lexicon.py` wrote them and the hermetic cases under
`internal/nav/audits/testdata/palette/`. The real cases count corpus modules (the `--all`
census, "N module dirs"), so a corpus change changes them: regenerate them from `bin/mnav`
("Freezing and regenerating").

The case layout is the one above, but the stored bytes are the harness's normalized output,
whose SHA-256 is the `cli.json` hash: mnav replaces the scratch base, run directory, `organized/`
corpus and checkout paths with `<BASE>`, `<RUN>`, `<CORPUS>`, `<NAV>`, elapsed times, dates
and index sizes with `<T>`, `<DATE>`, `(<MB>)`, and drops corpus staleness warnings; nh
replaces the scratch directory and checkout with `<TMP>` and `<REPO>` and elapsed times with
`<T>`. `{tmp}` in `args` is the mnav `--base-dir` scratch directory, or the nh scratch
directory. `TestGoldenReal` in each package runs the cases the way the harness does (same cwd,
`COLUMNS=100`; for mnav a fresh `--base-dir` linking `indexes/`) and normalizes the Go output
with the same rules (`internal/pyre`, so Python's regex classes) before comparing. `GOLDEN_BIN`
runs a binary instead of `Main`.

The tests need the real checkouts and are skipped (with the reason) when the variable is unset,
so `make ci` skips them; set, a missing checkout or corpus fails:

```sh
MODULE_NAVIGATOR_DIR=~/niagara-research/module-navigator go test ./internal/nav -run TestGoldenReal
NIAGARA_HELP_DIR=~/niagara-research/niagara-help go test ./internal/help -run TestGoldenReal
```

The same `NIAGARA_HELP_DIR` gates the nh checks carried over from niagara-help
`tests/test_search_quality.py` and `tests/test_corpus_invariants.py` before their deletion
(`go test ./internal/help -run 'Quality|Invariant'`, on the 4.14.0.162 root corpus).
`TestQualityReal*` score the golden doc questions (a hermetic copy in
`internal/help/testdata/quality/`, its header names the source commit) and fail when hit@1 /
hit@5 fall below the Python engine's floors; `TestInvariantReal*` assert the Baja bedrock facts
through `class`, `slots`, `find` and `stats`, and through `inheritance.json` for the retired
`hierarchy` and `implementors`. The synthetic ranking rules of the first file live in
`internal/help/fts/ranking_test.go` and need no corpus.

mnav also needs the `organized/` corpus (`$NAV_CORPUS_BASE`, else next to the checkout) and
uses `niagara-help/` next to the checkout when it exists. nh must be the main checkout
(untracked files there change several counts, see the harness docstring).

The cases are frozen: `golden-real/capture.py` of each package (it ran the kept cases through
the tag's harness and refused to write when a hash differed from `cli.json`) retired with D47.
A change is regenerated from the Go binary as for any case ("Freezing and regenerating"), with
the normalization of `TestGoldenReal` applied to the bytes it writes.

These cases replaced both Python harnesses for the kept commands: Phase 5 deleted
module-navigator and niagara-help `tests/golden_cli.py` and their `tests/golden/cli.json`
(D39 (b)).
