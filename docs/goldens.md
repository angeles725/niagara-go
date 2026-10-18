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
  tool prints there: for Windows, run both on the Windows host (Windows CPython against the
  pre-switch script, niagara-research `e21fe8825^`) from a copy of `testdata/` with the case's
  args and env. A difference there is a bug to fix in the port, not a case to bind. Text-mode
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
checked on the real corpus by `make parity` (`TestSearchQuality`).

## Freezing and regenerating

A Python golden has no update mode: it is Python's output, so the Go side never rewrites it
(Go-only cases, below, are the exception). Each package freezes its cases with a generator
next to them, run against the Python tool, for example:

```sh
python3 -I internal/typecatalog/testdata/freeze_golden.py
python3 -I internal/nav/testdata/gen_golden.py
python3 -I internal/bognav/testdata/convert_golden.py
```

The generator's docstring states its inputs. Phase 5 deleted the Python from the live
checkouts, so a generator reads it from a worktree of the source repo's `pre-go-retire` tag
(D47), named by one variable per repo:

| Variable | Repo | Pinned revision |
|---|---|---|
| `NIAGARA_RESEARCH_PY_REF` | niagara-research | `c846ce1e5` |
| `MODULE_NAVIGATOR_PY_REF` | module-navigator | `f897268` |
| `NIAGARA_HELP_PY_REF` | niagara-help | `8410d7451` |

Each defaults to `$HOME/niagara-research-worktrees/pre-go-retire-<repo>`; create it once with

```sh
git -C ~/niagara-research worktree add --detach ~/niagara-research-worktrees/pre-go-retire-niagara-research pre-go-retire
```

(the same with `-C <module-navigator or niagara-help checkout>` for the other two). Every
generator carries the same `py_ref` snippet: it refuses a worktree whose HEAD is not the pinned
revision or that has local changes or untracked files (also in `internal/secaudit/testdata/freeze_golden.py`) (ignored files such as `__pycache__` pass). Rerunning a generator from the tag must leave `git status`
clean; that is the proof its fixtures are still the Python's output.

A tag script that carries an escape runs with it: `MODULE_NAV_PYTHON=1` (module_nav.py is still
full Python at the tag) or `NIAGARA_HELP_PYTHON=1` (niagara_help.py is a shim there), set in
`os.environ` and in any subprocess environment a generator builds from scratch, which also passes
`NIAGARA_GO_BIN_DIR` through: a run with `NIAGARA_GO_BIN_DIR=/nonexistent` proves no Go port
answered. The real-corpus captures (`internal/{nav,help}/testdata/golden-real/capture.py`) take
the harness and the CLI from the tag and the data (indexes/, organized/, fulltext.sqlite,
versions/) from the live `MODULE_NAVIGATOR_DIR` / `NIAGARA_HELP_DIR`. Go tests reach the same
worktrees through `internal/pyref`: `Dir` (same pin and clean check; a missing worktree skips
unless `PYREF_REQUIRED=1`, which `make parity` exports, then fails), `Python` (python3; skips off
Linux, where the references write CRLF and hit cp1252) and `Env` (the escape). A Go test that
only checks a value the Python once recorded (a hash, an exit status) copies it
into its own testdata with the tag it came from instead of reading the reference: helpbuild's
`testdata/cov/cli-goldens.json` holds the coverage cases of niagara-help `tests/golden/cli.json`.

At the tag the switched entry scripts exec the Go ports (D35), so running `tools/<x>.py` would
record Go checking itself. The capture scripts that run a CLI (`internal/{bogcycles,bognav,
bogwrite,stationmodules,typecatalog}/testdata`) run it through `FROZEN`: the switch block is cut
out, which gives back the exact source of the parent of the switch commit, and the code runs as
`__main__` under the tool's own path and line numbers. The older converters export the frozen
tools from niagara-research at that parent commit (`git archive`), check each run against the
frozen JSON in text mode and write the exact bytes (`internal/{corpusnav,checkcoverage,
modulefind,gencatalog}/testdata/convert_golden.py`). A case that needs state git cannot hold (an empty root, a mode-000 file, a
CATALOG.md state) lives in its own `testdata/<set>/golden/` and a package test stages it in a
temporary directory before calling `golden.Run` on it. When a generator owns the cases it also writes
their `intended.json` (see `INTENDED` in `internal/nav/testdata/gen_golden_cases.py`), so a re-run
keeps them; a re-capture also keeps every `cmd.json` key it does not write (`gen_golden.py` owns
`args` and `env`, so a hand-added `requires` survives). Unit-test fixtures of shared packages follow the same rule (`testdata/gen.py`,
`gen_fixtures.py`).

In the Python suites of the source repos (niagara-research `tools/tests/goldencli.py`),
`GOLDEN_UPDATE=1` rewrites the Python goldens, and `GOLDEN_BIN_<TOOL>` (script name
without `.py`, upper-cased, `-` -> `_`, e.g. `GOLDEN_BIN_BOG_NAV`) runs a tool's Python golden
suite against the Go binary, and `GOLDEN_UPDATE` refuses to run with such an override so the
port's output is never frozen as the expectation.

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

## Search quality: `make parity`

`make parity` runs `TestSearchQuality` (internal/searchdoor/quality_test.go), the search quality
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
(0, 60) degC). The client data never enters this repo. Unset, the test skips; `make parity` sets
it to that file in the checkout (`make parity HDBREAD_REAL_HDB=` skips it), and a missing file fails.

make expands `$` in an overriding value, whether it comes from the command line or the
environment (`make -e` too): `UMA14$20Ret.hdb` reaches the test as `UMA140Ret.hdb`. Write each `$`
of a path as `$$` in either form (checked with GNU Make 4.3); `go test` takes the plain path:

```sh
make parity HDBREAD_REAL_HDB='/path/to/seg1/UMA14$$20Ret.hdb'
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
counts 9 indexes and prints no call-index line). `capture.py`'s `INTENDED` names them.

`palette-lexicon-agents` follows niagara-research `tools/palette-lexicon-agents.py`, not
`module_nav.py` (D39 e), so its cases come from that script: the kept
`palette-lexicon-agents` case and the four `palette-lexicon-agents-*` cases hold its output,
and `help-palette-lexicon-agents` keeps `module_nav.py`'s bytes with the Go help in
`intended.json` (the script's own `--help` names another program). They are written by
`internal/nav/testdata/gen_palette_lexicon.py --real bin/mnav`, which `capture.py` runs after
its own capture; the same script writes the hermetic cases under
`internal/nav/audits/testdata/palette/`. The real cases count corpus modules (the `--all`
census, "N module dirs"), so re-run `gen_palette_lexicon.py --real` after a corpus change.

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

mnav also needs the `organized/` corpus (`$NAV_CORPUS_BASE`, else next to the checkout) and
uses `niagara-help/` next to the checkout when it exists. nh must be the main checkout
(untracked files there change several counts, see the harness docstring).

To re-capture, run the generator next to the cases. It runs each kept case through the
harness's own `run_case`/`run_isolated`, with `MODULE_NAV_PYTHON=1` / `NIAGARA_HELP_PYTHON=1`
so a tag shim runs Python and never Go, refuses to write anything when a hash differs from
`cli.json`, and takes the Go binary for the `intended.json` output. The code (harness,
`cli.json`, the CLI) comes from the frozen `pre-go-retire` worktree, the data
(`MODULE_NAVIGATOR_DIR` / `NIAGARA_HELP_DIR`) from the live checkout ("Freezing and regenerating"):

```sh
make build
python3 -I internal/nav/testdata/golden-real/capture.py bin/mnav
python3 -I internal/help/testdata/golden-real/capture.py bin/nh
```

These cases replace both Python harnesses for the kept commands: once Phase 5 removes the
frozen Python tools, module-navigator and niagara-help `tests/golden_cli.py` (and their
`tests/golden/cli.json`) can be deleted (D39 (b)); the generators then run only from a
`pre-go-retire` worktree.
