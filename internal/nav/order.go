package nav

// Order is the --help order of the commands the Go port keeps (module_nav_lib/commands/__init__.py
// ORDER without the retired ones, Retired).
var Order = []string{
	"selftest", "inventory", "module", "modules", "class", "search", "grep", "rank", "source",
	"license-feature", "license-features", "hierarchy", "implementors", "drivers", "stats",
	"resources", "callers", "callees", "token", "ask", "palette-lexicon-agents",
}

// Group is one heading of the grouped --help list: the package under nav/ that
// registers the commands, and its title.
type Group struct{ Name, Title string }

// Groups lists the headings in --help order (module_nav_lib/commands/__init__.py GROUPS) of
// the groups that keep a command.
var Groups = []Group{
	{"inventory", "Inventory and self-checks"},
	{"lookup", "Find classes and concepts"},
	{"source", "Source text, strings and resources"},
	{"structure", "Types: hierarchy, members, imports"},
	{"flow", "Call and data flow"},
	{"domain", "Niagara domain: drivers, servlets, ORDs, topics, BQL, alarms, BOG"},
	{"security", "Security, licensing and permissions"},
	{"audits", "Module audits"},
}
