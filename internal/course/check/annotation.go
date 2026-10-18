package check

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/niagaraannot"
)

// The annotation kind: what the Slot-o-Matic annotations of one Java source declare
// (internal/niagaraannot). The sources are the participant's lab modules (--labsrc,
// COURSE_LAB_SOURCES: the Niagara dev home that holds training/ and trafficLightDriver/, each laid
// out by the New Module Wizard as <module>/<module>-rt/src/<package path>/<Class>.java); Glob is
// one file under it. Only the annotations are read, never the generated region (a file may not
// have one yet), so the check decides from what the author declared what Slot-o-Matic generates
// (enum constants, slots) and what the AX Slot Sheet shows (flag letters).

// Annotation facts: what a check reads from the annotations, rendered as Want spells it.
const (
	// factFlags: each declared slot as name=letters, the Slot Sheet letters of its flags
	// (Flags.encodeToString), "-" for none: "out=rs in1=s in3=-".
	factFlags = "flags"
	// factEnum: the generated int constants and the DEFAULT tag: "DIRECT=0 REVERSE=1 DEFAULT=direct".
	factEnum = "enum"
	// factSlots: each declared slot: "property name: Type", "action name(Param = default)" (the
	// parameter and default only when declared: "action name(= default)" without a parameter type),
	// "topic name(EventType)", joined with ", ".
	factSlots = "slots"
	// factFacets: each declared slot with facets as name=facet;facet.
	factFacets = "facets"
	// factDefaults: each declared property as "name: Type = default" (a missing or empty
	// defaultValue as ""), joined with ", ".
	factDefaults = "defaults"
	// factExtends: the superclass of the class declaration after the annotations, as written
	// (BAbstractService, BList<BObject>): the one fact read past the annotations.
	factExtends = "extends"
)

var annotationFacts = []string{factFlags, factEnum, factSlots, factFacets, factDefaults, factExtends}

// NeedsLabSources reports whether a check reads the lab module sources (an annotation row without
// a Root, or any row whose Root is the lab sources), so a summary names it.
func NeedsLabSources(checks []*Check) bool {
	return slices.ContainsFunc(checks, func(ck *Check) bool {
		return (ck.Kind == kindAnnotation && ck.Root == rootStation) || ck.Root == rootLabsrc
	})
}

// checkAnnotation renders Fact from the annotations of the file Glob of the lab module sources
// (Slot, when set, limits it to that slot) and compares it with Want. Without a tree, with a tree
// that is not a readable directory, or with annotations the parser cannot decide, the check is
// SKIP; a missing file, a missing slot or a different fact is FAIL.
func (s *Station) checkAnnotation(ck *Check) (string, string) {
	src := *ck
	if src.Root == rootStation {
		src.Root = rootLabsrc // an annotation row reads the lab sources unless it names a root
	}
	dir, skip := s.rootDir(&src)
	if skip != "" {
		return Skip, skip
	}
	text, err := readText(s.rootPath(&src, ck.Glob))
	if os.IsNotExist(err) {
		return Fail, "missing " + ck.Glob + " under " + dir
	}
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s: %v", ck.Glob, err)
	}
	ty, err := niagaraannot.Parse(text)
	if err != nil {
		return Skip, fmt.Sprintf("cannot read the annotations of %s: %v", ck.Glob, err)
	}
	slots := ty.Slots
	if ck.Slot != "" {
		sl := ty.Slot(ck.Slot)
		if sl == nil {
			return Fail, fmt.Sprintf("%s: %s declares no slot %s", ck.Glob, ty.Name, ck.Slot)
		}
		slots = []niagaraannot.Slot{*sl}
	}
	got, err := annotationFact(ck.Fact, ty, slots, text)
	if err != nil {
		return Skip, fmt.Sprintf("%s: %s %s not decided: %v", ck.Glob, ty.Name, ck.Fact, err)
	}
	ev := fmt.Sprintf("%s: %s %s %s", ck.Glob, ty.Name, ck.Fact, got)
	if got != ck.Want {
		return Fail, ev + ", want " + ck.Want
	}
	return Pass, ev
}

// annotationFact renders one fact of a parsed type over the given slots (text is the source, for
// the extends fact).
func annotationFact(fact string, ty *niagaraannot.Type, slots []niagaraannot.Slot, text string) (string, error) {
	var parts []string
	switch fact {
	case factExtends:
		return superclass(ty, text)
	case factDefaults:
		for _, sl := range slots {
			if sl.Kind == niagaraannot.Property {
				d := sl.Default
				if d == "" {
					d = `""`
				}
				parts = append(parts, sl.Name+": "+sl.Type+" = "+d)
			}
		}
		return strings.Join(parts, ", "), nil
	case factFlags:
		for _, sl := range slots {
			l, err := niagaraannot.FlagLetters(sl.Flags)
			if err != nil {
				return "", fmt.Errorf("%s: %v", sl.Name, err)
			}
			if l == "" {
				l = "-"
			}
			parts = append(parts, sl.Name+"="+l)
		}
		return strings.Join(parts, " "), nil
	case factSlots:
		for _, sl := range slots {
			parts = append(parts, slotText(sl))
		}
		return strings.Join(parts, ", "), nil
	case factFacets:
		for _, sl := range slots {
			if len(sl.Facets) > 0 {
				parts = append(parts, sl.Name+"="+strings.Join(sl.Facets, ";"))
			}
		}
		return strings.Join(parts, " "), nil
	case factEnum:
		if ty.Enum == nil {
			return "", fmt.Errorf("no @NiagaraEnum")
		}
		cs, err := ty.Enum.Constants()
		if err != nil {
			return "", err
		}
		d, err := ty.Enum.DefaultTag()
		if err != nil {
			return "", err
		}
		for _, c := range cs {
			parts = append(parts, fmt.Sprintf("%s=%d", c.Name, c.Ordinal))
		}
		return strings.Join(append(parts, "DEFAULT="+d), " "), nil
	}
	return "", fmt.Errorf("unknown fact %q", fact)
}

// slotText is one slot as the slots fact spells it.
func slotText(sl niagaraannot.Slot) string {
	switch sl.Kind {
	case niagaraannot.Property:
		return "property " + sl.Name + ": " + sl.Type
	case niagaraannot.Action:
		param := sl.Type
		if sl.Default != "" {
			param = strings.TrimSpace(param + " = " + sl.Default)
		}
		out := "action " + sl.Name + "(" + param + ")"
		if sl.ReturnType != "" {
			out += ": " + sl.ReturnType
		}
		return out
	}
	return "topic " + sl.Name + "(" + sl.Type + ")"
}

// superclass reads the extends clause of the class declaration that starts on the type's line: the
// type name and its type arguments as written, without spaces.
func superclass(ty *niagaraannot.Type, text string) (string, error) {
	lines := strings.Split(text, "\n")
	if ty.Line < 1 || ty.Line > len(lines) {
		return "", fmt.Errorf("no class declaration at line %d", ty.Line)
	}
	decl := strings.Join(lines[ty.Line-1:], "\n")
	if i := strings.IndexByte(decl, '{'); i >= 0 {
		decl = decl[:i]
	}
	m := extendsRe.FindStringSubmatch(decl)
	if m == nil {
		return "", fmt.Errorf("no extends clause")
	}
	return strings.Join(strings.Fields(m[1]), ""), nil
}

var extendsRe = regexp.MustCompile(`\bextends\s+([A-Za-z_][\w.]*(?:\s*<[^{]*?>)?)(?:\s+implements\b|\s*$)`)
