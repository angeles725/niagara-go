# nh golden testdata

`testdata/` is a minimal niagara-help checkout: `versions/registry.json` (the real
registry) and, for version 4.14.0.162 whose `data_root` is `.`, a small fixture
corpus: hand-written `indexes/*.json` and `bajadoc-clean/` texts chosen to reach the
edge cases (name collisions, generics, case-insensitive fallbacks, CRLF text). The real indexes are megabytes; full parity is proven by
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
A lookup that misses prints the absolute data root, so miss outputs live in
`misses/*.txt` with `<TESTDATA>` for that root and are checked by `miss_test.go`.

`compat/` is a second checkout (same registry entry, `data_root` `.`) for the `compatible`
cases, which run with `"cwd_fixture": "compat"` and were captured with
`NIAGARA_HELP_DIR=<testdata>/compat`: its class names reach every branch of the name
heuristics without changing the class-index counts the other cases print.

`s4/` is a third checkout (same registry entry) for the S4 commands (module, module-deps,
module-summary, related, profile, graph, hotspots, stats, guide, devguide): its indexes
add source-index, guides-index, devguide-toc, pdf-index and call-index, and its text
directories (with a dangling link and a link to a directory) feed `stats` and the guides /
devguide miss scopes. Its cases run with `"cwd_fixture": "s4"`, captured with
`NIAGARA_HELP_DIR=<testdata>/s4`; misses and `stats` print the absolute data root, so they
live in `misses/s4-*.txt` and are checked by `s4miss_test.go`. `pyset.json` holds `hash()`
values and a `set()` iteration order printed by Python 3.15 with `PYTHONHASHSEED=0`.
