package bog

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Comp is one component of the graph: a <p> element that carries a handle (bognav/model.py Comp).
type Comp struct {
	Name   string // SlotPath-decoded name
	Handle string
	Type   string // e.g. "c:NumericWritable"
	Pfx    string // type prefix before ':' ("" without one)
	Module string // module the prefix maps to in the bog's m= declarations ("" when unknown)
	Path   string // slash-joined decoded names from the top component, without a leading '/'
	// ParentH is the handle of the nearest enclosing component; nil for a top-level component.
	ParentH *string
	// Children are the handles of the direct child components, in document order.
	Children []string
	// Slots are the value and composite properties stored on the component, in document order
	// (platform slots wsAnnotation, value, status and displayName excluded).
	Slots Ordered[*Slot]
	// Actions maps each <a n=..> action to its flags.
	Actions     Ordered[string]
	HasFallback bool
	IsWritable  bool // simple type is BooleanWritable, NumericWritable, FloatWritable or IntegerWritable
	// ValueType and ValueVal are the typed handle-less `value` child (<p n="value" t=".." v=".."/>);
	// ValueType is "" when the component has none.
	ValueType string
	ValueVal  *string
	// Present holds every direct <p n=..> child name the bog stores (platform slots included):
	// the slots whose value is not the type default.
	Present map[string]struct{}
}

// SimpleType is the type name after the last ':'.
func (c *Comp) SimpleType() string {
	if i := strings.LastIndexByte(c.Type, ':'); i >= 0 {
		return c.Type[i+1:]
	}
	return c.Type
}

// Slot is a stored property of a component. Value is nil when the bog stores none (a composite
// property, or a flags-only slot). Child holds the nested value/status of a composite property.
type Slot struct {
	Type  string
	Value *string
	Flags string
	Child Ordered[string]
}

// Link is a b:Link or b:ConversionLink stored inside its target component (the container).
// A nil field is one the bog does not store.
type Link struct {
	ContainerH    *string
	ContainerPath string
	LinkName      string
	SrcH          *string // source handle of a sourceOrd "h:<handle>"
	SrcOrd        *string // any other sourceOrd
	SrcSlot       *string
	TgtSlot       *string
}

// LinkRow is a link rendered with resolved endpoints (bognav/model.py Bog.link_row).
type LinkRow struct {
	Link        string
	Source      string // "<source path>.<slot>"
	Target      string // "<container path>.<slot>"
	SrcResolved bool
}

// Bog is a parsed BOG graph (bognav/model.py Bog).
type Bog struct {
	SrcPath     string          // the path as given
	Prefixes    Ordered[string] // type prefix -> module, from every m= declaration
	Comps       Ordered[*Comp]  // handle -> component (a repeated handle keeps the last component)
	HandleCount Ordered[int]    // handle -> number of components declaring it
	Links       []*Link         // in document order of their closing tags
	Roots       []string        // handles of the top-level components
}

// Load reads and parses the bog file at path. The error is a *ReadError for an unreadable input.
func Load(path string) (*Bog, error) {
	text, err := ReadText(path)
	if err != nil {
		return nil, err
	}
	return Parse(path, text), nil
}

// Resolve finds a component by "h:<handle>", by exact path (leading/trailing '/' ignored, escaped
// or decoded) or by an unambiguous leaf name or path suffix; nil when none or ambiguous.
func (b *Bog) Resolve(ref string) *Comp {
	if strings.HasPrefix(ref, "h:") {
		c, _ := b.Comps.Get(ref[2:])
		return c
	}
	ref = UnescapeName(strings.Trim(ref, "/"))
	for _, c := range b.Comps.All() {
		if c.Path == ref {
			return c
		}
	}
	var hit *Comp
	hits := 0
	for _, c := range b.Comps.All() {
		if strings.HasSuffix(c.Path, "/"+ref) || c.Name == ref {
			hit = c
			hits++
		}
	}
	if hits == 1 {
		return hit
	}
	return nil
}

// LinkRow renders l: an unresolved source falls back to its ord, then "h:<handle>", then "?";
// a missing slot name prints as "None", as the Python f-string does.
func (b *Bog) LinkRow(l *Link) LinkRow {
	var src *Comp
	if l.SrcH != nil {
		src, _ = b.Comps.Get(*l.SrcH)
	}
	var srcPath string
	switch {
	case src != nil:
		srcPath = src.Path
	case l.SrcOrd != nil && *l.SrcOrd != "":
		srcPath = *l.SrcOrd
	case l.SrcH != nil && *l.SrcH != "":
		srcPath = "h:" + *l.SrcH
	default:
		srcPath = "?"
	}
	return LinkRow{
		Link:        l.LinkName,
		Source:      srcPath + "." + pycompat.OptStr(l.SrcSlot),
		Target:      l.ContainerPath + "." + pycompat.OptStr(l.TgtSlot),
		SrcResolved: src != nil,
	}
}
