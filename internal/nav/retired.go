package nav

// Retired are the module-nav commands the Go port does not carry (decision D32: no
// recorded use), in Python's --help order. Each still parses to one stderr line
// pointing to docs/retired-commands.md and exit status 2; none is listed in --help.
var Retired = []string{
	"package", "snippet", "imports", "ui", "xref", "deps", "method", "methods", "slots",
	"annotations", "profile", "repl", "full-profile", "cross-ref", "patch-target", "patch-plan",
	"extract", "orphans", "deps-graph", "api-surface", "security-audit", "strings", "trace-type",
	"version-diff", "call-chain", "hotspots", "fields", "field", "type-consumers", "type-producers",
	"type-flow", "module-calls", "module-api-usage", "coupling", "patterns", "pattern", "bog-trace",
	"bog-classes", "bog-coverage", "throws", "catches", "export", "impact", "find", "deprecated",
	"deprecated-users", "deprecated-risk", "similar", "clones", "config-usage", "config-keys",
	"resources-usage", "serial", "serial-audit", "serial-conflicts", "thread-safety", "thread-scan",
	"lifecycle", "lifecycle-scan", "string-search", "string-constants", "servlets", "routes",
	"servlet", "ords", "ord-usage", "ord-flow", "topics", "subscribers", "pub-sub", "bql",
	"bql-tables", "bql-class", "permissions", "license-inspect", "policy-inspect",
	"trust-anchor-check", "bypass-status", "permission-report", "explain", "feasibility-check",
	"permission-flow", "credentials", "alarm-flow", "alarm-types", "alarm-trace", "cycles",
	"god-classes", "layer-check", "complexity", "complexity-scan", "metrics", "bookmark", "bookmarks",
	"unbookmark", "history", "unified", "compare", "integration-contract", "example-mine",
	"virtual-callers", "feature-brief", "slot-validate", "slot-collision", "ord-validate",
	"resolve-audit", "driver-cleanup-audit", "resource-leak", "service-order", "dependency-audit",
	"module-health", "session", "full-stack-trace",
}
