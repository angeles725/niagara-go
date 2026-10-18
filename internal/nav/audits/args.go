// Package audits declares the module-nav audits commands (Module audits);
// they register into nav/registry from init.
package audits

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("audits", []registry.Command{
		{Name: "integration-contract", Help: "Extract integration contract from real corpus usages", Args: argsIntegrationContract},
		{Name: "example-mine", Help: "Mine real usage examples (full method bodies) of a class", Args: argsExampleMine},
		{Name: "feature-brief", Help: "One-command research brief for a Niagara feature domain", Args: argsFeatureBrief},
		{Name: "slot-validate", Help: "Pre-flight validation for @NiagaraProperty slots", Args: argsSlotValidate},
		{Name: "slot-collision", Help: "Detect slot name collisions across modules", Args: argsSlotCollision},
		{Name: "ord-validate", Help: "Validate ORDs referencing non-existent station tree paths", Args: argsOrdValidate},
		{Name: "resolve-audit", Help: "Audit .resolve() calls for null-safety", Args: argsResolveAudit},
		{Name: "driver-cleanup-audit", Help: "Audit driver doStop() cleanup", Args: argsDriverCleanupAudit},
		{Name: "resource-leak", Help: "Audit resource acquisitions for missing close()", Args: argsResourceLeak},
		{Name: "service-order", Help: "Analyze service startup order issues", Args: argsServiceOrder},
		{Name: "dependency-audit", Help: "Audit third-party and external dependencies", Args: argsDependencyAudit},
		{Name: "module-health", Help: "Overall module health scorecard (A/B/C/D/F)", Args: argsModuleHealth},
		{Name: "palette-lexicon-agents", Help: "Palette entries, lexicon keys (+ dup-key report), and agent registrations for a module", Args: argsPaletteLexiconAgents, Run: runPaletteLexiconAgents},
	})
}

// argsIntegrationContract declares `integration-contract`.
func argsIntegrationContract(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
	p.Add(argparse.Arg{Default: 5, Type: argparse.Int, Help: "Min external callers for entry-point methods (default 5)", Dest: "ic_min_callers"}, "--min-callers")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "ic_json"}, "--json")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Aggregate entry-points via virtual dispatch", Dest: "ic_resolve_virtual"}, "--resolve-virtual")
}

// argsExampleMine declares `example-mine`.
func argsExampleMine(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
	p.Add(argparse.Arg{Help: "Filter method bodies by substring (case-insensitive)", Dest: "em_pattern"}, "--pattern")
	p.Add(argparse.Arg{Default: 3, Type: argparse.Int, Help: "Max examples to return (default 3)", Dest: "em_top"}, "--top")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Skip com/tridium/* files", Dest: "em_excl"}, "--exclude-tridium")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Include docSource-doc module (mirror copy; excluded by default)", Dest: "em_incl_doc"}, "--include-docsource")
	p.Add(argparse.Arg{Default: 5, Type: argparse.Int, Help: "Minimum method body lines (default 5)", Dest: "em_min_lines"}, "--min-lines")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "em_json"}, "--json")
}

// argsFeatureBrief declares `feature-brief`.
func argsFeatureBrief(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Feature name (e.g. alarms, schedules, points)"}, "feature")
	p.Add(argparse.Arg{Default: "quick", Choices: []any{"quick", "full"}, Help: "quick=top-level, full=with code examples (default quick)", Dest: "fb_depth"}, "--depth")
	p.Add(argparse.Arg{Help: "Write brief to file instead of stdout", Dest: "fb_out"}, "--out")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "fb_json"}, "--json")
}

// argsSlotValidate declares `slot-validate`.
func argsSlotValidate(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Target class (e.g. BMyDashboardService)"}, "class_name")
	p.Add(argparse.Arg{Required: true, Help: "Slot spec: name:type[:defaultValue]", Dest: "sv_add"}, "--add")
	p.Add(argparse.Arg{Default: "property", Choices: []any{"property", "action", "topic"}, Help: "Slot kind (default: property)", Dest: "sv_kind"}, "--kind")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "sv_json"}, "--json")
}

// argsSlotCollision declares `slot-collision`.
func argsSlotCollision(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter: only show collisions involving this module", Dest: "sc_module"}, "--module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "sc_json"}, "--json")
}

// argsOrdValidate declares `ord-validate`.
func argsOrdValidate(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "ov_json"}, "--json")
}

// argsResolveAudit declares `resolve-audit`.
func argsResolveAudit(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "ra_json"}, "--json")
}

// argsDriverCleanupAudit declares `driver-cleanup-audit`.
func argsDriverCleanupAudit(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. bacnet-rt)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "dca_json"}, "--json")
}

// argsResourceLeak declares `resource-leak`.
func argsResourceLeak(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. fox-rt)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "rl_json"}, "--json")
}

// argsServiceOrder declares `service-order`.
func argsServiceOrder(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. nmodsreflow-rt)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "so_json"}, "--json")
}

// argsDependencyAudit declares `dependency-audit`.
func argsDependencyAudit(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "da_json"}, "--json")
}

// argsModuleHealth declares `module-health`.
func argsModuleHealth(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "mh_json"}, "--json")
}

// argsPaletteLexiconAgents declares `palette-lexicon-agents`.
func argsPaletteLexiconAgents(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name in organized/ (e.g. alarm)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Structured JSON output", Dest: "pla_json"}, "--json")
}
