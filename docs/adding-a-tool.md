# Adding a tool

How a new Go-native command is added here. The Python tools are retired (Phase 5), so a new
tool has no Python to match; the Python goldens and the `pre-go-retire` worktree serve only the
parity of the tools already ported. The rules are decisions D2-D4, D7, D18, D29, D34 and D42 in
niagara-research `odd/tasks/go-migration.md`.

## 1. Layout

```
cmd/<tool>/main.go      one directory per installed command (D3: one binary each)
internal/<pkg>/         the tool's logic, split by command group
internal/<pkg>/testdata the tool's fixtures and golden cases
```

`cmd/<tool>/main.go` holds nothing but the entry point (D2):

```go
// Command bog-nav navigates a Niagara N4 station config.bog (or a bare file.xml), read-only.
package main

import (
	"github.com/angeles725/niagara-go/internal/bognav"
	"github.com/angeles725/niagara-go/internal/cli"
)

func main() { cli.Run(bognav.Main) }
```

`internal/<pkg>` exports `Main(args []string, s cli.Stdio) int`. `cli.Run` passes the process
arguments and stdio and exits with the returned code. Return `cli.ExitFound` (0),
`cli.ExitNoMatch` (1) or `cli.ExitCannotSearch` (2) (a ported tool keeps the codes its Python used).
Write only to `s.Out` and `s.Err`, so tests run `Main` in process.

A tool never imports another tool's package. There are two exceptions. D29: a library
subpackage `internal/<tool>/<lib>` that does not print, returns typed rows, does not depend on the
tool's env or registry, and exports the formatter for its CLI line. D42: nsearch
(`internal/searchdoor`) renders whole mnav command outputs through `nav.RunAt`, which runs
`nav.Main` with the checkout pinned per call and no global state. Shared code goes in a shared
package (see the README table); ask its owner before adding to it.

## 2. Command line

Build the parser with `internal/argparse` (`argparse.New`, `p.Add`, `p.Parse`) so help, usage,
errors and exit codes match Python byte for byte; see [argparse.md](argparse.md) for the
supported subset. `--version` prints `version.Line("<tool>")` (D18): the Go build's own VCS
stamp, not the Python checkout's.

## 3. Goldens

1. Write each case as a Go-only golden (D34): `testdata/go/golden/<case>/cmd.json`, with
   synthetic inputs built by a Go test helper and committed under `testdata/go/inputs/`.
2. Wire them: `func TestGoldenGo(t *testing.T) { golden.RunGo(t, Main) }`.
3. Generate the expected bytes from the Go code with
   `GOLDEN_UPDATE=1 go test ./internal/<pkg> -run 'TestGoInputs|TestGoldenGo'`, and have a human
   review the diff before committing; never edit `stdout`/`stderr`/`exit` by hand.
4. After `make build`, run the same cases against the binary:
   `GOLDEN_BIN=$PWD/bin/<tool> go test ./internal/<pkg> -run TestGoldenGo`.

A tool ported from Python also keeps its frozen Python cases under `testdata/golden/`
(`golden.Run(t, "testdata", Main)`), which a generator re-captures only from the
`pre-go-retire` worktree (D47); each deliberate divergence from Python gets an `intended.json`
with a mandatory `reason`.

Details: [goldens.md](goldens.md).

## 4. Size gate

`internal/sizegate` fails `go test` when a `.go` file outside `testdata/`, or a `.py` file
anywhere (the generator and parity scripts under `testdata/` included), exceeds 500 lines
(D4). `internal/sizegate/allowlist.txt` lists the exceptions as `<path> <max-lines>`, each
after a `#` line with its reason; entries may only be lowered or removed, and the test fails
on an entry that is too high or matches no file. Split the file instead of adding an entry;
the only accepted new entry is a frozen reference generator that D39(d) moves to the
`pre-go-retire` tag. The list is empty today: its last entry,
`internal/nav/testdata/gen_golden.py`, was split under the limit.

## 5. Checks

```sh
make hooks     # once per clone/worktree: pre-commit runs gofmt, go vet, go test -short
make check     # gofmt + vet + full tests
make lint      # staticcheck (pinned in the Makefile)
make cover     # coverage across ./internal/... and the total
make deadcode  # report-only list of functions no command reaches
make cross     # linux-amd64 and windows-amd64 binaries
make wintest   # the test suites as Windows processes (WSL only; not part of ci)
```

`make ci` (check, lint, cross) is what `.github/workflows/ci.yml` runs, as three separate jobs.

### Windows test run

`make wintest` (scripts/wintest.sh) cross-compiles each package's tests for windows/amd64
into `bin/wintest/` and runs them on the Windows host through WSL interop, so `*_windows.go`
paths and Windows file-system behaviour are actually exercised. Outside WSL it prints
`wintest: skipped (not running under WSL)` and exits 0. It prints `ok <pkg> <secs>` or
`FAIL <pkg>` with the tail of the package's output (the full log stays in
`bin/wintest/<pkg>.log`), then `wintest: N packages, P ok, F failed, S skipped`; any
failure makes it exit non-zero.

Each binary runs from its package directory, as `go test` does. By default that is a copy
of the tree (tracked and untracked-unignored files) under `%TEMP%\niagara-go-wintest\`,
removed on exit: from the `\\wsl.localhost\...` path itself, `t.Chdir` cannot restore the
working directory and panics, aborting the package. Committed symlinks cannot be created
on the Windows disk; the script lists them before the run.

```sh
WINTEST_PKGS="./internal/pycompat ./internal/secaudit" make wintest  # restrict packages
WINTEST_RUN='^TestWinError' make wintest  # -test.run regex
WINTEST_SHORT=0 make wintest              # full suites (default passes -test.short)
WINTEST_TIMEOUT=20m make wintest          # per-package -test.timeout (default 10m)
WINTEST_MIRROR=0 make wintest             # run in place from the UNC path
PYCOMPAT_WINPY=/mnt/c/.../python.exe make wintest  # or WINTEST_PYTHON
```

The Windows CPython that the Python-comparison tests run is auto-detected under
`/mnt/c/Users/*/AppData/Local/Programs/Python/Python3*/python.exe` and passed as
`PYCOMPAT_WINPY` through `WSLENV`; without one the script says so and those tests skip.

## 6. Commits and review

Commit by work unit with a Conventional Commit message, tests and fixtures with the code they
prove. Each unit is reviewed with the receipt-driven review driver
([rdd-review-driver.md](rdd-review-driver.md)); keep a unit at about 700 changed lines or
less, fixtures included. A unit near 1,200 lines fails review START with
`lens_context_budget_exceeded`, and a golden set with hundreds of files can fail well below
that. When fixtures and code are split into two commits, review them as one slice.

## 7. Real-corpus parity (`make pyparity`)

Builder ports (`internal/helpbuild`) compare Python and Go on real data: the EC-Net
4.3.58.18 and Niagara 4.15.6.20 installs under `/mnt/c`, their niagara-help `versions/`
trees, the real PDFs and the developer course. These tests read gigabytes, so `go test`,
`make test` and `make ci` skip them, each skip naming the variable that turns it on.
`make pyparity` (and so `make parity`) sets every one (`HELPBUILD_PARITY_REAL=1`, `HELPBUILD_PARITY_FULL=1`,
`HELPBUILD_PARITY_415=1`) and runs the package's `Parity` tests, synthetic ones included;
it takes several minutes (see [helpbuild.md](helpbuild.md#real-data-parity) for the list
and the timing). Run it before handing any builder PR for review, and after changing a
helper the builders share. A real install that is not mounted skips visibly; it never
passes without comparing. The Python reference is not optional under `make pyparity`: a
missing `python3` or reference worktree fails every parity test with a message, where
plain `go test` skips naming the variable. After Phase 5 the Python reference is the
niagara-help `pre-go-retire` worktree named by `NIAGARA_HELP_PY_REF` (`internal/pyref`, D47;
[goldens.md](goldens.md#freezing-and-regenerating)); `NIAGARA_HELP_DIR` stays the live data.
