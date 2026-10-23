package bognav

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
)

// Link commands: links (--to/--from/--slot/--dangling), handle and path (bognav/links.py).

func ptr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func linkRowJSON(r bog.LinkRow) row {
	return row{{"link", r.Link}, {"source", r.Source}, {"target", r.Target}, {"src_resolved", r.SrcResolved}}
}

func cmdLinks(e *env) (int, int) {
	var dangling map[string]string // slot names declared by @NiagaraProperty under --src (CHECK7)
	if src := e.str("src"); src != "" {
		dangling = sourceTypes(src)
	}
	to, from := e.str("to"), e.str("from_")
	var rows []row
	for _, lk := range e.bog.Links {
		r := linkRowJSON(e.bog.LinkRow(lk))
		if to != "" && !matchRef(lk.ContainerH, lk.ContainerPath, lk.TgtSlot, to) {
			continue
		}
		if !slotFilter(lk, e.str("slot"), from, to, e.flag("slot_any")) {
			continue
		}
		if from != "" {
			srcPath := ""
			if src := linkSource(e.bog, lk); src != nil {
				srcPath = src.Path
			}
			if !matchRef(lk.SrcH, srcPath, lk.SrcSlot, from) {
				continue
			}
		}
		// CHECK7: a link whose targetSlotName is no source slot name, restricted to own-module
		// target containers (a kitControl/driver target's slots are not in the scanned source).
		isDangling := false
		if dangling != nil && ptr(lk.TgtSlot) != "" {
			if cont := linkContainer(e.bog, lk); cont != nil && ownModules[cont.Module] {
				_, declared := dangling[*lk.TgtSlot]
				isDangling = !declared
			}
		}
		if e.flag("dangling") && !isDangling {
			continue
		}
		r = append(r, kv{"dangling", isDangling})
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		// An empty result must not hide a typo: a --to/--from that names no component of the bog
		// is reported like `slot` reports an unknown path, not as a clean "(no matching links)".
		for _, ref := range []string{to, from} {
			if ref != "" && !refExists(e.bog, ref) {
				fmt.Fprintf(e.err, "bog-nav: cannot resolve %s\n", ref)
				return 0, cli.ExitNoMatch
			}
		}
	}
	return e.render(rows, func(r row) string {
		line := r.s("source") + "  -->  " + r.s("target")
		if !r.get("src_resolved").(bool) {
			line += "  [orphan sourceOrd]"
		}
		if r.get("dangling").(bool) {
			line += "  [DANGLING tgt]"
		}
		return line
	}, "(no matching links)", nil), -1
}

func linkSource(b *bog.Bog, lk *bog.Link) *bog.Comp {
	if lk.SrcH == nil {
		return nil
	}
	c, _ := b.Comps.Get(*lk.SrcH)
	return c
}

func linkContainer(b *bog.Bog, lk *bog.Link) *bog.Comp {
	if lk.ContainerH == nil {
		return nil
	}
	c, _ := b.Comps.Get(*lk.ContainerH)
	return c
}

// slotFilter is _slot_filter (endpoint-aware since 2026-09-06): with --from the slot names the
// SOURCE slot, with --to the TARGET slot, with neither, both, or --slot-any either end.
func slotFilter(lk *bog.Link, slot, from, to string, slotAny bool) bool {
	if slot == "" {
		return true
	}
	slot = bog.UnescapeName(slot) // link slot names are stored decoded; accept either spelling
	srcOK := lk.SrcSlot != nil && *lk.SrcSlot == slot
	tgtOK := lk.TgtSlot != nil && *lk.TgtSlot == slot
	if slotAny || (from == "") == (to == "") {
		return srcOK || tgtOK
	}
	if from != "" {
		return srcOK
	}
	return tgtOK
}

// matchPath is _match_path: the endpoint path is the needle or ends with /needle, or the needle
// occurs in "<path>.<slot>".
func matchPath(path string, slot *string, needle string) bool {
	needle = bog.UnescapeName(strings.Trim(needle, "/"))
	if path == needle || strings.HasSuffix(path, "/"+needle) {
		return true
	}
	return strings.Contains(path+"."+pyStr(slot), needle)
}

// matchRef is _match_ref: an h:xxxx needle matches the endpoint handle exactly, any other needle
// the path/slot (matchPath).
func matchRef(compH *string, path string, slot *string, needle string) bool {
	if strings.HasPrefix(needle, "h:") {
		return compH != nil && *compH == needle[2:]
	}
	return matchPath(path, slot, needle)
}

// refExists reports whether a --to/--from needle names something in the bog: an h:xxxx handle
// that exists, or a path that equals, ends, or occurs in some component path (optionally followed
// by ".<slot>", the matchPath form). It is consulted only when no link matched, so a needle that
// matches by slot name alone still filters as before.
func refExists(b *bog.Bog, needle string) bool {
	if strings.HasPrefix(needle, "h:") {
		_, ok := b.Comps.Get(needle[2:])
		return ok
	}
	n := bog.UnescapeName(strings.Trim(needle, "/"))
	candidates := []string{n}
	if i := strings.LastIndex(n, "."); i > 0 {
		candidates = append(candidates, n[:i])
	}
	for _, c := range b.Comps.All() {
		for _, cand := range candidates {
			if cand != "" && strings.Contains(c.Path, cand) {
				return true
			}
		}
	}
	return false
}

// compAt is _comp_at: the component at 'h:xxxx' or 'xxxx'; nil after printing the miss.
func (e *env) compAt(ref string) (string, *bog.Comp) {
	h := strings.TrimPrefix(ref, "h:")
	c := e.comp(h)
	if c == nil {
		e.println("h:", h, ": no such handle")
	}
	return h, c
}

func cmdHandle(e *env) (int, int) {
	h, c := e.compAt(e.str("handle"))
	if c == nil {
		return 0, cli.ExitNoMatch
	}
	var fedBy, feeds []any
	for _, lk := range e.bog.Links {
		if lk.ContainerH != nil && *lk.ContainerH == h {
			fedBy = append(fedBy, linkRowJSON(e.bog.LinkRow(lk)))
		}
	}
	for _, lk := range e.bog.Links {
		if lk.SrcH != nil && *lk.SrcH == h {
			feeds = append(feeds, linkRowJSON(e.bog.LinkRow(lk)))
		}
	}
	if e.flag("json") {
		e.printJSON(row{{"path", c.Path}, {"type", c.Type}, {"fed_by", orEmpty(fedBy)}, {"feeds", orEmpty(feeds)}}, 2)
		return 0, -1
	}
	e.println(fmt.Sprintf("h:%s  %s  <%s>  parent=h:%s", h, c.Path, c.Type, pyStr(c.ParentH)))
	for _, r := range fedBy {
		e.println("  fed-by   ", r.(row).s("source"), "  -->  ", r.(row).s("target"))
	}
	for _, r := range feeds {
		e.println("  feeds    ", r.(row).s("source"), "  -->  ", r.(row).s("target"))
	}
	return 0, -1
}

func orEmpty(v []any) []any {
	if v == nil {
		return []any{}
	}
	return v
}

func cmdPath(e *env) (int, int) {
	h, c := e.compAt(e.str("handle"))
	if c == nil {
		return 0, cli.ExitNoMatch
	}
	if e.flag("json") {
		e.printJSON(row{{"handle", h}, {"path", c.Path}, {"type", c.Type}}, -1)
		return 0, -1
	}
	e.println(c.Path)
	return 0, -1
}
