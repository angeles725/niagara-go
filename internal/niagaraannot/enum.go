package niagaraannot

import (
	"fmt"
	"strings"
	"unicode"
)

// Constant is one generated enum constant: the int constant's name, its ordinal and the tag.
type Constant struct {
	Name    string // DIRECT
	Ordinal int
	Tag     string // direct
}

// Constants are the int constants Slot-o-Matic generates for the range: ConstName of each tag,
// with its ordinal, or the previous ordinal plus 1 (0 for the first) when the @Range omits it
// (Range.ordinal() defaults to -1, but an explicit -1 is generated as -1).
// Two tags with the same ordinal, name or tag are an error.
func (e *Enum) Constants() ([]Constant, error) {
	var out []Constant
	next := 0
	ords, names, tags := map[int]string{}, map[string]bool{}, map[string]bool{}
	for _, r := range e.Range {
		o := r.Ordinal
		if !r.HasOrdinal {
			o = next
		} else if o == unresolved {
			return nil, fmt.Errorf("line %d: ordinal of %s is not an int literal: %s", e.Line, r.Tag, r.OrdinalText)
		}
		next = o + 1
		c := Constant{Name: ConstName(r.Tag), Ordinal: o, Tag: r.Tag}
		if prev, dup := ords[o]; dup {
			return nil, fmt.Errorf("line %d: tags %s and %s share ordinal %d", e.Line, prev, r.Tag, o)
		}
		if names[c.Name] || tags[r.Tag] {
			return nil, fmt.Errorf("line %d: tag %s (%s) declared twice", e.Line, r.Tag, c.Name)
		}
		ords[o], names[c.Name], tags[r.Tag] = r.Tag, true, true
		out = append(out, c)
	}
	return out, nil
}

// DefaultTag is the tag DEFAULT is set to: defaultValue, else the first tag of the range. A
// defaultValue that is not a tag of the range is an error.
func (e *Enum) DefaultTag() (string, error) {
	if e.Default == "" {
		if len(e.Range) == 0 {
			return "", fmt.Errorf("line %d: empty range", e.Line)
		}
		return e.Range[0].Tag, nil
	}
	for _, r := range e.Range {
		if r.Tag == e.Default {
			return r.Tag, nil
		}
	}
	return "", fmt.Errorf("line %d: defaultValue %s is not a tag of the range", e.Line, e.Default)
}

// ConstName is the int constant name of a tag: an underscore before each upper-case letter or
// digit that follows a lower-case letter, then all upper case (toActive → TO_ACTIVE,
// stopBits2 → STOP_BITS_2, io16V1 → IO_16V1, ScptLimitCO2 → SCPT_LIMIT_CO2; checked against the
// generated code of the 204 enums of the 4.15.6.20 source tree).
func ConstName(tag string) string {
	var b strings.Builder
	var prev rune
	for i, r := range tag {
		if i > 0 && unicode.IsLower(prev) && (unicode.IsUpper(r) || unicode.IsDigit(r)) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToUpper(r))
		prev = r
	}
	return b.String()
}
