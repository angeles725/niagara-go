package nav

// Order is the --help order of the commands (module_nav_lib/commands/__init__.py ORDER).
var Order = []string{
	"selftest", "inventory", "module", "modules", "class", "search", "package", "ui", "stats",
	"profile", "full-profile", "cross-ref", "find", "ask", "explain", "unified", "compare",
}

// Group is one heading of the grouped --help list: the package under nav/ that
// registers the commands, and its title.
type Group struct{ Name, Title string }

// Groups lists the headings in --help order (module_nav_lib/commands/__init__.py GROUPS).
var Groups = []Group{
	{"inventory", "Inventory and self-checks"},
	{"lookup", "Find classes and concepts"},
	{"ui", "UI and Swing"},
}
