package course

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
	Within string // type: an ancestor type "module:Type" every counted component must have
	Op     string // type, link, files, jar-entry, slot with Distinct: opMin or opEq
	N      int

	Slot     string   // slot: the slot; link: the target slot
	Want     string   // slot: the expected effective value
	Default  string   // slot: the type's default, the value of a slot the bog does not store
	Set      bool     // slot: the slot must be stored (differ from its default) instead of Want
	Distinct bool     // slot: N distinct effective values across the components instead of Want
	Stored   []string // slot: further slots every component must also store (a down = false is the default on a component never pinged)

	From     string // link: source type "module:Type"
	FromSlot string // link: source slot

	Glob     string   // files: path under the station dir ({station} = station name); jar-*: entry
	Jar      string   // jar-*: module part name, e.g. training-rt
	Contains []string // jar-text
}
