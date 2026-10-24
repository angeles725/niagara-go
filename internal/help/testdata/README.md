# nh golden testdata

`testdata/` is a minimal niagara-help checkout: `versions/registry.json` (the real
registry) and, for version 4.14.0.162 whose `data_root` is `.`, a small fixture
corpus: hand-written `indexes/*.json` and `bajadoc-clean/` texts chosen to reach the
edge cases (name collisions, generics, case-insensitive fallbacks, CRLF text). The real indexes are megabytes; the
real corpus is checked by `TestGoldenReal` (`golden-real/`).

The generators named below retired with the Python reference (D47): every output here is frozen
and changes only from reviewed Go output (`docs/goldens.md`).

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

Three cases are no longer Python output: `help`, `no-command` and
`parse-optional-positional-extra` print the subcommand list, which no longer names the
commands retired in the Go port (decision D32, `docs/retired-commands.md`); they hold what
nh prints. The cases of retired commands (and their `--help`) were removed with them.

`s4/` is a second checkout (same registry entry) for the S4 commands (of which stats, guide
and devguide remain; module, module-deps, module-summary, related, profile, graph and hotspots
are retired): its indexes
add source-index, guides-index, devguide-toc, pdf-index and call-index, and its text
directories (with a dangling link and a link to a directory) feed `stats` and the guides /
devguide miss scopes. Its cases run with `"cwd_fixture": "s4"`, captured with
`NIAGARA_HELP_DIR=<testdata>/s4`; misses and `stats` print the absolute data root, so they
live in `misses/s4-*.txt` and are checked by `s4miss_test.go`.

`call-index.json` is retired (decision I3-5): the Go port neither builds nor reads it.
`s4/indexes/call-index.json` stays as a stale file on disk, and `misses/s4-stats.txt` and
`misses/s4-stats-json.txt` are no longer Python output: they hold what nh prints (9 indexes,
no `CALL INDEX` block, no `call-index` entry). The real `stats` and `stats-json` cases under
`golden-real/` keep Python's bytes and declare nh's in `intended.json` (reason I3-5).

`s3adv/` (no method-index.json; cyclic and self-extending chains, an untyped property, a
class named `B`) is one more checkout for the S3 review advisories of `slots --inherited`,
captured the same way with its own `NIAGARA_HELP_DIR`.

## Full-text fixture

`fulltext/` is another checkout for the full-text commands (find, guide-search,
devguide-search, pdf): version 1.0 at `.` with every text source, and 2.0 under
`versions/2.0` with guides only, so it borrows 1.0's PDF manuals (`docs-text/`). Its files are
hand-written to reach the edge cases: paged PDF text, change logs, code, skipped example files
(`ext/`, `*.min.js`), CRLF text, navigation and table-of-contents lines, a line long enough to clip.

`fulltext_gen.py` (retired with the Python reference in D47) copied `fulltext/` to a temporary
folder, set every file's mtime to `FIXED_NS` and ran the Python niagara-help there:

| Output | What Python wrote |
|---|---|
| `fulltext-db/<version>.sqlite` | the index `fulltext_index.build` writes, with the temporary folder in the stored fingerprint replaced by `<ROOT>` |
| `fulltext-search.json` | `fulltext.search` results (scores as `float.hex`), each hit's snippet, and `fulltext.clip` cases |
| `fulltext-cli/` | the stdout of each CLI case (`<case>.txt`, `<ROOT>` for the checkout) and `cases.json` with its arguments, exit status and stderr |

The Go tests (`fts/ftsfixture`) copy `fulltext/` to a temporary folder with the same mtimes and
install the databases with `<ROOT>` replaced by that folder; the fingerprint test then proves
that Go computes exactly the fingerprint Python stored. These outputs are frozen: a change to
`fulltext/` needs them regenerated from the Go side and reviewed.

## Full-text rebuild

`rebuild_gen.py` (retired in D47) froze what Python did
when the full-text index must be rebuilt (`fulltext_index.open_index`): each case runs the
Python niagara-help on a fresh copy of `fulltext/` with no index (`missing`), an index whose
stored fingerprint was changed (`stale`), a read-only `indexes/` (`ro-indexes`: Python builds in
the cache directory) or a read-only `indexes/` and cache directory (`ro-everywhere`: Python keeps
the index in memory). `fulltext-rebuild/<case>.txt` is its stdout and `fulltext-rebuild/cases.json`
its arguments, exit status, stderr (`<ROOT>`, `<CACHE>` for `XDG_CACHE_HOME`, `<CACHEDIR>` and
`<CACHEDB>` for the data root's cache folder and database) and where an up-to-date index exists
afterwards (`indexes`, `cache` or `none`).

## s5 fixture

`s5/` is a checkout for the commands ported after S6 (source, method, freshness, class --jdk,
selftest --recall): javadoc HTML pages under `bajadoc/` (navigation, `class-use/`, `doc-files/`
and `index-files/` pages the recall skips, a page declaring another package), `.bajadoc` XML
under `devguide/`, `guides/` and `jdk/` (an unparseable file, a `&#x0;` reference only the
sanitizer gets past), Java sources (CRLF, invalid UTF-8, trailing whitespace, a colliding simple
name, a missing file), guide and devguide pages, a stamped `indexes/.manifest.json` with every
freshness status, and `install/` with doc jars against `versions/4.14.0.162.manifest.json`.
`s5_gen.py` (retired in D47) ran the Python niagara-help on a
fresh copy per case and wrote `s5-cli/<case>.txt` (stdout, `<ROOT>` for the checkout) and
`s5-cli/cases.json` (arguments, exit status, stderr), checked by `s5_test.go`. The `freshness` case
exits 1 where Python exited 0: Go's STALE is exit 1 (FRESH 0, not checkable 2), so a
script can gate on the status.
