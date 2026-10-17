// Package quality declares the module-nav quality commands (Code quality);
// they register into nav/registry from init.
package quality

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("quality", []registry.Command{
		{Name: "orphans", Help: "Find classes never imported (dead code)", Args: argsOrphans},
		{Name: "api-surface", Help: "Public API surface of a module", Args: argsApiSurface},
		{Name: "security-audit", Help: "Scan for insecure patterns", Args: argsSecurityAudit},
		{Name: "deprecated", Help: "List @Deprecated classes and methods", Args: argsDeprecated},
		{Name: "deprecated-users", Help: "Who uses a deprecated class or method", Args: argsDeprecatedUsers},
		{Name: "deprecated-risk", Help: "Deprecated usage risk summary", Args: argsDeprecatedRisk},
		{Name: "similar", Help: "Find classes with similar structure", Args: argsSimilar},
		{Name: "clones", Help: "Top duplicate class pairs", Args: argsClones},
		{Name: "config-usage", Help: "Classes that read configuration", Args: argsConfigUsage},
		{Name: "config-keys", Help: "Top config keys referenced in code", Args: argsConfigKeys},
		{Name: "serial", Help: "Serialization info for a class", Args: argsSerial},
		{Name: "serial-audit", Help: "Audit Serializable classes: missing UIDs, risks", Args: argsSerialAudit},
		{Name: "serial-conflicts", Help: "Classes with same serialVersionUID (collision)", Args: argsSerialConflicts},
		{Name: "thread-safety", Help: "Thread safety analysis for a class", Args: argsThreadSafety},
		{Name: "thread-scan", Help: "Module-wide thread safety summary", Args: argsThreadScan},
		{Name: "lifecycle", Help: "Lifecycle methods implemented by a class", Args: argsLifecycle},
		{Name: "lifecycle-scan", Help: "Classes with lifecycle overrides", Args: argsLifecycleScan},
	})
}

// argsOrphans declares `orphans`.
func argsOrphans(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "orph_module"}, "--module", "-m")
	p.Add(argparse.Arg{Help: "Filter by submodule type (rt, wb, ux)", Dest: "orph_type"}, "--type", "-t")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results (default 50)"}, "-n")
}

// argsApiSurface declares `api-surface`.
func argsApiSurface(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "name")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results (default 50)"}, "-n")
}

// argsSecurityAudit declares `security-audit`.
func argsSecurityAudit(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module (recommended)", Dest: "sec_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 30, Type: argparse.Int, Help: "Max findings per category (default 30)"}, "-n")
}

// argsDeprecated declares `deprecated`.
func argsDeprecated(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "dep_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsDeprecatedUsers declares `deprecated-users`.
func argsDeprecatedUsers(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BLegacyService)"}, "class_name")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Method name (optional; omit for class-level)"}, "method")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsDeprecatedRisk declares `deprecated-risk`.
func argsDeprecatedRisk(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "deprisk_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsSimilar declares `similar`.
func argsSimilar(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BNumericWritable)"}, "class_name")
	p.Add(argparse.Arg{Default: 20, Type: argparse.Int, Help: "Limit results (default 20)"}, "-n")
}

// argsClones declares `clones`.
func argsClones(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "clones_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 20, Type: argparse.Int, Help: "Limit results (default 20)"}, "-n")
}

// argsConfigUsage declares `config-usage`.
func argsConfigUsage(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "cfgu_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
}

// argsConfigKeys declares `config-keys`.
func argsConfigKeys(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
}

// argsSerial declares `serial`.
func argsSerial(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmRecord)"}, "class_name")
}

// argsSerialAudit declares `serial-audit`.
func argsSerialAudit(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "sa_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
}

// argsSerialConflicts declares `serial-conflicts`.
func argsSerialConflicts(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 30, Type: argparse.Int, Help: "Limit results (default 30)"}, "-n")
}

// argsThreadSafety declares `thread-safety`.
func argsThreadSafety(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
}

// argsThreadScan declares `thread-scan`.
func argsThreadScan(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "ts_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
}

// argsLifecycle declares `lifecycle`.
func argsLifecycle(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Class name (e.g. BAlarmService)"}, "class_name")
	p.Add(argparse.Arg{Help: "Show classes implementing a specific lifecycle (e.g. started)", Dest: "lc_pattern"}, "--pattern", "-p")
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "lc_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
}

// argsLifecycleScan declares `lifecycle-scan`.
func argsLifecycleScan(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by module", Dest: "lcs_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
}
