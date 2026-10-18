package check

// Check kinds.
const (
	kindType      = "type"       // components of Type (under a Within ancestor) counted against N
	kindSlot      = "slot"       // the effective value of Slot on every Type component
	kindLink      = "link"       // links From.FromSlot -> Type.Slot counted against N
	kindFiles     = "files"      // files matching Glob under the station directory
	kindJar       = "jar"        // <install>/modules/<Jar>.jar exists
	kindJarEntry  = "jar-entry"  // entries of the jar matching Glob counted against N
	kindJarText   = "jar-text"   // the jar entry Glob holds every Contains string
	kindJarSigned = "jar-signed" // the jar has a META-INF signature file and signature block
	kindFileText  = "file-text"  // every file matching Glob has Line (or N lines) matching Match
	kindAudit     = "audit"      // audit records of the Glob history files matching Hist, counted against N
	kindLog       = "log"        // log records of the Glob history files matching Hist, counted against N
	kindTrend     = "trend"      // every Glob trend history: N records (Distinct: N values), values in Hist's range
)

// Count operators.
const (
	opMin = "min" // at least N
	opEq  = "eq"  // exactly N
)

// Check is one row of the spec.
type Check struct {
	ID   string
	Lab  string   // "1".."11", "B" (Bonus lab)
	Keys []string // answer-key items the check confirms
	// Unobserved maps a key to what of that item's answer the check does not observe (a console
	// line, a log, a view, a runtime status): when every passing check of a predicted item has a
	// note, verify-predictions reports PARTIAL, never CONFIRMED. No note = the whole answer.
	Unobserved map[string]string
	Kind       string
	Desc       string

	Type   string // component type "module:Type" (type, slot, link target)
	Within string // type, slot: an ancestor type "module:Type" every counted component must have
	Under  string // type, slot: a folder path ("Review2") every counted component must be below
	Op     string // type, link, files, jar-entry, slot with Distinct, file-text without Line, audit, log, trend: opMin or opEq
	N      int

	Slot     string   // slot: the slot; link: the target slot
	Want     string   // slot: the expected effective value
	Default  string   // slot: the type's default, the value of a slot the bog does not store
	Set      bool     // slot: the slot must be stored (differ from its default) instead of Want
	Distinct bool     // slot: N distinct effective values across the components instead of Want; trend: N distinct values per file
	Stored   []string // slot: further slots every component must also store (a down = false is the default on a component never pinged)

	From     string // link: source type "module:Type"
	FromSlot string // link: source slot

	Glob     string   // files, file-text, audit, log, trend: path under the station dir ({station} = station name); jar-*: entry
	Jar      string   // jar-*: module part name, e.g. training-rt
	Contains []string // jar-text

	Line  int    // file-text: the 1-based line that must match (a BOM and CR line ends dropped); 0 = count lines
	Match string // file-text: a Go regexp; {station} = the station name, quoted

	Hist *HistMatch // audit, log: the record fields to match; trend: the value range
}
