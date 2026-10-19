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

The Phase 5 deletion of the Python tools left the tag `pre-go-retire` on each repository:
niagara-help `8410d7451`, module-navigator `f897268`, niagara-research `c846ce1e5`. The
worktrees of that tag that the Go tests once read (D47) are retired with them; the tag remains.
To use a retired command, check the tag out in a worktree of its repository and run the Python
tool there (`NIAGARA_HELP_PYTHON=1 python3 tools/niagara_help.py <command> ...` in
niagara-help, whose tagged script is already the Go shim; `python3 tools/module_nav.py
<command> ...` in module-navigator); to bring it back into Go, port it from that tag with the
golden cases of `tests/golden_cli.py`. A command that once had a Go port can also be
recovered from this repository's history (the commit that retired it names the files).

## nh (niagara-help)

Kept: class (with --jdk), search, package, method, slots (all modes), guide, guide-search,
devguide, devguide-search, pdf, find, source (with --code, --grep), source-grep, stats,
coverage (with --doc-jars, --sweep), freshness (with --all), rebuild-all (with --only,
--dry-run, --stamp-only), selftest (with --recall), versions.

`coverage` and `rebuild-all` were retired here first and then ported (S11: nh runs the
reports, gates and index builders of `internal/helpbuild` in process, `docs/helpbuild.md`).

Paths are under `tools/niagara_help_lib/` of the niagara-help repository.

| Command | Python module path | Last Python commit | Notes |
|---|---|---|---|
| `hierarchy` | `cli/cmd_classes.py`, `hierarchy.py`, `interface_hierarchy_search.py` | `pre-go-retire` (8410d7451) | Go port (S2) removed; its key resolver stays for selftest --recall |
| `implementors` | `cli/cmd_classes.py`, `hierarchy.py` | `pre-go-retire` (8410d7451) | Go port (S2) removed; implementor_matches stays for selftest --recall |
| `interface-hierarchy` | `cli/cmd_classes.py`, `interface_hierarchy_search.py` | `pre-go-retire` (8410d7451) | alias of `hierarchy --interface`; Go port (S2) removed |
| `archetypes` | `cli/cmd_classes.py`, `interface_hierarchy_search.py` | `pre-go-retire` (8410d7451) | Go port (S2) removed |
| `orphans` | `cli/cmd_classes.py`, `interface_hierarchy_search.py` | `pre-go-retire` (8410d7451) | Go port (S2) removed |
| `enum` | `cli/cmd_classes.py`, `enum_search.py` | `pre-go-retire` (8410d7451) | never ported; resolve_enum stays for selftest --recall |
| `xref` | `cli/cmd_analysis.py`, `xref_search.py` | `pre-go-retire` (8410d7451) | Go port (S2) removed |
| `method-sig` | `cli/cmd_members.py`, `method_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `inherited-slots` | `cli/cmd_members.py`, `slots_search.py` | `pre-go-retire` (8410d7451) | alias of `slots --inherited`, which is kept; Go alias (S3) removed |
| `inherited-methods` | `cli/cmd_members.py`, `method_search.py` | `pre-go-retire` (8410d7451) | Go port (S3) removed |
| `api` | `cli/cmd_members.py`, `api_search.py` | `pre-go-retire` (8410d7451) | Go port (S3) removed |
| `overrides` | `cli/cmd_members.py`, `overrides_search.py` | `pre-go-retire` (8410d7451) | Go port (S3) removed |
| `contract` | `cli/cmd_members.py`, `contract_search.py` | `pre-go-retire` (8410d7451) | Go port (S3) removed |
| `compatible` | `cli/cmd_members.py`, `compatible_search.py` | `pre-go-retire` (8410d7451) | Go port (S3) removed |
| `module` | `cli/cmd_analysis.py`, `module_search.py` | `pre-go-retire` (8410d7451) | Go port (S4) removed; module_class_matches stays for selftest --recall |
| `related` | `cli/cmd_analysis.py`, `related_search.py` | `pre-go-retire` (8410d7451) | Go port (S4) removed |
| `module-deps` | `cli/cmd_analysis.py`, `module_deps.py` | `pre-go-retire` (8410d7451) | Go port (S4) removed |
| `compare` | `cli/cmd_analysis.py`, `compare_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `profile` | `cli/cmd_analysis.py`, `profile_search.py` | `pre-go-retire` (8410d7451) | Go port (S4) removed |
| `graph` | `cli/cmd_analysis.py`, `graph_search.py` | `pre-go-retire` (8410d7451) | Go port (S4) removed |
| `hotspots` | `cli/cmd_analysis.py`, `hotspots_search.py` | `pre-go-retire` (8410d7451) | Go port (S4) removed |
| `module-summary` | `cli/cmd_analysis.py`, `module_summary.py` | `pre-go-retire` (8410d7451) | Go port (S4) removed (with its CPython set-order emulation, pyset.go) |
| `fuzzy` | `cli/cmd_docs.py`, `fuzzy_search.py` | `pre-go-retire` (8410d7451) | never ported as a command; its search stays inside selftest --recall (sampled row) |
| `snippet` | `cli/cmd_source.py`, `source_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `callers` | `cli/cmd_source.py`, `source_search.py` | `pre-go-retire` (8410d7451) | never ported (mnav `callers` is a different, kept tool) |
| `chain` | `cli/cmd_source.py`, `chain_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `examples` | `cli/cmd_source.py`, `examples_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `annotations` | `cli/cmd_source.py`, `annotations_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `recipe` | `cli/cmd_develop.py`, `recipe_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `checklist` | `cli/cmd_develop.py`, `recipe_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `topics` | `cli/cmd_develop.py`, `topics_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `action-params` | `cli/cmd_develop.py`, `topics_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `event-flow` | `cli/cmd_develop.py`, `topics_search.py` | `pre-go-retire` (8410d7451) | never ported |
| `export` | `cli/cmd_export.py`, `export_search.py`, `html_export.py`, `module_export.py` | `pre-go-retire` (8410d7451) | never ported |
| `export-html` | `cli/cmd_export.py`, `html_export.py` | `pre-go-retire` (8410d7451) | alias of `export --html`; never ported |
| `export-module` | `cli/cmd_export.py`, `module_export.py` | `pre-go-retire` (8410d7451) | alias of `export --module`; never ported |
| `diff-api` | `cli/cmd_export.py`, `api_diff.py` | `pre-go-retire` (8410d7451) | never ported |
| `build-call-index` | `cli/cmd_admin.py`, `../build_call_index.py` | `pre-go-retire` (8410d7451) | index builder; never ported (D32) |
| `repl` | `cli/cmd_admin.py`, `cli/repl.py` | `pre-go-retire` (8410d7451) | never ported |

## mnav (module-navigator)

Kept: class, search, inventory, module, modules, rank, ask, hierarchy, implementors, drivers,
grep, source, token, license-feature, license-features, resources, callers, callees, stats,
palette-lexicon-agents, selftest (with --recall).

Before module-navigator deleted its Python CLI tests, D48 checked them against Go mnav: every
difference is in a retired command or is already marked intended, so no kept command has a gap
([d48-mnav-cli-test-triage.md](d48-mnav-cli-test-triage.md)).

Python: module-navigator `tools/module_nav.py`. Module paths are relative to `tools/`; "registered in" is the `commands/` file that declares the subcommand.

| Command | Python module | Registered in | Last Python commit | Notes |
|---|---|---|---|---|
| `package` | `module_nav_lib/class_search.py` | `commands/lookup.py` | `pre-go-retire` (f897268) | was ported (S2); Go code removed |
| `snippet` | `module_nav_lib/grep_search.py` | `commands/source.py` | `pre-go-retire` (f897268) | S4 slice, never ported |
| `imports` | `module_nav_lib/header_imports.py` | `commands/source.py` | `pre-go-retire` (f897268) | S4 slice, never ported |
| `ui` | `module_nav_lib/swing_search.py` | `commands/ui.py` | `pre-go-retire` (f897268) |  |
| `xref` | `module_nav_lib/xref.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `deps` | `module_nav_lib/xref.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `method` | `module_nav_lib/methods.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `methods` | `module_nav_lib/methods.py` | `commands/structure.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `slots` | `module_nav_lib/annotations.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `annotations` | `module_nav_lib/annotations.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `profile` | `module_nav_lib/profile.py` | `commands/lookup.py` | `pre-go-retire` (f897268) |  |
| `repl` | `module_nav_lib/repl/` | `commands/session.py` | `pre-go-retire` (f897268) |  |
| `full-profile` | `module_nav_lib/help_bridge/reports.py` | `commands/lookup.py` | `pre-go-retire` (f897268) |  |
| `cross-ref` | `module_nav_lib/help_bridge/reports.py` | `commands/lookup.py` | `pre-go-retire` (f897268) |  |
| `patch-target` | `module_nav_lib/patch/target.py` | `commands/patching.py` | `pre-go-retire` (f897268) |  |
| `patch-plan` | `module_nav_lib/patch/plan.py` | `commands/patching.py` | `pre-go-retire` (f897268) |  |
| `extract` | `module_nav_lib/patch/plan.py` | `commands/patching.py` | `pre-go-retire` (f897268) |  |
| `orphans` | `module_nav_lib/analysis.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `deps-graph` | `module_nav_lib/analysis.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `api-surface` | `module_nav_lib/analysis.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `security-audit` | `module_nav_lib/analysis.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `strings` | `module_nav_lib/extras.py` | `commands/source.py` | `pre-go-retire` (f897268) |  |
| `trace-type` | `module_nav_lib/extras.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `version-diff` | `module_nav_lib/extras.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `call-chain` | `module_nav_lib/callgraph.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `hotspots` | `module_nav_lib/callgraph.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `fields` | `module_nav_lib/fields.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `field` | `module_nav_lib/fields.py` | `commands/structure.py` | `pre-go-retire` (f897268) |  |
| `type-consumers` | `module_nav_lib/typeflow.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `type-producers` | `module_nav_lib/typeflow.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `type-flow` | `module_nav_lib/typeflow.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `module-calls` | `module_nav_lib/coupling.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `module-api-usage` | `module_nav_lib/coupling.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `coupling` | `module_nav_lib/coupling.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `patterns` | `module_nav_lib/patterns.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `pattern` | `module_nav_lib/patterns.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `bog-trace` | `module_nav_lib/bog_bridge.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `bog-classes` | `module_nav_lib/bog_bridge.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `bog-coverage` | `module_nav_lib/bog_bridge.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `throws` | `module_nav_lib/exceptions.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `catches` | `module_nav_lib/exceptions.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `export` | `module_nav_lib/export/` | `commands/patching.py` | `pre-go-retire` (f897268) |  |
| `impact` | `module_nav_lib/impact.py` | `commands/flow.py` | `pre-go-retire` (f897268) |  |
| `find` | `module_nav_lib/fuzzy.py` | `commands/lookup.py` | `pre-go-retire` (f897268) |  |
| `deprecated` | `module_nav_lib/deprecated.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `deprecated-users` | `module_nav_lib/deprecated.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `deprecated-risk` | `module_nav_lib/deprecated.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `similar` | `module_nav_lib/similarity.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `clones` | `module_nav_lib/similarity.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `config-usage` | `module_nav_lib/config_coupling.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `config-keys` | `module_nav_lib/config_coupling.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `resources-usage` | `module_nav_lib/config_coupling.py` | `commands/source.py` | `pre-go-retire` (f897268) |  |
| `serial` | `module_nav_lib/serialization.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `serial-audit` | `module_nav_lib/serialization.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `serial-conflicts` | `module_nav_lib/serialization.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `thread-safety` | `module_nav_lib/threading.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `thread-scan` | `module_nav_lib/threading.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `lifecycle` | `module_nav_lib/lifecycle.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `lifecycle-scan` | `module_nav_lib/lifecycle.py` | `commands/quality.py` | `pre-go-retire` (f897268) |  |
| `string-search` | `module_nav_lib/string_constants.py` | `commands/source.py` | `pre-go-retire` (f897268) |  |
| `string-constants` | `module_nav_lib/string_constants.py` | `commands/source.py` | `pre-go-retire` (f897268) |  |
| `servlets` | `module_nav_lib/servlets.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `routes` | `module_nav_lib/servlets.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `servlet` | `module_nav_lib/servlets.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `ords` | `module_nav_lib/ords.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `ord-usage` | `module_nav_lib/ords.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `ord-flow` | `module_nav_lib/ords.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `topics` | `module_nav_lib/topics/listing.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `subscribers` | `module_nav_lib/topics/subscribers.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `pub-sub` | `module_nav_lib/topics/listing.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `bql` | `module_nav_lib/bql.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `bql-tables` | `module_nav_lib/bql.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `bql-class` | `module_nav_lib/bql.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `permissions` | `module_nav_lib/permissions/` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `license-inspect` | `module_nav_lib/license_inspect.py` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `policy-inspect` | `module_nav_lib/policy_inspect.py` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `trust-anchor-check` | `module_nav_lib/trust_anchor.py` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `bypass-status` | `module_nav_lib/bypass_status.py` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `permission-report` | `module_nav_lib/permission_report.py` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `explain` | `module_nav_lib/explain.py` | `commands/lookup.py` | `pre-go-retire` (f897268) |  |
| `feasibility-check` | `module_nav_lib/feasibility.py` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `permission-flow` | `module_nav_lib/permissions/flow.py` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `credentials` | `module_nav_lib/permissions/credentials.py` | `commands/security.py` | `pre-go-retire` (f897268) |  |
| `alarm-flow` | `module_nav_lib/alarm_domain/flow.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `alarm-types` | `module_nav_lib/alarm_domain/flow.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `alarm-trace` | `module_nav_lib/alarm_domain/trace.py` | `commands/domain.py` | `pre-go-retire` (f897268) |  |
| `cycles` | `module_nav_lib/architecture.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `god-classes` | `module_nav_lib/architecture.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `layer-check` | `module_nav_lib/architecture.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `complexity` | `module_nav_lib/complexity.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `complexity-scan` | `module_nav_lib/complexity.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `metrics` | `module_nav_lib/complexity.py` | `commands/architecture.py` | `pre-go-retire` (f897268) |  |
| `bookmark` | `module_nav_lib/bookmarks.py` | `commands/session.py` | `pre-go-retire` (f897268) |  |
| `bookmarks` | `module_nav_lib/bookmarks.py` | `commands/session.py` | `pre-go-retire` (f897268) |  |
| `unbookmark` | `module_nav_lib/bookmarks.py` | `commands/session.py` | `pre-go-retire` (f897268) |  |
| `history` | `module_nav_lib/bookmarks.py` | `commands/session.py` | `pre-go-retire` (f897268) |  |
| `unified` | `module_nav_lib/unified/search.py` | `commands/lookup.py` | `pre-go-retire` (f897268) |  |
| `compare` | `module_nav_lib/unified/compare.py` | `commands/lookup.py` | `pre-go-retire` (f897268) |  |
| `integration-contract` | `module_nav_lib/contracts/report.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `example-mine` | `module_nav_lib/mining.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `virtual-callers` | `module_nav_lib/virtual_dispatch.py` | `commands/flow.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `feature-brief` | `module_nav_lib/research/brief.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `slot-validate` | `module_nav_lib/slot_validate.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `slot-collision` | `module_nav_lib/slot_collision.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `ord-validate` | `module_nav_lib/ord_validate.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `resolve-audit` | `module_nav_lib/resolve_audit.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `driver-cleanup-audit` | `module_nav_lib/driver_cleanup.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `resource-leak` | `module_nav_lib/resource_leak.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `service-order` | `module_nav_lib/service_order.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `dependency-audit` | `module_nav_lib/dependency_audit.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `module-health` | `module_nav_lib/module_health/report.py` | `commands/audits.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
| `session` | `module_nav_lib/sessions.py` | `commands/session.py` | `pre-go-retire` (f897268) |  |
| `full-stack-trace` | `module_nav_lib/fullstack.py` | `commands/flow.py` | `pre-go-retire` (f897268) | only caller: module-navigator's own tests (contract consumer table) |
