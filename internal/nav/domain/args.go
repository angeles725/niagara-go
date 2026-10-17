// Package domain declares the module-nav domain commands (Niagara domain: drivers, servlets, ORDs, topics, BQL, alarms, BOG);
// they register into nav/registry from init.
package domain

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("domain", []registry.Command{
		{Name: "drivers", Help: "Driver networks and their anatomy (device, extensions, managers, license)", Description: "One row per driver network (a BDeviceNetwork subclass in an -rt module): family, device, license feature, doc module. With a module: the full anatomy of its networks (devices, point-device-ext, proxy-ext, managers, discovery jobs, license features, doc module).", Epilog: "examples: drivers | drivers modbusTcp-rt | drivers niagaraDriver --json", Args: argsDrivers},
		{Name: "bog-trace", Help: "Trace BOG component to Java class with profile", Args: argsBogTrace},
		{Name: "bog-classes", Help: "List unique Niagara types in BOG + Java resolution", Args: argsBogClasses},
		{Name: "bog-coverage", Help: "Code-base vs BOG coverage analysis", Args: argsBogCoverage},
		{Name: "servlets", Help: "List all web servlets with their routes and HTTP handlers", Args: argsServlets},
		{Name: "routes", Help: "Map HTTP verbs to servlet classes and handler methods", Args: argsRoutes},
		{Name: "servlet", Help: "Detailed info for a specific servlet class", Args: argsServlet},
		{Name: "ords", Help: "List all ORDs (Object Resolution Descriptors) grouped by scheme", Args: argsOrds},
		{Name: "ord-usage", Help: "Classes that use a specific ORD scheme", Args: argsOrdUsage},
		{Name: "ord-flow", Help: "ORDs created/resolved by a specific class", Args: argsOrdFlow},
		{Name: "topics", Help: "List all @NiagaraTopic declarations with publishers/subscribers", Args: argsTopics},
		{Name: "subscribers", Help: "Who subscribes to or fires a specific topic or class", Args: argsSubscribers},
		{Name: "pub-sub", Help: "Publish->subscribe graph for a module", Args: argsPubSub},
		{Name: "bql", Help: "List BQL/SQL queries found in the corpus", Args: argsBql},
		{Name: "bql-tables", Help: "Tables/sources most queried by BQL and SQL", Args: argsBqlTables},
		{Name: "bql-class", Help: "BQL/SQL queries used by a specific class", Args: argsBqlClass},
		{Name: "alarm-flow", Help: "Classes in the alarm chain by role", Args: argsAlarmFlow},
		{Name: "alarm-types", Help: "Alarm types defined in the corpus"},
		{Name: "alarm-trace", Help: "Role of a class in the alarm flow", Args: argsAlarmTrace},
	})
}

// argsDrivers declares `drivers`.
func argsDrivers(p *argparse.Parser) {
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Submodule (modbusTcp-rt) or module directory (modbusTcp)"}, "module")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "JSON output", Dest: "drivers_json"}, "--json")
}

// argsBogTrace declares `bog-trace`.
func argsBogTrace(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "BOG path (e.g. '/Drivers/BacnetNetwork/Termo1') or type (e.g. 'c:NumericWritable')"}, "query")
	p.Add(argparse.Arg{Help: "Path to bog_index.json (auto-detected if omitted)"}, "--bog-index")
}

// argsBogClasses declares `bog-classes`.
func argsBogClasses(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Path to bog_index.json (auto-detected if omitted)"}, "--bog-index")
}

// argsBogCoverage declares `bog-coverage`.
func argsBogCoverage(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Path to bog_index.json (auto-detected if omitted)"}, "--bog-index")
}

// argsServlets declares `servlets`.
func argsServlets(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "srv_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsRoutes declares `routes`.
func argsRoutes(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter by HTTP verb (GET, POST, PUT, DELETE, SERVICE)", Dest: "rt_verb"}, "--verb")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "rt_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsServlet declares `servlet`.
func argsServlet(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Servlet class name (e.g. BBoxServlet)"}, "class_name")
}

// argsOrds declares `ords`.
func argsOrds(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "ords_module"}, "--module", "-m")
	p.Add(argparse.Arg{Help: "Filter by ORD scheme (station, slot, history, module, etc.)", Dest: "ords_scheme"}, "--scheme", "-s")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsOrdUsage declares `ord-usage`.
func argsOrdUsage(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "ORD scheme (e.g. station, history, module)"}, "scheme")
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "ordu_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsOrdFlow declares `ord-flow`.
func argsOrdFlow(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
}

// argsTopics declares `topics`.
func argsTopics(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "topics_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsSubscribers declares `subscribers`.
func argsSubscribers(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Topic name (e.g. alarm) or class name (e.g. BAlarmService)"}, "query")
}

// argsPubSub declares `pub-sub`.
func argsPubSub(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Module name (e.g. alarm-rt)"}, "module_name")
}

// argsBql declares `bql`.
func argsBql(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "bql_module"}, "--module", "-m")
	p.Add(argparse.Arg{Help: "Filter by kind: bql or sql", Dest: "bql_type"}, "--type", "-t")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsBqlTables declares `bql-tables`.
func argsBqlTables(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "bqlt_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsBqlClass declares `bql-class`.
func argsBqlClass(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
}

// argsAlarmFlow declares `alarm-flow`.
func argsAlarmFlow(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Filter to a specific module", Dest: "alarm_module"}, "--module", "-m")
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Max results to show (default 50)"}, "-n")
}

// argsAlarmTrace declares `alarm-trace`.
func argsAlarmTrace(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name (e.g. BAlarmService)"}, "class_name")
}
