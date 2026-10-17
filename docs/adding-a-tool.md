# Adding a tool

How a Python tool becomes a Go command here. The rules are decisions D2-D4, D7, D18 and D29
in niagara-research `odd/tasks/go-migration.md`.

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
`cli.ExitNoMatch` (1) or `cli.ExitCannotSearch` (2), or whatever code the Python tool uses.
Write only to `s.Out` and `s.Err`, so tests run `Main` in process.

A tool never imports another tool's package. The one exception (D29) is a library subpackage
`internal/<tool>/<lib>` that does not print, returns typed rows, does not depend on the tool's
env or registry, and exports the formatter for its CLI line. Shared code goes in a shared
package (see the README table); ask its owner before adding to it.

## 2. Command line

Build the parser with `internal/argparse` (`argparse.New`, `p.Add`, `p.Parse`) so help, usage,
errors and exit codes match Python byte for byte; see [argparse.md](argparse.md) for the
supported subset. `--version` prints `version.Line("<tool>")` (D18): the Go build's own VCS
stamp, not the Python checkout's.

## 3. Goldens

1. Write a generator under `internal/<pkg>/testdata/` that runs the Python tool and writes
   `golden/<case>/{cmd.json,stdout,stderr,exit}`; run it with `python3 -I`.
2. Wire them: `func TestGolden(t *testing.T) { golden.Run(t, "testdata", Main) }`.
3. Every deliberate divergence from Python gets an `intended.json` with a mandatory `reason`;
   never edit `stdout`/`stderr`/`exit` by hand.
4. After `make build`, run the same cases against the binary:
   `GOLDEN_BIN=$PWD/bin/<tool> go test ./internal/<pkg> -run TestGolden`.

Details: [goldens.md](goldens.md).

## 4. Size gate

`internal/sizegate` fails `go test` when a `.go` file outside `testdata/` exceeds 500 lines
(D4). `internal/sizegate/allowlist.txt` lists the exceptions as `<path> <max-lines>`; entries
may only be lowered or removed, and the test fails on an entry that is too high or matches no
file. Split the file instead of adding an entry.

## 5. Checks

```sh
make hooks     # once per clone/worktree: pre-commit runs gofmt, go vet, go test -short
make check     # gofmt + vet + full tests
make lint      # staticcheck (pinned in the Makefile)
make cover     # coverage across ./internal/... and the total
make deadcode  # report-only list of functions no command reaches
make cross     # linux-amd64 and windows-amd64 binaries
```

`make ci` (check, lint, cross) is what `.github/workflows/ci.yml` runs.

## 6. Commits and review

Commit by work unit with a Conventional Commit message, tests and fixtures with the code they
prove. Each unit is reviewed with the receipt-driven review driver
([rdd-review-driver.md](rdd-review-driver.md)); keep a unit at about 700 changed lines or
less, fixtures included. A unit near 1,200 lines fails review START with
`lens_context_budget_exceeded`, and a golden set with hundreds of files can fail well below
that. When fixtures and code are split into two commits, review them as one slice.
