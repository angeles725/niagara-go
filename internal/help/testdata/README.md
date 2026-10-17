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

`s3adv/` (no method-index.json; cyclic and self-extending chains, an untyped property, a
class named `B`) and `s3adv-enum/` (an enum for `contract`) are two more checkouts for the
S3 review advisories, captured the same way with their own `NIAGARA_HELP_DIR`.

## Full-text fixture

`fulltext/` is another checkout for the full-text commands (find, guide-search,
devguide-search, pdf): version 1.0 at `.` with every text source, and 2.0 under
`versions/2.0` with guides only, so it borrows 1.0's PDF manuals (`docs-text/`). Its files are
hand-written to reach the edge cases: paged PDF text, change logs, code, skipped example files
(`ext/`, `*.min.js`), CRLF text, navigation and table-of-contents lines, a line long enough to clip.

`fulltext_gen.py` (`python3 -I internal/help/testdata/fulltext_gen.py`) copies `fulltext/` to a
temporary folder, sets every file's mtime to `FIXED_NS` and runs the Python niagara-help there:

| Output | What Python wrote |
|---|---|
| `fulltext-db/<version>.sqlite` | the index `fulltext_index.build` writes, with the temporary folder in the stored fingerprint replaced by `<ROOT>` |
| `fulltext-search.json` | `fulltext.search` results (scores as `float.hex`), each hit's snippet, and `fulltext.clip` cases |

The Go tests (`fts/ftsfixture`) copy `fulltext/` to a temporary folder with the same mtimes and
install the databases with `<ROOT>` replaced by that folder; the fingerprint test then proves
that Go computes exactly the fingerprint Python stored. Regenerate everything after changing
`fulltext/`.
