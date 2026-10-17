package nav

// Order is the --help order of the commands (module_nav_lib/commands/__init__.py ORDER).
var Order = []string{
	"selftest", "inventory", "module", "modules", "class", "search", "package", "grep", "rank",
	"source", "snippet", "imports", "ui", "hierarchy", "implementors", "xref", "deps", "method",
	"methods", "slots", "annotations", "stats", "profile", "repl", "full-profile", "cross-ref",
	"patch-target", "patch-plan", "extract", "deps-graph", "strings", "resources", "trace-type",
	"version-diff", "fields", "field", "export", "token", "find", "resources-usage",
	"string-search", "string-constants", "ask", "explain", "bookmark", "bookmarks", "unbookmark",
	"history", "unified", "compare", "session",
}

// Group is one heading of the grouped --help list: the package under nav/ that
// registers the commands, and its title.
type Group struct{ Name, Title string }

// Groups lists the headings in --help order (module_nav_lib/commands/__init__.py GROUPS).
var Groups = []Group{
	{"inventory", "Inventory and self-checks"},
	{"lookup", "Find classes and concepts"},
	{"source", "Source text, strings and resources"},
	{"structure", "Types: hierarchy, members, imports"},
	{"ui", "UI and Swing"},
	{"patching", "Patching"},
	{"session", "Session: bookmarks, history, REPL"},
}
