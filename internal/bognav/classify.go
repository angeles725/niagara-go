package bognav

import (
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/slotomatic"
)

// External-writability classification of value slots (viewer record aa7054702 / B823 / B825)
// and slot types read from module source (bognav/classify.py).

// simpleWritable: a B-class/module simple form (…Numeric, b:Double) or a whole-string Java
// primitive (double, int); the primitives are anchored so NumericPoint never false-matches.
var simpleWritable = slotomatic.Compile(`(Boolean|Numeric|Float|Integer|Double|String|Enum|RelTime|AbsTime)$` +
	`|^(?:double|boolean|float|int|long|short|byte|char)$`)

// ownModules are the client modules whose links the kit checks audit.
var ownModules = map[string]bool{"ColdRoomPan": true, "DashboardPan": true, "CompPan": true}

var (
	propNameRE = slotomatic.Compile(`name\s*=\s*"([^"]+)"`)
	propTypeRE = slotomatic.Compile(`type\s*=\s*"([^"]+)"`)
)

// sourceTypes is source_types: best-effort {slotName: type} from the @NiagaraProperty
// annotations of a module Java source tree, for the bog value slots stored without a t= attr.
// The first declaration of a name wins, in os.walk order (dot directories skipped).
func sourceTypes(root string) map[string]string {
	m := map[string]string{}
	if root == "" || !pycompat.IsDir(root) {
		return m
	}
	pycompat.WalkWith(root, pycompat.WalkOptions{Unsorted: true}, func(dir string, dirs, files []string) []string {
		var kept []string
		for _, d := range dirs {
			if !strings.HasPrefix(d, ".") {
				kept = append(kept, d)
			}
		}
		for _, fn := range files {
			if !strings.HasSuffix(fn, ".java") {
				continue
			}
			text, err := slotomatic.ReadSource(slotomatic.Join(dir, fn))
			if err != nil {
				continue
			}
			lines := strings.Split(text, "\n")
			for i := 0; i < len(lines); {
				if !strings.Contains(lines[i], "@NiagaraProperty") {
					i++
					continue
				}
				buf := lines[i]
				depth := strings.Count(lines[i], "(") - strings.Count(lines[i], ")")
				j := i + 1
				for depth > 0 && j < len(lines) {
					buf += " " + lines[j]
					depth += strings.Count(lines[j], "(") - strings.Count(lines[j], ")")
					j++
				}
				i = j
				nm, tm := propNameRE.FindStringSubmatch(buf), propTypeRE.FindStringSubmatch(buf)
				if nm != nil && tm != nil {
					if _, ok := m[nm[1]]; !ok {
						m[nm[1]] = tm[1]
					}
				}
			}
		}
		return kept
	})
	return m
}

// typeDisplay is type_display: the slot type, filled from source when the bog stores none.
func typeDisplay(slotType, slotName string, src map[string]string) string {
	if slotType != "" {
		return slotType
	}
	if t, ok := src[slotName]; ok {
		return t + " (from source)"
	}
	return "frozen simple (type in source)"
}

// slotWritability is slot_writability: writability of a stored slot, its type filled from --src.
func slotWritability(s *bog.Slot, slotName string, src map[string]string) (string, string) {
	t := s.Type
	if t == "" {
		t = src[slotName]
	}
	return writability(t)
}

// childSuffix is child_suffix: "  child={value=3.0, ...}" for a composite slot's captured children.
func childSuffix(s *bog.Slot) string {
	if s.Child.Len() == 0 {
		return ""
	}
	var parts []string
	for k, v := range s.Child.All() {
		parts = append(parts, k+"="+v)
	}
	return "  child={" + strings.Join(parts, ", ") + "}"
}

// writability classifies a direct value slot's external write shape (B823 §823.2): (class, note).
func writability(slotType string) (string, string) {
	t := slotType
	if i := strings.LastIndexByte(t, ':'); i >= 0 {
		t = t[i+1:]
	}
	switch {
	case strings.HasPrefix(t, "Status"):
		// a complex property: writable via its child leaf (bare <real>), or the wrapped-obj
		// parent PUT (silent-zero hazard). [CERT-live] B826-G2 / viewer record.
		return "complex", "child-leaf ORD bare <real> preferred; parent wrapped-obj = silent-zero hazard"
	case t == "":
		return "bare", "no type attr (untyped value/mode slot)"
	case simpleWritable.MatchString(t):
		return "simple", `plain simple value; carries writable="true" externally`
	}
	return "other", t
}
