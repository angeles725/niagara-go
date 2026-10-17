package nav

// Order is the --help order of the commands (module_nav_lib/commands/__init__.py ORDER).
var Order = []string{
	"selftest", "inventory", "module", "modules", "class", "search", "package", "grep", "rank",
	"source", "snippet", "imports", "ui", "hierarchy", "implementors", "xref", "deps", "method",
	"methods", "slots", "annotations", "stats", "profile", "repl", "full-profile", "cross-ref",
	"patch-target", "patch-plan", "extract", "orphans", "deps-graph", "api-surface",
	"security-audit", "strings", "resources", "trace-type", "version-diff", "callers", "callees",
	"call-chain", "hotspots", "fields", "field", "type-consumers", "type-producers", "type-flow",
	"module-calls", "module-api-usage", "coupling", "patterns", "pattern", "throws", "catches",
	"export", "token", "impact", "find", "deprecated", "deprecated-users", "deprecated-risk",
	"similar", "clones", "config-usage", "config-keys", "resources-usage", "serial",
	"serial-audit", "serial-conflicts", "thread-safety", "thread-scan", "lifecycle",
	"lifecycle-scan", "string-search", "string-constants", "ask", "explain", "cycles",
	"god-classes", "layer-check", "complexity", "complexity-scan", "metrics", "bookmark",
	"bookmarks", "unbookmark", "history", "unified", "compare", "virtual-callers", "session",
	"full-stack-trace",
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
	{"flow", "Call and data flow"},
	{"ui", "UI and Swing"},
	{"patching", "Patching"},
	{"quality", "Code quality"},
	{"architecture", "Architecture and dependencies"},
	{"session", "Session: bookmarks, history, REPL"},
}
