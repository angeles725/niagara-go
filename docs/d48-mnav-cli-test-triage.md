# D48: Go mnav vs the module-navigator Python CLI tests

module-navigator deletes its Python CLI tests (branch `i2/lib-retire`). Before that, every
behaviour those tests pin was checked against the Go `mnav`, so that no difference is dropped
without a reason. Rule D48: a difference in a command outside the D32 keep set is dropped (the
command is retired, `docs/retired-commands.md`); a difference in a kept command is fixed in Go
with a golden that pins the Python output, or is marked intended with a reason.

**Result: every difference is in a retired command (or is already marked intended). No kept
command has a gap, so this change has no Go code fix.**

## How it was run

- Tests: a scratch copy of module-navigator at tag `pre-go-retire` (`f897268`, where
  `tools/module_nav.py` is still the full Python CLI, not a shim), with `indexes/` linked to the
  real indexes and a sibling `organized/` linked to the real corpus. (`NAV_CORPUS_BASE` cannot be
  used: `test_corpus_location` deletes it from `os.environ` and later modules lose it.)
- Modules: the 17 `tests/test_*.py` that run the CLI (`_harness.run_nav` / `cli_command`), with
  `MNAV_SKIP_GOLDEN=1`; then `tests/golden_cli.py` and `test_deterministic_order` separately.
- Go: `GOLDEN_BIN=<go build ./cmd/mnav from this branch>` (the harness's own port switch).
- Python: no `GOLDEN_BIN`, `MODULE_NAV_PYTHON=1`, `NIAGARA_GO_BIN_DIR=/nonexistent`, and no
  niagara-go directory on `PATH`, so no Go binary can answer.

| Run | Python (tag) | Go mnav |
|---|---|---|
| 17 CLI test modules (171 tests) | OK (2 skipped) | 71 failures (2 skipped) |
| `golden_cli.py` (314 cases) | 9 differ: index drift since the capture (see below) | 252 differ |
| `test_deterministic_order` | not re-run | OK |

The 2 skips are the same on both sides: `test_class_rank.GlossaryCopy.test_equals_the_canonical_glossary`
(needs module-navigator inside a niagara-research checkout; a Python library test, not the CLI;
Go reads the one shared `internal/glossary`) and `test_deterministic_order` (skipped by
`MNAV_SKIP_GOLDEN`, run separately).

## Triage: the 71 failing unit tests

Every Go result below is the D32 line `mnav: '<command>' was retired in the Go port (no recorded
use); see docs/retired-commands.md` with exit 2. In the methods that loop over several commands,
the kept commands' subtests pass on Go; only the retired ones fail.

| Test (module.Class.method) | Failing subtests: command | Keep set? | Python behaviour | Class |
|---|---|---|---|---|
| test_honest_misses.Misses.test_class_misses_exit_1_with_suggestions | methods, lifecycle, thread-safety, complexity | no | class miss exits 1 with suggestions | retired-drop |
| test_indexes.SharedLoader.test_a_missing_detail_index_is_not_a_silent_zero | unified, feature-brief | no | missing method index is reported, not 0 hits | retired-drop |
| test_indexes.SharedLoader.test_commands_exit_2_without_their_index | methods, annotations, bql-tables, ui | no | exit 2 without the index | retired-drop |
| test_indexes.SharedLoader.test_former_private_loaders_exit_2_without_their_index | 23: service-order, dependency-audit, integration-contract, feature-brief, slot-validate, resolve-audit, ord-validate, resource-leak, driver-cleanup-audit, virtual-callers, unified, compare, slot-collision, profile, strings, deps-graph, patch-plan, export, example-mine, full-stack-trace, full-profile, cross-ref, explain | no | exit 2 without the index | retired-drop |
| test_indexes.SharedLoader.test_servlet_runs_without_the_string_index | servlet | no | runs without string-index | retired-drop |
| test_json_contract.JsonContract.test_json_stdout_is_pure_json | 13: integration-contract, example-mine, virtual-callers, feature-brief, slot-collision, ord-validate, resolve-audit, driver-cleanup-audit, resource-leak, service-order, dependency-audit, module-health, full-stack-trace | no | `--json` stdout is pure JSON | retired-drop |
| test_json_contract.JsonContract.test_slot_validate_json_contract | slot-validate | no | JSON contract | retired-drop |
| test_lookup_fallbacks.BogIndexLookup.test_missing_bog_index_names_every_path_tried | bog-classes | no | names every bog-index path tried | retired-drop |
| test_lookup_fallbacks.ClassLookup.test_truncated_index_is_an_error_not_a_miss | xref, fields, methods | no | truncated index exits 2, not a miss | retired-drop |
| test_lookup_fallbacks.FieldLookup.test_property_fields_are_listed, test_miss_reports_scope | fields, field | no | lists Property fields; miss with scope | retired-drop |
| test_lookup_fallbacks.XrefLookup (6 methods, 9 cases: external importers, case-insensitive, qualified name, qualified inner class x4, imports of external class, miss scope) | xref | no | `--importers`/`--imports` lookups | retired-drop |
| test_repl.ReplSession.test_session_exits_0_whatever_the_last_command_did | repl (quit, EOF) | no | session exits 0 | retired-drop |
| test_search_coverage.StringSearchFolding (accent/case x2, miss scope) | string-search | no | accent- and case-insensitive match | retired-drop |
| test_search_coverage.StringSearchFoldingOnFixture.test_unaccented_query_finds_accented_literal | string-search (x2) | no | accent-insensitive on a fixture | retired-drop |
| test_smoke.CoreSmoke.test_methods | methods | no | lists methods | retired-drop |

## Triage: the golden CLI suite (`golden_cli.py`)

| Cases | Command | Keep set? | Go behaviour | Class | Reason |
|---|---|---|---|---|---|
| 135 command cases | 113 retired commands | no | retired line, exit 2 | retired-drop | D32 |
| 113 `help-<cmd>` cases | the same retired commands | no | case not generated (Go `--help` does not list them) | retired-drop | D32 |
| `help` | top-level `--help` | yes | lists only the kept commands | intended | D32; pinned by `testdata/golden-real/help/intended.json` |
| `help-palette-lexicon-agents` | palette-lexicon-agents | yes | the niagara-research script's help | intended | D39 e; `golden-real/help-palette-lexicon-agents/intended.json` |
| `palette-lexicon-agents` (`alarm --json`) | palette-lexicon-agents | yes | exit 1, JSON error naming the searched base dir | intended | D39 e: follows niagara-research `tools/palette-lexicon-agents.py`; pinned by `golden-real/palette-lexicon-agents` |
| `stats` | stats | yes | same bytes as Python today | not a Go difference | the tag Python also differs from `cli.json` (133 -> 129 lines): the indexes changed after the capture; Go and Python outputs are identical (`--keep` diff empty) |

The tag Python's own 9 differences from `cli.json` (stats, compare, cross-ref, feature-brief,
full-profile, patch-plan, resources-usage, unified, version-diff) come from the indexes rebuilt
after the capture; all but `stats` are retired commands.

## Not in scope

Test modules that do not run the CLI (builder, parser, size-gate, git-hook, registry tests) test
Python internals; the Go builders have their own parity (D33, `docs/navbuild.md`).
