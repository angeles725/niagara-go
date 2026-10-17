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

## Freezing and regenerating

There is no update mode in this repository: a golden is Python's output, so the Go side never
rewrites it. Each package freezes its cases with a generator next to them, run against the
Python tool, for example:

```sh
NIAGARA_RESEARCH=~/niagara-research python3 -I internal/typecatalog/testdata/freeze_golden.py
python3 -I internal/nav/testdata/gen_golden.py
NIAGARA_RESEARCH=~/niagara-research python3 -I internal/bognav/testdata/convert_golden.py
```

The generator's docstring states its inputs. When a generator owns the cases it also writes
their `intended.json` (see `INTENDED` in `internal/nav/testdata/gen_golden.py`), so a re-run
keeps them. Unit-test fixtures of shared packages follow the same rule (`testdata/gen.py`,
`gen_fixtures.py`).

`GOLDEN_UPDATE=1` belongs to the Python suites of the source repos
(niagara-research `tools/tests/goldencli.py`). There, `GOLDEN_BIN_<TOOL>` (script name
without `.py`, upper-cased, `-` -> `_`, e.g. `GOLDEN_BIN_BOG_NAV`) runs a tool's Python golden
suite against the Go binary, and `GOLDEN_UPDATE` refuses to run with such an override so the
port's output is never frozen as the expectation.
