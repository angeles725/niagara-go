# nh golden testdata

`testdata/` is a minimal niagara-help checkout: `versions/registry.json` (the real
registry) and, for version 4.14.0.162 whose `data_root` is `.`, a small fixture
corpus under `indexes/`. The real indexes are megabytes; full parity is proven by
the repository golden suite (`niagara-help/tests/golden_cli.py` with `GOLDEN_BIN`).

Every case under `golden/` holds what the Python CLI printed, captured with:

| Setting | Value |
|---|---|
| Python | 3.15.0 |
| Tool | `tools/niagara_help.py` of the niagara-help repository at a9a15642e |
| Environment | `COLUMNS=100 NO_COLOR=1 PYTHONHASHSEED=0`, no other `NIAGARA_HELP_*` |
| Checkout | `NIAGARA_HELP_DIR=<this testdata directory>`, run with it as the cwd |
| Version | `--niagara-version 4.14.0.162` first on the command line |

The Go runner (`internal/golden`) applies `COLUMNS=100` and `NO_COLOR=1` to every
case; `PYTHONHASHSEED=0` only fixes Python set order and has no Go counterpart.
A lookup that misses prints the absolute data root, so miss cases are unit tests
rather than goldens.
