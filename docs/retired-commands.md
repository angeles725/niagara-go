# Retired commands

The Go port carries only the commands that have a recorded use. Decision D32 (D-I2-7,
accepted by the integrator; `odd/tasks/go-migration.md` in niagara-research) retired the rest
after a usage audit of 2026-09-18..10-17: 29 session transcripts, 1,403 research blocks, 143
retros, both hook cards and the hooks, the kit (917 files), the tools compatibility contract,
the READMEs, and indirect calls through searchdoor. A retired command has no recorded use in
any of them; the nh MCP server (which exposed some of them) is configured nowhere and is
retired too (D32 supersedes D15).

A retired command still parses: it prints one line on stderr and exits 2.

```
nh: 'hierarchy' was retired in the Go port (no recorded use); see docs/retired-commands.md
```

## Restore path

Before the Phase 5 deletion of the Python tools, the integrator tags niagara-research,
niagara-help and module-navigator with `pre-go-retire` (the tag is pending until then). To use
a retired command, check out that tag and run the Python tool
(`python3 tools/niagara_help.py <command> ...` in niagara-help, `python3 tools/module_nav.py
<command> ...` in module-navigator); to bring it back into Go, port it from that tag with the
golden cases of `tests/golden_cli.py`. A command that once had a Go port can also be
recovered from this repository's history (the commit that retired it names the files).

## nh (niagara-help)

Kept: class (with --jdk), search, package, method, slots (all modes), guide, guide-search,
devguide, devguide-search, pdf, find, source (with --code, --grep), source-grep, stats,
freshness (with --all), selftest (with --recall), versions. source --grep and source-grep wait
for the shared Python-regex package (internal/pyre) to reach this branch.

Paths are under `tools/niagara_help_lib/` of the niagara-help repository.

| Command | Python module path | Last Python commit | Notes |
|---|---|---|---|
| `hierarchy` | `cli/cmd_classes.py`, `hierarchy.py`, `interface_hierarchy_search.py` | `pre-go-retire` (tag pending) | Go port (S2) removed; its key resolver stays for selftest --recall |
| `implementors` | `cli/cmd_classes.py`, `hierarchy.py` | `pre-go-retire` (tag pending) | Go port (S2) removed; implementor_matches stays for selftest --recall |
| `interface-hierarchy` | `cli/cmd_classes.py`, `interface_hierarchy_search.py` | `pre-go-retire` (tag pending) | alias of `hierarchy --interface`; Go port (S2) removed |
| `archetypes` | `cli/cmd_classes.py`, `interface_hierarchy_search.py` | `pre-go-retire` (tag pending) | Go port (S2) removed |
| `orphans` | `cli/cmd_classes.py`, `interface_hierarchy_search.py` | `pre-go-retire` (tag pending) | Go port (S2) removed |
| `enum` | `cli/cmd_classes.py`, `enum_search.py` | `pre-go-retire` (tag pending) | never ported; resolve_enum stays for selftest --recall |
| `xref` | `cli/cmd_analysis.py`, `xref_search.py` | `pre-go-retire` (tag pending) | Go port (S2) removed |
| `method-sig` | `cli/cmd_members.py`, `method_search.py` | `pre-go-retire` (tag pending) | never ported |
| `inherited-slots` | `cli/cmd_members.py`, `slots_search.py` | `pre-go-retire` (tag pending) | alias of `slots --inherited`, which is kept; Go alias (S3) removed |
| `inherited-methods` | `cli/cmd_members.py`, `method_search.py` | `pre-go-retire` (tag pending) | Go port (S3) removed |
| `api` | `cli/cmd_members.py`, `api_search.py` | `pre-go-retire` (tag pending) | Go port (S3) removed |
| `overrides` | `cli/cmd_members.py`, `overrides_search.py` | `pre-go-retire` (tag pending) | Go port (S3) removed |
| `contract` | `cli/cmd_members.py`, `contract_search.py` | `pre-go-retire` (tag pending) | Go port (S3) removed |
| `compatible` | `cli/cmd_members.py`, `compatible_search.py` | `pre-go-retire` (tag pending) | Go port (S3) removed |
| `module` | `cli/cmd_analysis.py`, `module_search.py` | `pre-go-retire` (tag pending) | Go port (S4) removed; module_class_matches stays for selftest --recall |
| `related` | `cli/cmd_analysis.py`, `related_search.py` | `pre-go-retire` (tag pending) | Go port (S4) removed |
| `module-deps` | `cli/cmd_analysis.py`, `module_deps.py` | `pre-go-retire` (tag pending) | Go port (S4) removed |
| `compare` | `cli/cmd_analysis.py`, `compare_search.py` | `pre-go-retire` (tag pending) | never ported |
| `profile` | `cli/cmd_analysis.py`, `profile_search.py` | `pre-go-retire` (tag pending) | Go port (S4) removed |
| `graph` | `cli/cmd_analysis.py`, `graph_search.py` | `pre-go-retire` (tag pending) | Go port (S4) removed |
| `hotspots` | `cli/cmd_analysis.py`, `hotspots_search.py` | `pre-go-retire` (tag pending) | Go port (S4) removed |
| `module-summary` | `cli/cmd_analysis.py`, `module_summary.py` | `pre-go-retire` (tag pending) | Go port (S4) removed (with its CPython set-order emulation, pyset.go) |
| `fuzzy` | `cli/cmd_docs.py`, `fuzzy_search.py` | `pre-go-retire` (tag pending) | never ported as a command; its search stays inside selftest --recall (sampled row) |
| `snippet` | `cli/cmd_source.py`, `source_search.py` | `pre-go-retire` (tag pending) | never ported |
| `callers` | `cli/cmd_source.py`, `source_search.py` | `pre-go-retire` (tag pending) | never ported (mnav `callers` is a different, kept tool) |
| `chain` | `cli/cmd_source.py`, `chain_search.py` | `pre-go-retire` (tag pending) | never ported |
| `examples` | `cli/cmd_source.py`, `examples_search.py` | `pre-go-retire` (tag pending) | never ported |
| `annotations` | `cli/cmd_source.py`, `annotations_search.py` | `pre-go-retire` (tag pending) | never ported |
| `recipe` | `cli/cmd_develop.py`, `recipe_search.py` | `pre-go-retire` (tag pending) | never ported |
| `checklist` | `cli/cmd_develop.py`, `recipe_search.py` | `pre-go-retire` (tag pending) | never ported |
| `topics` | `cli/cmd_develop.py`, `topics_search.py` | `pre-go-retire` (tag pending) | never ported |
| `action-params` | `cli/cmd_develop.py`, `topics_search.py` | `pre-go-retire` (tag pending) | never ported |
| `event-flow` | `cli/cmd_develop.py`, `topics_search.py` | `pre-go-retire` (tag pending) | never ported |
| `export` | `cli/cmd_export.py`, `export_search.py`, `html_export.py`, `module_export.py` | `pre-go-retire` (tag pending) | never ported |
| `export-html` | `cli/cmd_export.py`, `html_export.py` | `pre-go-retire` (tag pending) | alias of `export --html`; never ported |
| `export-module` | `cli/cmd_export.py`, `module_export.py` | `pre-go-retire` (tag pending) | alias of `export --module`; never ported |
| `coverage` | `cli/cmd_export.py`, `coverage_report.py`, `doc_coverage.py`, `doc_jars.py`, `install_sweep.py` | `pre-go-retire` (tag pending) | never ported; the --doc-jars / --sweep gates stay in Python with the Phase 4 builders |
| `diff-api` | `cli/cmd_export.py`, `api_diff.py` | `pre-go-retire` (tag pending) | never ported |
| `build-call-index` | `cli/cmd_admin.py`, `../build_call_index.py` | `pre-go-retire` (tag pending) | index builder: stays in Python until the Phase 4 builders |
| `rebuild-all` | `cli/cmd_admin.py`, `maintenance.py` | `pre-go-retire` (tag pending) | index maintenance: stays in Python until the Phase 4 builders (freshness is kept) |
| `repl` | `cli/cmd_admin.py`, `cli/repl.py` | `pre-go-retire` (tag pending) | never ported |

## mnav (module-navigator)

To be filled by the mnav retirement slice.
