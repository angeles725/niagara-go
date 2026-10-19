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
| `internal/sizegate` | Test that keeps every `.go` file outside `testdata/` and every `.py` script at 500 lines or less (allowlist may only shrink). |
| `scripts/git-hooks/` | Pre-commit gate: gofmt, `go vet`, `go test -short`. |

Shared libraries live in their own `internal/` packages; a tool never imports another
tool's package (D2, refined by D29 and D42 in niagara-research `odd/tasks/go-migration.md`). Ask the
owner before adding to a shared package.

| Package | Contents | Owner |
|---|---|---|
| `cli` | exit codes, `Stdio`, `Run` | Integrante1 |
| `argparse` | Python 3.15 argparse parity ([docs/argparse.md](docs/argparse.md)) | Integrante1 |
| `golden` | golden CLI runner ([docs/goldens.md](docs/goldens.md)) | Integrante1 |
| `bog` | BOG reader and component graph | Integrante1 |
| `textfold` | Python text normalization (folding, terms) | Integrante1 |
| `bm25` | BM25 scoring presets | Integrante1 |
| `glossary` | Spanish -> English query glossary | Integrante1 |
| `honest` | the "honest zero" no-match lines | Integrante1 |
| `version` | the `--version` line | Integrante1 |
| `pycompat` | Python 3.15 behaviour the tools reproduce | Integrante1 |
| `difflib`, `fnmatch` | Python stdlib ports | Integrante1 |
| `slotomatic` | Slotomatic slot declarations of a Java source | Integrante1 |
| `sizegate` | the 500-line source limit (test only) | Integrante1 |
| `index` | niagara-help corpus metadata readers | Integrante2 |
| `pyre` | Python `re` patterns for user input | Integrante2 |
| `blockfiles` | block discovery of the research corpus | Integrante2 |
| `pyjson` | JSON codec with Python `json` semantics | Integrante3 |
| `pycsv` | Python `csv.writer` (excel dialect), byte for byte | Integrante1 |
| `pyxml` | expat-compatible XML parser | Integrante1 |
| `locate` | finds the checkout a tool works on (D44) | Integrante1 |
| `pyref` | locates the frozen `pre-go-retire` Python references (D47; test only) | Integrante2 |
| `atomicfile` | atomic file and directory-tree writes | Integrante3 |
| `procgroup` | child processes whose stop stops everything they started (D36) | Integrante3 |
| `pyhtml` | Python `html` module family (D37) | Integrante3 |
| `bajadocxml` | Niagara `.bajadoc` XML reader | Integrante3 |
| `niagaraannot` | Slot-o-Matic annotations of a Niagara Java source | Integrante3 |

## Working

```sh
make hooks        # once per clone/worktree: enable the pre-commit gate
make check        # gofmt + vet + tests
make lint         # staticcheck
make cover        # coverage across ./internal/... and the total
make deadcode     # report-only list of unreachable functions (test-only packages filtered)
make realdata     # real-data gates on the niagara-research checkout (not in ci)
make pyparity     # Go vs the frozen pre-go-retire Python reference (D47; not in ci)
make parity       # realdata + pyparity
make ci           # check + lint + cross (what CI runs)
make build        # static binaries in bin/
make cross        # bin/linux-amd64/ and bin/windows-amd64/
```

New tool: [docs/adding-a-tool.md](docs/adding-a-tool.md).

Behaviour parity with the retired Python tools is proven by golden cases frozen from Python and
run against the Go code (and, with `GOLDEN_BIN`, against the built binary). Behaviour added after
a tool froze has Go-only goldens (D34). Each intended difference is declared next to its case in
`testdata/golden/<case>/intended.json`, with a mandatory reason ([docs/goldens.md](docs/goldens.md)).
Rules: `docs/TOOLING-GUIDELINES.md` and `docs/tools-compatibility-contract.md` in
niagara-research.
