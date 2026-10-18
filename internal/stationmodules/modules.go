// Package stationmodules is the Go port of niagara-research tools/station-modules.py:
// resolve the module dependencies of a Niagara N4 station (check) or of an install
// (doctor) against a local install's modules/, and locate missing jars staged under sw/.
//
// Intended differences, none visible in the frozen cases: sw/ is walked in sorted order
// (Python: file system order, which decides between two exact-version staged copies);
// a module jar that cannot be read for any reason is skipped (Python crashes on a corrupt
// deflate stream or an encrypted member); a failed --fix copy prints
// "station-modules: <error>" and exits 1 where Python prints a traceback and exits 1.
package stationmodules

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// installCandidates is INSTALL_CANDIDATES, tried in order when --install is omitted.
var installCandidates = []string{
	"/mnt/c/Honeywell/OptimizerSupervisor-N4.14.0.162",
	"/home/cristian/Honeywell/OptimizerSupervisor-N4.14.0.162",
}

// exitError is sys.exit(code) after a message on stderr.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

func fail(code int, format string, a ...any) error {
	return &exitError{code, fmt.Sprintf(format, a...)}
}

func parser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: "station-modules.py",
		Description: "Resolve a station's module deps against a Niagara install."})
	sub := p.AddSubparsers(argparse.SubparsersConfig{Dest: "cmd", Required: true})
	c := sub.AddParser("check", argparse.SubparserConfig{Help: "station-driven dependency check"})
	c.Add(argparse.Arg{Help: "config.bog | .dist backup | file.xml"}, "target")
	d := sub.AddParser("doctor", argparse.SubparserConfig{Help: "install-integrity check (unresolved deps of installed parts)"})
	for _, q := range []*argparse.Parser{c, d} {
		q.Add(argparse.Arg{}, "--install")
		q.Add(argparse.Arg{Action: argparse.StoreTrue}, "--fix")
		q.Add(argparse.Arg{Action: argparse.StoreTrue}, "--json")
	}
	return p
}

type options struct {
	install, target string
	fix, json       bool
}

// Main is station-modules.py main().
func Main(args []string, s cli.Stdio) int {
	ns, code, done := parser().Parse(args, s)
	if done {
		return code
	}
	o := options{install: ns.String("install"), target: ns.String("target"), fix: ns.Bool("fix"), json: ns.Bool("json")}
	var err error
	if ns.String("cmd") == "check" {
		code, err = check(o, s.Out)
	} else {
		code, err = doctor(o, s.Out)
	}
	if e, ok := err.(*exitError); ok {
		fmt.Fprintln(s.Err, e.msg)
		return e.code
	}
	if err != nil {
		fmt.Fprintf(s.Err, "station-modules: %s\n", err)
		return 1
	}
	return code
}

func autodetect(explicit string) (string, error) {
	if explicit != "" {
		if !pycompat.IsDir(explicit) {
			return "", fail(1, "error: install root not found: %s", pycompat.StrRepr(explicit))
		}
		return explicit, nil
	}
	for _, c := range installCandidates {
		if pycompat.IsDir(pycompat.Join(c, "modules")) {
			return c, nil
		}
	}
	return "", fail(1, "error: could not autodetect a Niagara install; pass --install ROOT")
}

// install is the indexed install shared by both commands.
type install struct {
	root, modulesDir string
	parts            *partIndex
	staged           map[string][]string
}

func openInstall(explicit string) (*install, error) {
	root, err := autodetect(explicit)
	if err != nil {
		return nil, err
	}
	parts, dir, ok := indexInstall(root)
	if !ok {
		return nil, fail(1, "error: no modules/ under install root %s", pycompat.StrRepr(root))
	}
	return &install{root, dir, parts, indexSw(root)}, nil
}

// missingSet is an insertion-ordered {part: reason} dict filled with setdefault.
type missingSet struct {
	order  []string
	reason map[string]string
}

func (m *missingSet) setDefault(p, reason string) {
	if _, ok := m.reason[p]; !ok {
		m.order = append(m.order, p)
		m.reason[p] = reason
	}
}

func newMissing() *missingSet { return &missingSet{reason: map[string]string{}} }

type moduleRow struct {
	module          string
	installed, refs []string
}

func check(o options, w io.Writer) (int, error) {
	in, err := openInstall(o.install)
	if err != nil {
		return 0, err
	}
	text, err := bog.ReadText(o.target)
	if err != nil {
		return 0, fail(2, "error: cannot read station file: %s", err)
	}
	_, refs := stationRefs(text)
	missing := newMissing()
	var rows []moduleRow
	seed := map[string]bool{}
	var seedOrder []string
	for _, module := range slices.Sorted(maps.Keys(setOfKeys(refs))) {
		installed := map[string]bool{}
		in.parts.each(func(p *part) {
			if p.module == module {
				installed[p.name] = true
			}
		})
		ts := slices.Sorted(maps.Keys(refs[module]))
		for _, tn := range ts {
			if prov := typeProvider(in.parts, module, tn); prov != "" {
				if !seed[prov] {
					seed[prov] = true
					seedOrder = append(seedOrder, prov)
				}
			} else {
				missing.setDefault(module+"-rt", fmt.Sprintf("provides %s:%s (used by station)", module, tn))
			}
		}
		if len(installed) == 0 && len(ts) == 0 {
			missing.setDefault(module+"-rt", fmt.Sprintf("module %s referenced, no part installed", module))
		}
		rows = append(rows, moduleRow{module, slices.Sorted(maps.Keys(installed)), ts})
	}
	// Python iterates a set here; with more than one new name its order follows the string
	// hash. Sorted is the deterministic choice.
	for _, dm := range slices.Sorted(maps.Keys(resolveDeps(in.parts, seedOrder))) {
		missing.setDefault(dm, "dependency of an installed part the station uses")
	}
	fixes, err := planFixes(missing, in, o.fix)
	if err != nil {
		return 0, err
	}
	if o.json {
		mods := pyjson.NewObject()
		for _, r := range rows {
			m := pyjson.NewObject()
			m.Set("installed", pyjson.StringList(r.installed))
			m.Set("types", pyjson.StringList(r.refs))
			mods.Set(r.module, m)
		}
		out := pyjson.NewObject()
		out.Set("install", pyjson.String(in.root))
		out.Set("station", pyjson.String(o.target))
		out.Set("modules", mods)
		out.Set("missing", missing.json())
		out.Set("fixes", fixes.json())
		fmt.Fprintln(w, pyjson.Dumps(out, pyjson.Indent2))
	} else {
		fmt.Fprintf(w, "install : %s\nstation : %s\nmodules referenced: %d\n", in.root, o.target, len(rows))
		for _, r := range rows {
			tag := "— NONE —"
			if len(r.installed) > 0 {
				profiles := map[string]bool{}
				for _, p := range r.installed {
					profiles[in.parts.byName[p].profile] = true
				}
				tag = strings.Join(slices.Sorted(maps.Keys(profiles)), ",")
			}
			tsug := ""
			if len(r.refs) > 0 {
				tsug = "  types:" + strings.Join(r.refs, ",")
			}
			fmt.Fprintf(w, "  %-18s parts:[%s]%s\n", r.module, tag, tsug)
		}
		printMissing(w, missing, fixes)
	}
	return exitFor(missing), nil
}

func setOfKeys[V any](m map[string]V) map[string]bool {
	out := make(map[string]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func typeProvider(x *partIndex, module, typ string) string {
	for _, n := range x.order {
		if p := x.byName[n]; p.module == module && p.types[typ] {
			return n
		}
	}
	return ""
}

// resolveDeps is resolve_deps: the dependencies, over the transitive closure of the
// installed seed parts, that are not installed.
func resolveDeps(x *partIndex, seed []string) map[string]bool {
	seen, missing := map[string]bool{}, map[string]bool{}
	stack := append([]string(nil), seed...)
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[p] {
			continue
		}
		seen[p] = true
		meta := x.byName[p]
		if meta == nil {
			continue
		}
		for _, dep := range meta.deps {
			if x.byName[dep] != nil {
				stack = append(stack, dep)
			} else {
				missing[dep] = true
			}
		}
	}
	return missing
}

func doctor(o options, w io.Writer) (int, error) {
	in, err := openInstall(o.install)
	if err != nil {
		return 0, err
	}
	missing := newMissing()
	in.parts.each(func(p *part) {
		for _, dep := range p.deps {
			if in.parts.byName[dep] == nil {
				missing.setDefault(dep, "required by "+p.name)
			}
		}
	})
	fixes, err := planFixes(missing, in, o.fix)
	if err != nil {
		return 0, err
	}
	if o.json {
		out := pyjson.NewObject()
		out.Set("install", pyjson.String(in.root))
		out.Set("installed_parts", pyjson.IntValue(int64(len(in.parts.order))))
		out.Set("missing", missing.json())
		out.Set("fixes", fixes.json())
		fmt.Fprintln(w, pyjson.Dumps(out, pyjson.Indent2))
	} else {
		fmt.Fprintf(w, "install : %s\ninstalled module-parts: %d\n", in.root, len(in.parts.order))
		printMissing(w, missing, fixes)
	}
	return exitFor(missing), nil
}

func exitFor(m *missingSet) int {
	if len(m.order) > 0 {
		return 1
	}
	return 0
}

func (m *missingSet) json() pyjson.Value {
	out := pyjson.NewObject()
	for _, p := range m.order {
		out.Set(p, pyjson.String(m.reason[p]))
	}
	return out
}
