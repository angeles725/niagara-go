package modulefind

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/honest"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// The cmd_* functions of module-find.py. A search command returns its row count (0 means
// exit 1); the others return -1.

// scope is modulefind.sources.scope: java files read, plus the unreadable ones.
func scope(m *module, where string) string {
	if len(m.unreadable) > 0 {
		return honest.Scope(where, honest.Count{Label: "java_files", N: m.files},
			honest.Count{Label: "unreadable", N: len(m.unreadable)})
	}
	return honest.Scope(where, honest.Count{Label: "java_files", N: m.files})
}

// flagsHave is _flags_have: the flags= expression names every wanted flag (each
// character of --flags: o/s/h/r/t, any other character upper-cased).
func flagsHave(declared, want string) bool {
	df := strings.ToUpper(declared)
	long := map[rune]string{'o': "OPERATOR", 's': "SUMMARY", 'h': "HIDDEN", 'r': "READONLY", 't': "TRANSIENT"}
	for _, w := range want {
		name, ok := long[w]
		if !ok {
			name = strings.ToUpper(string(w))
		}
		if !strings.Contains(df, name) {
			return false
		}
	}
	return true
}

type slotFilter struct {
	typ, flags       string
	hasType, hasFlag bool
	name             *pyre.Regexp
}

func cmdSlots(m *module, o *opts, f slotFilter) int {
	var rows []row
	for _, cls := range m.slots.sortedKeys() {
		slots, _ := m.slots.get(cls)
		for _, slot := range slots.keys {
			s := slots.m[slot]
			if f.hasType && f.typ != "" && !strings.Contains(s.typ, f.typ) {
				continue
			}
			if f.name != nil && !f.name.MatchString(slot) {
				continue
			}
			if f.hasFlag && f.flags != "" && !flagsHave(s.flags, f.flags) {
				continue
			}
			rows = append(rows, row{{"class", cls}, {"slot", slot}, {"type", s.typ}, {"flags", s.flags},
				{"min", s.min}, {"complex", complexType.MatchString(s.typ)}})
		}
	}
	return render(rows, o, func(r row) string {
		line := fmt.Sprintf("%s.%s  <%s>  flags=%s", r.str("class"), r.str("slot"), r.str("type"), orDash(r.str("flags")))
		if v := r.get("min").(*float64); v != nil {
			line += "  min=" + pycompat.FloatRepr(*v)
		}
		if r.get("complex").(bool) {
			line += "  [COMPLEX]"
		}
		return line
	}, "(no matching @NiagaraProperty)")
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cmdActions(m *module, o *opts, name *pyre.Regexp) int {
	var rows []row
	for _, cls := range m.actions.sortedKeys() {
		acts, _ := m.actions.get(cls)
		for _, a := range acts.keys {
			if name != nil && !name.MatchString(a) {
				continue
			}
			rows = append(rows, row{{"class", cls}, {"action", a}, {"flags", acts.m[a]}})
		}
	}
	return render(rows, o, func(r row) string {
		return fmt.Sprintf("%s.%s()  f=%s", r.str("class"), r.str("action"), r.str("flags"))
	}, "(no matching @NiagaraAction)")
}

func cmdWriters(m *module, o *opts, slot string) {
	w := sortedSet(m.writers[slot])
	all := map[string]bool{}
	for _, s := range m.dynWriters {
		for x := range s {
			all[x] = true
		}
	}
	dyn := sortedSet(all)
	if o.json {
		fmt.Fprintln(o.out, jsonDumps(row{{"slot", slot}, {"writers", w}, {"dynamic", dyn}}, 0))
		return
	}
	for _, line := range w {
		fmt.Fprintf(o.out, "  %s\n", line)
	}
	if len(w) == 0 {
		head, rest := "", ""
		if rs := []rune(slot); len(rs) > 0 {
			head, rest = strings.ToUpper(string(rs[0])), string(rs[1:])
		}
		fmt.Fprintf(o.out, "%s: no STATIC writer (.set(\"%s\",...) or set%s%s(...))\n", slot, slot, head, rest)
	}
	if len(dyn) > 0 {
		fmt.Fprintln(o.out, "  dynamic writers (slot chosen at runtime — any slot could be the target):")
		for _, line := range dyn {
			fmt.Fprintf(o.out, "    %s\n", line)
		}
	}
}

func cmdExtends(m *module, o *opts, of string, hasOf bool) {
	if hasOf && of != "" {
		chain := m.resolveChain(of)
		if o.json {
			fmt.Fprintln(o.out, jsonDumps(row{{"class", of}, {"chain", chain}}, 0))
			return
		}
		fmt.Fprintln(o.out, strings.Join(chain, " -> "))
		return
	}
	if o.json {
		r := row{}
		for _, cls := range m.extends.keys {
			r = append(r, field{cls, m.extends.m[cls]})
		}
		fmt.Fprintln(o.out, jsonDumps(r, 0))
		return
	}
	for _, cls := range m.extends.sortedKeys() {
		fmt.Fprintf(o.out, "%s -> %s\n", cls, m.extends.m[cls])
	}
}

func cmdOrds(m *module, o *opts, name *pyre.Regexp) int {
	seen := map[[2]string]bool{}
	var rows []row
	for _, x := range m.ords {
		if (name != nil && !name.MatchString(x[1])) || seen[x] {
			continue
		}
		seen[x] = true
		rows = append(rows, row{{"class", x[0]}, {"ord", x[1]}})
	}
	return render(rows, o, func(r row) string { return r.str("class") + "  " + r.str("ord") }, "(no ORD literals)")
}

func cmdGrep(m *module, o *opts, rx *pyre.Regexp, source bool) int {
	var rows []row
	if source { // real content search: every line of every .java file
		for _, l := range javaLines(m.root, m.unreadable) {
			if rx.MatchString(l.text) {
				rel, err := filepath.Rel(m.root, l.path)
				if err != nil {
					rel = l.path
				}
				rows = append(rows, row{{"file", rel}, {"line", l.n}, {"text", truncate(pycompat.Strip(l.text), 120)}})
			}
		}
		o.scope = scope(m, "source lines in "+m.root)
		return render(rows, o, func(r row) string {
			return fmt.Sprintf("%s:%d  %s", r.str("file"), r.get("line").(int), r.str("text"))
		}, "(no matches)")
	}
	for _, cls := range m.slots.keys {
		slots, _ := m.slots.get(cls)
		for _, slot := range slots.keys {
			if rx.MatchString(slot) {
				rows = append(rows, row{{"kind", "slot"}, {"class", cls}, {"name", slot}})
			}
		}
	}
	for _, cls := range m.actions.keys {
		acts, _ := m.actions.get(cls)
		for _, a := range acts.keys {
			if rx.MatchString(a) {
				rows = append(rows, row{{"kind", "action"}, {"class", cls}, {"name", a}})
			}
		}
	}
	classes := map[string]bool{}
	for _, cls := range m.extends.keys {
		classes[cls], classes[m.extends.m[cls]] = true, true
	}
	for _, cls := range sortedSet(classes) { // sorted: set order is hash-seed dependent
		if rx.MatchString(cls) {
			rows = append(rows, row{{"kind", "class"}, {"class", cls}, {"name", m.extends.m[cls]}})
		}
	}
	o.scope = scope(m, "class/slot/action names in "+m.root+" (grep --source searches the code)")
	return render(rows, o, func(r row) string {
		return fmt.Sprintf("%s %s  %s", padRight(r.str("kind"), 7), r.str("class"), r.str("name"))
	}, "(no matches)")
}

func cmdCallers(m *module, o *opts, method string) int {
	rx := pyre.MustCompile(`\b`+escape(method)+`\s*\(`, 0)
	var rows []row
	for _, l := range javaLines(m.root, m.unreadable) {
		if rx.MatchString(l.text) {
			base := filepath.Base(l.path)
			rows = append(rows, row{{"class", base[:len(base)-5]}, {"line", l.n},
				{"text", truncate(pycompat.Strip(l.text), 90)}})
		}
	}
	o.scope = scope(m, m.root)
	return render(rows, o, func(r row) string {
		return fmt.Sprintf("%s:%d  %s", r.str("class"), r.get("line").(int), r.str("text"))
	}, "(no callers of "+method+")")
}
