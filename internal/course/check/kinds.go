package check

import "github.com/angeles725/niagara-go/internal/course/key"

// Check kinds.
const (
	kindType       = "type"        // components of Type (under a Within ancestor) counted against N
	kindSlot       = "slot"        // the effective value of Slot on every Type component
	kindLink       = "link"        // links From.FromSlot -> Type.Slot counted against N
	kindFiles      = "files"       // files matching Glob under the row's Root (the station directory by default)
	kindJar        = "jar"         // <install>/modules/<Jar>.jar exists
	kindJarEntry   = "jar-entry"   // entries of the jar matching Glob counted against N
	kindJarText    = "jar-text"    // the jar entry Glob holds every Contains string
	kindJarSigned  = "jar-signed"  // the jar has a META-INF signature file and signature block
	kindJarCert    = "jar-cert"    // the Fact of the signer certificate of the jar's signature block (jarcert.go)
	kindFileText   = "file-text"   // every file matching Glob under Root has Line (or N lines, or Whole: N matches) matching Match
	kindAudit      = "audit"       // audit records of the Glob history files matching Hist, counted against N
	kindLog        = "log"         // log records of the Glob history files matching Hist, counted against N
	kindTrend      = "trend"       // every Glob trend history: N records (Distinct: N values), values in Hist's range
	kindSourceText = "source-text" // the matches of Match in the source-tree file Glob counted against N
	kindAnnotation = "annotation"  // the Fact the annotations of the lab-source file Glob declare equals Want
	kindListing    = "listing"     // the station file Glob names exactly the Type components, each with its Slot value (listing.go)
)

// evidenceOf is the evidence kind of a verified marker each check kind gives (key.EvidenceLive,
// key.EvidenceOffline): live for what a station stores (its bog, the files in its directory, its
// histories), offline for what holds no station (a jar of the install, the source tree, the lab
// sources). A files or file-text row with an install or lab-source Root is offline whatever its
// kind says (checkEvidence). TestEvidenceOfEveryKind fails on a spec kind missing here.
var evidenceOf = map[string]string{
	kindType: key.EvidenceLive, kindSlot: key.EvidenceLive, kindLink: key.EvidenceLive,
	kindFiles: key.EvidenceLive, kindFileText: key.EvidenceLive, kindListing: key.EvidenceLive,
	kindAudit: key.EvidenceLive, kindLog: key.EvidenceLive, kindTrend: key.EvidenceLive,
	kindJar: key.EvidenceOffline, kindJarEntry: key.EvidenceOffline, kindJarText: key.EvidenceOffline,
	kindJarSigned: key.EvidenceOffline, kindJarCert: key.EvidenceOffline, kindSourceText: key.EvidenceOffline, kindAnnotation: key.EvidenceOffline,
}

// Count operators.
const (
	opMin = "min" // at least N
	opEq  = "eq"  // exactly N
)

// Check is one row of the spec.
type Check struct {
	ID   string
	Lab  string   // "1".."11", "B" (Bonus lab), "RA1", "RA2" (review assignments), "EXAM"
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
	Op     string // type, link, files, jar-entry, slot with Distinct, file-text without Line, audit, log, trend, source-text: opMin or opEq
	N      int

	Slot     string   // slot: the slot; link: the target slot; annotation: the one slot the Fact covers ("" = every slot)
	Want     string   // slot: the expected effective value; annotation: the expected Fact; listing: the priority every line is at
	Fact     string   // annotation: what the annotations declare (flags, enum, slots, facets: see annotation.go); jar-cert: what of the signer certificate (jarcert.go)
	CertOf   string   // jar-cert same: the jar part whose signer certificate must be the same
	Default  string   // slot: the type's default, the value of a slot the bog does not store
	Set      bool     // slot: the slot must be stored (differ from its default) instead of Want
	Distinct bool     // slot: N distinct effective values across the components instead of Want; trend: N distinct values per file
	Stored   []string // slot: further slots every component must also store (a down = false is the default on a component never pinged)

	From     string // link: source type "module:Type"
	FromSlot string // link: source slot

	Glob     string   // files, file-text: path under Root ({a,b} exact names; a name ending in "/" a directory); audit, log, trend: path under the station dir ({station} = station name); jar-*: entry; source-text: path under the source tree; annotation: path under the lab module sources
	Root     string   // files, file-text: rootStation (default), rootInstall or rootLabsrc (roots.go)
	Jar      string   // jar-*: module part name, e.g. training-rt
	Contains []string // jar-text

	Line  int    // file-text: the 1-based line that must match (a BOM and CR line ends dropped); 0 = count lines
	Match string // file-text: a Go regexp; {station} = the station name, quoted (station root); source-text: a Go regexp over the whole file; jar-cert: a Go regexp over a DN
	Whole bool   // file-text: Match runs over the whole text (lines joined with "\n"), its matches counted against N
	// Names (file-text with Whole): the first match's group 1 is a comma-separated NAME=value list
	// (a DN); its names, sorted and space-joined, must equal Names: each once, none other.
	Names string

	Hist *HistMatch // audit, log: the record fields to match; trend: the value range
}
