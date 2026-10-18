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
freshness (with --all), selftest (with --recall), versions.

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

Kept: class, search, inventory, module, modules, rank, ask, hierarchy, implementors, drivers,
grep, source, token, license-feature, license-features, resources, callers, callees, stats,
palette-lexicon-agents, selftest (with --recall).

Python: module-navigator `tools/module_nav.py`. Module paths are relative to `tools/`; "registered in" is the `commands/` file that declares the subcommand.

| Command | Python module | Registered in | Last Python commit | Notes |
|---|---|---|---|---|
| `package` | `module_nav_lib/class_search.py` | `commands/lookup.py` | `pre-go-retire` (tag pending) | was ported (S2); Go code removed |
| `snippet` | `module_nav_lib/grep_search.py` | `commands/source.py` | `pre-go-retire` (tag pending) | S4 slice, never ported |
| `imports` | `module_nav_lib/header_imports.py` | `commands/source.py` | `pre-go-retire` (tag pending) | S4 slice, never ported |
| `ui` | `module_nav_lib/swing_search.py` | `commands/ui.py` | `pre-go-retire` (tag pending) |  |
| `xref` | `module_nav_lib/xref.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `deps` | `module_nav_lib/xref.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `method` | `module_nav_lib/methods.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `methods` | `module_nav_lib/methods.py` | `commands/structure.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `slots` | `module_nav_lib/annotations.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `annotations` | `module_nav_lib/annotations.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `profile` | `module_nav_lib/profile.py` | `commands/lookup.py` | `pre-go-retire` (tag pending) |  |
| `repl` | `module_nav_lib/repl/` | `commands/session.py` | `pre-go-retire` (tag pending) |  |
| `full-profile` | `module_nav_lib/help_bridge/reports.py` | `commands/lookup.py` | `pre-go-retire` (tag pending) |  |
| `cross-ref` | `module_nav_lib/help_bridge/reports.py` | `commands/lookup.py` | `pre-go-retire` (tag pending) |  |
| `patch-target` | `module_nav_lib/patch/target.py` | `commands/patching.py` | `pre-go-retire` (tag pending) |  |
| `patch-plan` | `module_nav_lib/patch/plan.py` | `commands/patching.py` | `pre-go-retire` (tag pending) |  |
| `extract` | `module_nav_lib/patch/plan.py` | `commands/patching.py` | `pre-go-retire` (tag pending) |  |
| `orphans` | `module_nav_lib/analysis.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `deps-graph` | `module_nav_lib/analysis.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `api-surface` | `module_nav_lib/analysis.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `security-audit` | `module_nav_lib/analysis.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `strings` | `module_nav_lib/extras.py` | `commands/source.py` | `pre-go-retire` (tag pending) |  |
| `trace-type` | `module_nav_lib/extras.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `version-diff` | `module_nav_lib/extras.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `call-chain` | `module_nav_lib/callgraph.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `hotspots` | `module_nav_lib/callgraph.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `fields` | `module_nav_lib/fields.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `field` | `module_nav_lib/fields.py` | `commands/structure.py` | `pre-go-retire` (tag pending) |  |
| `type-consumers` | `module_nav_lib/typeflow.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `type-producers` | `module_nav_lib/typeflow.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `type-flow` | `module_nav_lib/typeflow.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `module-calls` | `module_nav_lib/coupling.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `module-api-usage` | `module_nav_lib/coupling.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `coupling` | `module_nav_lib/coupling.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `patterns` | `module_nav_lib/patterns.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `pattern` | `module_nav_lib/patterns.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `bog-trace` | `module_nav_lib/bog_bridge.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `bog-classes` | `module_nav_lib/bog_bridge.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `bog-coverage` | `module_nav_lib/bog_bridge.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `throws` | `module_nav_lib/exceptions.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `catches` | `module_nav_lib/exceptions.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `export` | `module_nav_lib/export/` | `commands/patching.py` | `pre-go-retire` (tag pending) |  |
| `impact` | `module_nav_lib/impact.py` | `commands/flow.py` | `pre-go-retire` (tag pending) |  |
| `find` | `module_nav_lib/fuzzy.py` | `commands/lookup.py` | `pre-go-retire` (tag pending) |  |
| `deprecated` | `module_nav_lib/deprecated.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `deprecated-users` | `module_nav_lib/deprecated.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `deprecated-risk` | `module_nav_lib/deprecated.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `similar` | `module_nav_lib/similarity.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `clones` | `module_nav_lib/similarity.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `config-usage` | `module_nav_lib/config_coupling.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `config-keys` | `module_nav_lib/config_coupling.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `resources-usage` | `module_nav_lib/config_coupling.py` | `commands/source.py` | `pre-go-retire` (tag pending) |  |
| `serial` | `module_nav_lib/serialization.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `serial-audit` | `module_nav_lib/serialization.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `serial-conflicts` | `module_nav_lib/serialization.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `thread-safety` | `module_nav_lib/threading.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `thread-scan` | `module_nav_lib/threading.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `lifecycle` | `module_nav_lib/lifecycle.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `lifecycle-scan` | `module_nav_lib/lifecycle.py` | `commands/quality.py` | `pre-go-retire` (tag pending) |  |
| `string-search` | `module_nav_lib/string_constants.py` | `commands/source.py` | `pre-go-retire` (tag pending) |  |
| `string-constants` | `module_nav_lib/string_constants.py` | `commands/source.py` | `pre-go-retire` (tag pending) |  |
| `servlets` | `module_nav_lib/servlets.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `routes` | `module_nav_lib/servlets.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `servlet` | `module_nav_lib/servlets.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `ords` | `module_nav_lib/ords.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `ord-usage` | `module_nav_lib/ords.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `ord-flow` | `module_nav_lib/ords.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `topics` | `module_nav_lib/topics/listing.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `subscribers` | `module_nav_lib/topics/subscribers.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `pub-sub` | `module_nav_lib/topics/listing.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `bql` | `module_nav_lib/bql.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `bql-tables` | `module_nav_lib/bql.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `bql-class` | `module_nav_lib/bql.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `permissions` | `module_nav_lib/permissions/` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `license-inspect` | `module_nav_lib/license_inspect.py` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `policy-inspect` | `module_nav_lib/policy_inspect.py` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `trust-anchor-check` | `module_nav_lib/trust_anchor.py` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `bypass-status` | `module_nav_lib/bypass_status.py` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `permission-report` | `module_nav_lib/permission_report.py` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `explain` | `module_nav_lib/explain.py` | `commands/lookup.py` | `pre-go-retire` (tag pending) |  |
| `feasibility-check` | `module_nav_lib/feasibility.py` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `permission-flow` | `module_nav_lib/permissions/flow.py` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `credentials` | `module_nav_lib/permissions/credentials.py` | `commands/security.py` | `pre-go-retire` (tag pending) |  |
| `alarm-flow` | `module_nav_lib/alarm_domain/flow.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `alarm-types` | `module_nav_lib/alarm_domain/flow.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `alarm-trace` | `module_nav_lib/alarm_domain/trace.py` | `commands/domain.py` | `pre-go-retire` (tag pending) |  |
| `cycles` | `module_nav_lib/architecture.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `god-classes` | `module_nav_lib/architecture.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `layer-check` | `module_nav_lib/architecture.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `complexity` | `module_nav_lib/complexity.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `complexity-scan` | `module_nav_lib/complexity.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `metrics` | `module_nav_lib/complexity.py` | `commands/architecture.py` | `pre-go-retire` (tag pending) |  |
| `bookmark` | `module_nav_lib/bookmarks.py` | `commands/session.py` | `pre-go-retire` (tag pending) |  |
| `bookmarks` | `module_nav_lib/bookmarks.py` | `commands/session.py` | `pre-go-retire` (tag pending) |  |
| `unbookmark` | `module_nav_lib/bookmarks.py` | `commands/session.py` | `pre-go-retire` (tag pending) |  |
| `history` | `module_nav_lib/bookmarks.py` | `commands/session.py` | `pre-go-retire` (tag pending) |  |
| `unified` | `module_nav_lib/unified/search.py` | `commands/lookup.py` | `pre-go-retire` (tag pending) |  |
| `compare` | `module_nav_lib/unified/compare.py` | `commands/lookup.py` | `pre-go-retire` (tag pending) |  |
| `integration-contract` | `module_nav_lib/contracts/report.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `example-mine` | `module_nav_lib/mining.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `virtual-callers` | `module_nav_lib/virtual_dispatch.py` | `commands/flow.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `feature-brief` | `module_nav_lib/research/brief.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `slot-validate` | `module_nav_lib/slot_validate.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `slot-collision` | `module_nav_lib/slot_collision.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `ord-validate` | `module_nav_lib/ord_validate.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `resolve-audit` | `module_nav_lib/resolve_audit.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `driver-cleanup-audit` | `module_nav_lib/driver_cleanup.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `resource-leak` | `module_nav_lib/resource_leak.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `service-order` | `module_nav_lib/service_order.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `dependency-audit` | `module_nav_lib/dependency_audit.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `module-health` | `module_nav_lib/module_health/report.py` | `commands/audits.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
| `session` | `module_nav_lib/sessions.py` | `commands/session.py` | `pre-go-retire` (tag pending) |  |
| `full-stack-trace` | `module_nav_lib/fullstack.py` | `commands/flow.py` | `pre-go-retire` (tag pending) | only caller: module-navigator's own tests (contract consumer table) |
