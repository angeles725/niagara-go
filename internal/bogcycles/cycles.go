// Package bogcycles is the Go port of niagara-research tools/bog-cycles.py: potential
// feedback loops (strongly connected components and self-loops) in the link graph of a
// station bog, read with the internal/bog model.
//
// Intended differences: none in output. A malformed document is rejected with expat's own
// message through internal/pyxml.
package bogcycles

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pyjson"
	"github.com/angeles725/niagara-go/internal/pyxml"
)

const description = "" +
	"bog-cycles: potential feedback loops in a station config.bog link graph (B1189-G3 / B1191-G3).\n" +
	"\n" +
	"N4 logic is event-driven (no execution order): a link cycle can oscillate or storm actions (B6/B538). This tool\n" +
	"reads the bog with the bog-nav model (tools/bognav: the same parser, SlotPath name decoding and parse cache),\n" +
	"builds the COMPONENT-level directed graph from the saved links (`b:Link` / `b:ConversionLink` are children of\n" +
	"the TARGET; `sourceOrd = h:<source handle>`) and reports strongly connected components of size > 1 and\n" +
	"self-loops. Each cycle lists the slot-level link edges that form it (`/A.out -> /B.in`): the links whose\n" +
	"source and target components are both in the cycle. It is a conservative CANDIDATE list for a human:\n" +
	"legitimate feedback (PID, hysteresis) also appears. Dangling `h:` references are reported (a link with an\n" +
	"unresolvable source creates no knob and no fault, B1115); a non-handle sourceOrd (`station:|slot:/...`)\n" +
	"is not an edge.\n" +
	"Usage: bogcycles <config.bog|backup.dist|file.xml> [--json]\n" +
	"Paths always start with `/` at the station root: `/Services/X`; an unnamed root (every station bog) is `/`,\n" +
	"a named root R renders as `/R` and its children as `/R/A`.\n" +
	"`links` counts the links whose source resolves (or is not a handle); a dangling link is counted in\n" +
	"`dangling` only. A missing slot name prints as `-` (null in --json).\n" +
	"`dangling` here is an unresolved `h:` SOURCE handle; bog-nav links' `dangling` is a different check\n" +
	"(a target slot no @NiagaraProperty declares, CHECK7) and bog-nav shows this case as `src_resolved: false`.\n" +
	"Exit 1 if cycles found, 0 otherwise, 2 on a missing/unreadable/non-BOG or not well-formed XML input or\n" +
	"bad usage. Read-only, stdlib.\n"

// Edge is one link inside a cycle; a nil slot is one the bog does not store.
type Edge struct {
	Source, Target         string
	SourceSlot, TargetSlot *string
	Link                   string
}

// Dangling is a link whose h: source handle resolves to no component. The JSON key "dangling" means
// this unresolved SOURCE; bog-nav links' "dangling" field is CHECK7 (an undeclared target slot).
type Dangling struct {
	Source, Target         string
	SourceSlot, TargetSlot *string
}

// Cycle is a strongly connected component of size > 1, or a self-loop.
type Cycle struct {
	Components []string // '/'-rooted paths, sorted
	Handles    []string // sorted
	Edges      []Edge
}

// Result is cycles_of(): the counts, the cycles (largest first) and the dangling links.
type Result struct {
	Links, Components int
	Cycles            []Cycle
	Dangling          []Dangling
}

func slash(path string) string { return "/" + path }

func set(s *string) bool { return s != nil && *s != "" }

// Find is cycles_of() for a parsed bog.
func Find(b *bog.Bog) Result {
	path := func(h string) string { c, _ := b.Comps.Get(h); return slash(c.Path) }
	edges := map[string]map[string]bool{}
	var links []*bog.Link
	var res Result
	for _, lk := range b.Links {
		if !set(lk.SrcH) || !set(lk.ContainerH) {
			continue // a non-handle sourceOrd, or a link outside any component: no graph edge
		}
		src, tgt := *lk.SrcH, *lk.ContainerH
		if _, ok := b.Comps.Get(src); !ok {
			target := tgt
			if _, ok := b.Comps.Get(tgt); ok {
				target = path(tgt)
			}
			res.Dangling = append(res.Dangling, Dangling{"h:" + src, target, lk.SrcSlot, lk.TgtSlot})
			continue
		}
		if edges[src] == nil {
			edges[src] = map[string]bool{}
		}
		edges[src][tgt] = true
		links = append(links, lk)
	}
	nodes := map[string]bool{}
	for v, ts := range edges {
		nodes[v] = true
		for t := range ts {
			nodes[t] = true
		}
	}
	for _, comp := range sccs(nodes, edges) {
		if len(comp) == 1 && !edges[comp[0]][comp[0]] {
			continue
		}
		members := map[string]bool{}
		for _, h := range comp {
			members[h] = true
		}
		var cyc Cycle
		for _, lk := range links {
			if members[*lk.SrcH] && members[*lk.ContainerH] {
				cyc.Edges = append(cyc.Edges, Edge{path(*lk.SrcH), path(*lk.ContainerH), lk.SrcSlot, lk.TgtSlot, lk.LinkName})
			}
		}
		sort.SliceStable(cyc.Edges, func(i, j int) bool { return edgeKey(cyc.Edges[i]) < edgeKey(cyc.Edges[j]) })
		for _, h := range comp {
			cyc.Components = append(cyc.Components, path(h))
		}
		sort.Strings(cyc.Components)
		cyc.Handles = append([]string(nil), comp...)
		sort.Strings(cyc.Handles)
		res.Cycles = append(res.Cycles, cyc)
	}
	sort.SliceStable(res.Cycles, func(i, j int) bool {
		a, c := res.Cycles[i].Components, res.Cycles[j].Components
		if len(a) != len(c) {
			return len(a) > len(c)
		}
		return lessStrings(a, c)
	})
	res.Links = len(b.Links) - len(res.Dangling)
	res.Components = b.Comps.Len()
	return res
}

// edgeKey is the sort key (source, sourceSlot or "", target, targetSlot or "", link) as one
// string: "\x00" sorts below every character a path or slot can hold.
func edgeKey(e Edge) string {
	return strings.Join([]string{e.Source, deref(e.SourceSlot), e.Target, deref(e.TargetSlot), e.Link}, "\x00")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func lessStrings(a, b []string) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func slotText(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

// Main is bog-cycles.py main().
func Main(args []string, s cli.Stdio) int {
	p := argparse.New(argparse.Config{Prog: "bogcycles", Description: description,
		Formatter: argparse.RawDescriptionHelpFormatter})
	p.Add(argparse.Arg{Help: "config.bog, a .dist station backup or a plain BOG file.xml"}, "bog")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "machine JSON output"}, "--json")
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	path := ns.String("bog")
	b, err := load(path)
	if err != nil {
		fmt.Fprintf(s.Err, "bog-cycles: %s\n", err)
		return cli.ExitCannotSearch
	}
	res := Find(b)
	if ns.Bool("json") {
		fmt.Fprintln(s.Out, pyjson.Dumps(res.json(), pyjson.Defaults(1)))
	} else {
		res.print(s.Out)
	}
	if len(res.Cycles) > 0 {
		return cli.ExitNoMatch
	}
	return cli.ExitFound
}

// load is Bog.read() followed by require_wellformed(): a *bog.ReadError, or the expat
// verdict "<path>: not well-formed XML (<expat message>)".
func load(path string) (*bog.Bog, error) {
	text, err := bog.ReadText(path)
	if err != nil {
		return nil, err
	}
	b := bog.Parse(path, text)
	if _, err := pyxml.Parse([]byte(text), pyxml.Mode{}); err != nil {
		return nil, fmt.Errorf("%s: not well-formed XML (%s)", path, err)
	}
	return b, nil
}

func (r Result) print(w io.Writer) {
	fmt.Fprintf(w, "components=%d links=%d cycles=%d dangling=%d\n", r.Components, r.Links, len(r.Cycles), len(r.Dangling))
	for _, c := range head(r.Cycles, 50) {
		more := ""
		if len(c.Components) > 8 {
			more = " ..."
		}
		fmt.Fprintf(w, "  CYCLE(%d): %s%s\n", len(c.Components), strings.Join(head(c.Components, 8), " <-> "), more)
		for _, e := range head(c.Edges, 12) {
			fmt.Fprintf(w, "      %s.%s -> %s.%s\n", e.Source, slotText(e.SourceSlot), e.Target, slotText(e.TargetSlot))
		}
		if len(c.Edges) > 12 {
			fmt.Fprintf(w, "      ... %d more edges (--json lists all)\n", len(c.Edges)-12)
		}
	}
	if len(r.Cycles) > 50 {
		fmt.Fprintf(w, "  ... %d more cycles (--json lists all)\n", len(r.Cycles)-50)
	}
	for _, d := range head(r.Dangling, 20) {
		fmt.Fprintf(w, "  DANGLING: %s -> %s.%s\n", d.Source, d.Target, slotText(d.TargetSlot))
	}
	if len(r.Dangling) > 20 {
		fmt.Fprintf(w, "  ... %d more dangling links (--json lists all)\n", len(r.Dangling)-20)
	}
}

func head[T any](s []T, n int) []T {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// object is a dict in insertion order from key, value pairs.
func object(kv ...any) pyjson.Value {
	o := pyjson.NewObject()
	for i := 0; i < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(pyjson.Value))
	}
	return o
}

func optStr(s *string) pyjson.Value {
	if s == nil {
		return pyjson.Value{}
	}
	return pyjson.String(*s)
}

func (r Result) json() pyjson.Value {
	cycles := pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{}}
	for _, c := range r.Cycles {
		edges := pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{}}
		for _, e := range c.Edges {
			edges.A = append(edges.A, object("source", pyjson.String(e.Source), "sourceSlot", optStr(e.SourceSlot),
				"target", pyjson.String(e.Target), "targetSlot", optStr(e.TargetSlot), "link", pyjson.String(e.Link)))
		}
		cycles.A = append(cycles.A, object("components", pyjson.StringList(c.Components), "handles", pyjson.StringList(c.Handles), "edges", edges))
	}
	dangling := pyjson.Value{Kind: pyjson.Array, A: []pyjson.Value{}}
	for _, d := range r.Dangling {
		dangling.A = append(dangling.A, object("source", pyjson.String(d.Source), "target", pyjson.String(d.Target),
			"sourceSlot", optStr(d.SourceSlot), "targetSlot", optStr(d.TargetSlot)))
	}
	return object("links", pyjson.IntValue(int64(r.Links)), "components", pyjson.IntValue(int64(r.Components)),
		"cycles", cycles, "dangling", dangling)
}
