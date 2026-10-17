# niagara-go

Go implementation of the maintained Niagara N4 research tools (niagara-research `tools/`,
module-navigator and niagara-help). One module, static binaries for Linux and Windows,
no cgo.

## Layout

| Path | Contents |
|---|---|
| `cmd/<tool>/main.go` | Thin entry point: `cli.Run(<tool>.Main)`. One directory per installed command. |
| `internal/<tool>/` | The tool's logic, split by command group; exports `Main(args, cli.Stdio) int`. |
| `internal/cli` | Exit codes (0 found, 1 searched-no-match, 2 cannot search) and injectable stdio. |
| `internal/sizegate` | Test that keeps every `.go` file at 500 lines or less (allowlist may only shrink). |
| `scripts/git-hooks/` | Pre-commit gate: gofmt, `go vet`, `go test -short`. |

Shared libraries (bog reader, text folding, BM25, ES->EN glossary, index readers, golden
runner) live in their own `internal/` packages; a tool never imports another tool's package.

## Working

```sh
make hooks        # once per clone/worktree: enable the pre-commit gate
make check        # gofmt + vet + tests
make build        # static binaries in bin/
make cross        # bin/linux-amd64/ and bin/windows-amd64/
```

Behaviour parity with the Python tools is proven by the golden CLI suites of each source
repo, run against the Go binary. Intended differences are listed in the commit message.
Rules: `docs/TOOLING-GUIDELINES.md` and `docs/tools-compatibility-contract.md` in
niagara-research.
