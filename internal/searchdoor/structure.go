package searchdoor

import (
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// Structural questions -> module_nav's structure commands, a lead sub-scope of
// CODE (searchdoor/structure.py). A text index cannot answer "all subclasses
// of BDeviceNetwork": the answer is a walk of the class graph.
//
//	subclasses / children / descendants / hierarchy of <Class>  module_nav hierarchy <Class> --depth 3
//	implementors / implementations of <Interface>               module_nav implementors <Interface> -n 200
//	list / which / available ... drivers [<module>-rt]          module_nav drivers [<module>]
//
// The rows are an ENUMERATION (tree or table order), not a ranking.

const (
	structRows     = 60 // enumerated rows shown (and counted by the golden runner)
	hierarchyDepth = 3
	structureTitle = "structure"
)

const (
	className = `(?P<name>(?-i:[A-Z])\w*)` // case-sensitive capital even under re.I
	hierWords = `sub-?class(?:es)?|subtypes?|children|child classes|descendants?|hierarchy|` +
		`derived (?:classes|types)|classes (?:that |which )?(?:extend|inherit)s?(?: from)?|extends|` +
		`subclases?|clases hijas|hijas|jerarqu[ií]a|descendientes|clases que (?:extienden|heredan) de|` +
		`heredan de`
	implWords = `implementors?|implementers?|implementations?|classes implementing|who implements|` +
		`implementing|implementadores|implementaciones|clases que implementan|implementan`
	driverLead  = `^[\s¿¡"'(]*(?:(?:can|could|would)\s+you\s+|please\s+|por\s+favor\s+|(?:puedes|podr[ií]as)\s+)?`
	driverVerb  = `(?:list|show|give|enumerate|lista|listar|enumera|dame|muestra|mu[eé]strame)(?:me)?\s+(?:me\s+)?` + `(?:(?:the|a|la|una)\s+)?(?:(?:list|lista)\s+)?(?:(?:of|de)\s+)?`
	driverQuant = `(?:all|every|which|what|todos|cu[aá]les|qu[eé])\s+(?:(?:the|los|son|de)\s+)*`
	driverArt   = `(?:the|los|las)\s+`
	driverAvail = `(?:available|supported|installed|disponibles|soportados|instalados)`
)

var (
	hierarchyRx = pyre.MustCompile(`\b(?:`+hierWords+`)\b(?:\s+(?:of|from|de|del|la clase))*\s+`+className, pyre.IGNORECASE)
	implRx      = pyre.MustCompile(`\b(?:`+implWords+`)\b(?:\s+(?:of|de|la interfaz|interface))*\s+`+className, pyre.IGNORECASE)
	driversRx   = pyre.MustCompile(driverLead+`(?:`+driverVerb+`(?:`+driverQuant+`)?(?:`+driverArt+`)?(?:`+driverAvail+`\s+)?|`+
		driverQuant+`(?:`+driverAvail+`\s+)?`+`|(?:`+driverArt+`)?`+driverAvail+`\s+|(?:`+driverArt+`(?=drivers\s+`+driverAvail+`\b)))?drivers\b`+
		`|\bdrivers?\s+anatomy\b|\banatomy\s+of\s+(?:the\s+|an?\s+)?(?:[\w-]+\s+)?drivers?\b`+
		`|\banatom[ií]a\s+(?:de\s+(?:los\s+|un\s+)?|del\s+)(?:[\w-]+\s+)?drivers?\b`, pyre.IGNORECASE)
	moduleRx   = re(`\b([a-z][\w]*-(?:rt|wb|ux|se))\b`)
	notFoundRx = pyre.MustCompile(`^\s*(?:Class|Interface) '[^']+' not found`, pyre.MULTILINE)
	treeRow    = re("^[\\s|`-]*?([A-Za-z_$][\\w$.]*)\\s+\\(([^)]+)\\)\\s*$")
	implRow    = re(`^\s*([A-Za-z_$][\w$]*)\s+(\S+)\s+(\S+)\s*$`)
	hiddenRx   = re(`\.\.\. (\d+) descendants`)
)

// detectStructure is structure.detect: (command argv tail, label) for a
// structural question, or nil.
func detectStructure(query string) ([]string, string) {
	if m := implRx.Search(query); m != nil {
		name, _ := m.Named("name")
		return []string{"implementors", name, "-n", "200"}, "implementors of " + name
	}
	if m := hierarchyRx.Search(query); m != nil {
		name, _ := m.Named("name")
		return []string{"hierarchy", name, "--depth", strconv.Itoa(hierarchyDepth)}, "subclasses of " + name
	}
	if driversRx.Search(query) != nil {
		if m := moduleRx.Search(query); m != nil {
			return []string{"drivers", m.Group(1)}, "driver anatomy of " + m.Group(1)
		}
		return []string{"drivers"}, "driver anatomy"
	}
	return nil, ""
}

// treeRows reads module_nav hierarchy: the class rows (root excluded) in tree
// order, plus its count lines.
func treeRows(out, root string) ([]string, []Item) {
	var scope []string
	for _, ln := range nonblank(out) {
		if s := pycompat.Strip(ln); strings.HasPrefix(s, "Hierarchy:") || strings.HasPrefix(s, "Depth limit") {
			scope = append(scope, s)
		}
	}
	var items []Item
	for _, ln := range lines(out) {
		if m := match(treeRow, ln); m != nil && m.Group(1) != root {
			items = append(items, keyed(m.Group(1), m.Group(1)+"  ["+m.Group(2)+"]", ""))
		}
	}
	hidden := 0
	for _, n := range hiddenRx.FindAll(out) {
		v, _ := pycompat.ParseInt(n)
		hidden += int(v)
	}
	if hidden != 0 {
		scope = append(scope, strconv.Itoa(hidden)+" deeper descendants not expanded (module_nav hierarchy --depth "+
			strconv.Itoa(hierarchyDepth+1)+"+)")
	}
	return scope, items
}

// implRows reads module_nav implementors: 'CLASS MODULE PACKAGE' rows after
// the dashed rule, plus the count line.
func implRows(out string) ([]string, []Item) {
	ls := lines(out)
	var scope []string
	rule := -1
	for i, ln := range ls {
		s := pycompat.Strip(ln)
		if strings.HasPrefix(s, "Implementors of") {
			scope = append(scope, s)
		}
		if rule < 0 && s != "" && strings.Trim(s, "-") == "" {
			rule = i
		}
	}
	var items []Item
	if rule >= 0 {
		for _, ln := range ls[rule+1:] {
			if m := match(implRow, ln); m != nil {
				items = append(items, keyed(m.Group(1), m.Group(1)+"  ["+m.Group(2)+"]  "+m.Group(3), ""))
			}
		}
	}
	return scope, items
}

// structureJob is structure.job: the structure sub-scope for a structural
// question (nil when it is not one), inside the CODE source's deadline.
func (d *door) structureJob(path, query string, deadline float64) *Outcome {
	argv, label := detectStructure(query)
	if argv == nil {
		return nil
	}
	command := "module_nav " + strings.Join(argv, " ")
	title := structureTitle + " · " + label
	left := deadline - d.now()
	if left <= 0 {
		return unavailable("code", title, command, "unavailable: timeout, the CODE budget is spent")
	}
	if argv[0] == "drivers" && !d.hasCommand(path, "drivers", left) {
		o := newOutcome("code", title, command, Empty)
		o.Notes = append(o.Notes, "this module_nav has no `drivers` command yet (`drivers --help` is not exit 0); "+
			"structure skipped -- try `hierarchy BDeviceNetwork` instead")
		return o
	}
	left = deadline - d.now()
	if left <= 0 { // the `drivers --help` probe used the rest of the budget
		return unavailable("code", title, command,
			"unavailable: timeout, the CODE budget was spent before the structure command ran")
	}
	raw := d.backend.Run(codeSource, append([]string{path}, argv...), left)
	if raw.HasRC && (raw.RC == 0 || raw.RC == 1) && notFoundRx.MatchString(raw.Out) {
		o := newOutcome("code", title, command, Empty)
		o.MS = raw.MS
		for _, ln := range nonblank(raw.Out) {
			if s := pycompat.Strip(ln); !strings.HasPrefix(s, "Loading ") && len(o.Scope) < 3 {
				o.Scope = append(o.Scope, s)
			}
		}
		return o
	}
	o := newOutcome("code", title, command, raw.State)
	o.Reason, o.MS = raw.Reason, raw.MS
	switch raw.State {
	case OK:
		switch argv[0] {
		case "hierarchy":
			o.Scope, o.Items = treeRows(raw.Out, argv[1])
		case "implementors":
			o.Scope, o.Items = implRows(raw.Out)
		default:
			o.Scope, o.Items = generic(raw.Out)
		}
		if len(o.Items) == 0 {
			o.State = Empty
		}
	case Empty:
		o.Scope = strippedNonblank(raw.Out, 3)
	}
	return o
}

// hasCommand reports whether this module_nav knows command (its --help exits 0).
func (d *door) hasCommand(path, command string, timeout float64) bool {
	raw := d.backend.Run(codeSource, []string{path, command, "--help"}, timeout)
	return raw.HasRC && raw.RC == 0
}
