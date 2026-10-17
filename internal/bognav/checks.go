package bognav

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/textfold"
)

// Kit audit views over a bog: writable, relays (CHECK11), hoa (CHECK8) and tiles (CHECK18)
// (bognav/checks.py).

func (e *env) moduleMiss(c *bog.Comp, module string) bool {
	return module != "" && c.Pfx != module && c.Module != module
}

func cmdWritable(e *env) (int, int) {
	src := sourceTypes(e.str("src"))
	module, klass := e.str("module"), e.str("klass")
	var rows []row
	for _, c := range e.bog.Comps.All() {
		if e.moduleMiss(c, module) {
			continue
		}
		for n, s := range c.Slots.All() {
			k, note := slotWritability(s, n, src)
			if klass != "" && k != klass {
				continue
			}
			rows = append(rows, row{{"path", c.Path}, {"slot", n}, {"type", typeDisplay(s.Type, n, src)},
				{"value", s.Value}, {"class", k}, {"note", note}})
		}
	}
	return e.render(rows, func(r row) string {
		line := ljust(r.s("class"), 8) + " " + r.s("path") + "." + r.s("slot")
		if v := r.get("value").(*string); v != nil {
			line += "=" + *v
		}
		return line + "  <" + r.s("type") + ">  " + r.s("note")
	}, "(no slots matched)", nil), -1
}

func (e *env) child(c *bog.Comp, pred func(*bog.Comp) bool) *bog.Comp {
	for _, h := range c.Children {
		if cc := e.comp(h); pred(cc) {
			return cc
		}
	}
	return nil
}

// slotOrDefault is _slot_or_default: the bog's own value ("bog"), else the type default from
// organized/ ("default"), else ("", why) — never a hardcoded guess.
func (e *env) slotOrDefault(c *bog.Comp, slot string) (string, string, bool) {
	if s, ok := c.Slots.Get(slot); ok && s.Value != nil {
		return *s.Value, "bog", true
	}
	if catalog() == nil {
		return "", "no organized/ catalog", false
	}
	key, props := componentDefaults(e.bog, c)
	if key == "" {
		return "", "type " + c.Type + " not in organized/", false
	}
	d, ok := props.Get(slot)
	if !ok {
		return "", slot + " not declared by " + simpleName(key) + " or its parents", false
	}
	return defaultLiteral(d.Default), "default", true
}

// writeOnUp is write_on_up: the writeOnUp of the tuning policy a proxy point uses
// (proxyExt.tuningPolicyName names a child of the nearest ancestor's tuningPolicies).
func (e *env) writeOnUp(point *bog.Comp) string {
	ext := e.child(point, func(c *bog.Comp) bool { return strings.HasSuffix(c.SimpleType(), "ProxyExt") })
	if ext == nil {
		return "n/a (no proxy ext)"
	}
	name, how, ok := e.slotOrDefault(ext, "tuningPolicyName")
	if !ok {
		return "? (tuningPolicyName: " + how + ")"
	}
	for node := e.parentOf(point); node != nil; node = e.parentOf(node) {
		tps := e.child(node, func(c *bog.Comp) bool { return c.Name == "tuningPolicies" })
		if tps == nil {
			continue
		}
		policy := e.child(tps, func(c *bog.Comp) bool { return c.Name == name })
		if policy == nil {
			return fmt.Sprintf("? (no tuning policy %s under %s)", pycompat.StrRepr(name), tps.Path)
		}
		value, how, ok := e.slotOrDefault(policy, "writeOnUp")
		if !ok {
			return "? (" + how + ")"
		}
		if how == "bog" {
			return value
		}
		return value + " (default)"
	}
	return "? (no tuningPolicies above " + point.Path + ")"
}

func (e *env) parentOf(c *bog.Comp) *bog.Comp {
	if c.ParentH == nil {
		return nil
	}
	return e.comp(*c.ParentH)
}

func cmdRelays(e *env) (int, int) {
	module := e.str("module")
	var rows []row
	for _, lk := range e.bog.Links {
		src, cont := linkSource(e.bog, lk), linkContainer(e.bog, lk)
		if src == nil || cont == nil || !cont.IsWritable {
			continue
		}
		if module != "" && src.Module != module && src.Pfx != module {
			continue
		}
		if module == "" && !ownModules[src.Module] {
			continue
		}
		fb := "NO"
		if cont.HasFallback {
			fb = "yes"
		}
		rows = append(rows, row{{"target", cont.Path}, {"type", cont.SimpleType()},
			{"source", src.Path + "." + pyStr(lk.SrcSlot)}, {"fallback", fb}, {"writeOnUp", e.writeOnUp(cont)}})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ni, nj := rows[i].s("fallback") != "NO", rows[j].s("fallback") != "NO"
		if ni != nj {
			return !ni
		}
		return rows[i].s("target") < rows[j].s("target")
	})
	noFB := 0
	for _, r := range rows {
		if r.s("fallback") == "NO" {
			noFB++
		}
	}
	e.render(rows, func(r row) string {
		return ljust(r.s("fallback"), 3) + " fb  " + r.s("target") + "  <" + r.s("type") + ">  <- " + r.s("source") +
			"  writeOnUp=" + r.s("writeOnUp")
	}, "(no own-module relays)", nil)
	if e.isText() {
		e.println(fmt.Sprintf("# %d own-module->writable relay targets; %d WITHOUT a fallback "+
			"(hold last state on stop/reload — B810)", len(rows), noFB))
	}
	return 0, -1
}

var prioRE = regexp.MustCompile(`^in[1-8]$`)

func cmdHoa(e *env) (int, int) {
	module := e.str("module")
	var rows []row
	overrides, auto, config := 0, 0, 0
	for _, c := range e.bog.Comps.All() {
		if module != "" {
			if e.moduleMiss(c, module) {
				continue
			}
		} else if !ownModules[c.Module] && !e.flag("all") {
			continue
		}
		for n, s := range c.Slots.All() {
			isPrio := prioRE.MatchString(n)
			if !strings.HasSuffix(n, "Mode") && !isPrio {
				continue
			}
			v := ptr(s.Value)
			hasVal := false
			switch textfold.Lower(v) {
			case "", "false", "0", "null":
			default:
				hasVal = true
			}
			var state string
			switch {
			case isPrio && hasVal:
				state = "OVERRIDE"
				overrides++
			case hasVal:
				state = "config"
				config++
			default:
				state = "AUTO"
				auto++
			}
			value := "(auto/null)"
			if s.Value != nil {
				value = *s.Value
			}
			rows = append(rows, row{{"slot", c.Path + "." + n}, {"value", value}, {"state", state}})
		}
	}
	order := map[string]int{"OVERRIDE": 0, "config": 1, "AUTO": 2}
	sort.SliceStable(rows, func(i, j int) bool {
		if oi, oj := order[rows[i].s("state")], order[rows[j].s("state")]; oi != oj {
			return oi < oj
		}
		return rows[i].s("slot") < rows[j].s("slot")
	})
	e.render(rows, func(r row) string {
		return ljust(r.s("state"), 8) + " " + r.s("slot") + " = " + r.s("value")
	}, "(no HOA/mode slots)", nil)
	if e.isText() {
		e.println(fmt.Sprintf("# %d HOA/mode slots; %d AUTO/null; %d persisted config-mode; "+
			"%d active priority-array override(s) [CHECK8]", len(rows), auto, config, overrides))
	}
	return 0, -1
}

var (
	tileRE = regexp.MustCompile(`^evap(\p{Nd}+)(ValveState|FanState)$`)
	unitRE = regexp.MustCompile(`EvaporatorUnit_?(\p{Nd}+)?$`)
)

func cmdTiles(e *env) (int, int) {
	var rooms []*bog.Comp
	for _, c := range e.compsByPath() {
		if c.SimpleType() == "RoomPanel" {
			rooms = append(rooms, c)
		}
	}
	var rows []row
	for _, room := range rooms {
		for _, lk := range e.bog.Links {
			if lk.ContainerH == nil || *lk.ContainerH != room.Handle {
				continue
			}
			m := tileRE.FindStringSubmatch(ptr(lk.TgtSlot))
			if m == nil {
				continue
			}
			src := linkSource(e.bog, lk)
			srcPath, unitName := "", "?"
			if src != nil {
				srcPath, unitName = src.Path, lastPart(src.Path, "/")
			}
			unit := "1"
			if um := unitRE.FindStringSubmatch(srcPath); um != nil && um[1] != "" {
				unit = um[1]
			}
			crossed := "ok"
			if m[1] != unit {
				crossed = "CROSSED"
			}
			rows = append(rows, row{{"room", room.Name}, {"tile", "evap" + m[1] + m[2]}, {"unit", unitName}, {"crossed", crossed}})
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if a, b := rows[i].s("room"), rows[j].s("room"); a != b {
			return a < b
		}
		return rows[i].s("tile") < rows[j].s("tile")
	})
	e.render(rows, func(r row) string {
		return ljust(r.s("crossed"), 7) + " " + r.s("room") + "." + r.s("tile") + "  <- " + r.s("unit")
	}, "(no RoomPanel tiles)", nil)
	if e.isText() {
		crossed := 0
		for _, r := range rows {
			if r.s("crossed") == "CROSSED" {
				crossed++
			}
		}
		e.println(fmt.Sprintf("# %d tile links; %d CROSSED (tile number != unit number)", len(rows), crossed))
	}
	return 0, -1
}
