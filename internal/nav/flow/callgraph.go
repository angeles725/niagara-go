package flow

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/lib/callgraph"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

// callers and callees (callgraph.py cmd_callers, cmd_callees): the CLI view of
// the nav/callgraph lookups.

// load is load_callgraph plus `if not cg: return` and the class table
// (`ci_data.get("classes", {}) if ci_data else {}`): nil when the call graph
// is missing, unreadable or empty.
func load(env *registry.Env) (*callgraph.Graph, *index.Object) {
	cg := env.Load("callgraph-index.json", core.LoadOpts{})
	if cg == nil || cg.Len() == 0 {
		return nil, nil
	}
	g := callgraph.New(cg)
	var classes *index.Object
	if ci := env.LoadClassIndex(); ci != nil {
		classes = ci.Object("classes")
	}
	return g, classes
}

// more is the "... and N more (use -n N to see all)" tail of a capped list.
func more(env *registry.Env, n, limit int) {
	if n > limit {
		fmt.Fprintln(env.Out)
		fmt.Fprintf(env.Out, "  ... and %d more (use -n %d to see all)\n", n-limit, n)
	}
}

// head is items[:limit] with Python's slice rule (a negative limit counts from the end).
func head[T any](items []T, limit int) []T {
	if limit < 0 {
		return items[:max(0, len(items)+limit)]
	}
	return items[:min(limit, len(items))]
}

func runCallers(env *registry.Env, ns *argparse.Namespace) int {
	class, limit := ns.String("class_name"), ns.Int("n")
	g, classes := load(env)
	if g == nil {
		return env.ExitCode()
	}
	w := env.Out
	method, ok := ns.Get("method").(string)
	if !ok {
		callersOfClass(env, g, classes, class, limit)
		return env.ExitCode()
	}
	r := g.CallersOf(classes, class, method)
	if len(r.Rows) == 0 {
		env.Miss(fmt.Sprintf("callers of '%s'", r.Key), g.Scope(), fmt.Sprintf("grep '\\.%s\\('", r.Method))
		if len(r.Partial) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  Other classes with callers for '%s':\n", r.Method)
			for _, p := range r.Partial[:min(15, len(r.Partial))] {
				fmt.Fprintln(w, p.Line())
			}
			if len(r.Partial) > 15 {
				fmt.Fprintf(w, "    ... and %d more\n", len(r.Partial)-15)
			}
		}
		fmt.Fprintln(w)
		return env.ExitCode()
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  CALLERS of %s.%s: %d callers\n", r.Class, r.Method, len(r.Rows))
	fmt.Fprintf(w, "  Target module: %s\n", r.TargetModule)
	fmt.Fprintln(w)
	fmt.Fprintln(w, callgraph.CallersHeader)
	fmt.Fprintln(w, callgraph.CallersRule)
	for _, row := range r.Rows[:max(0, min(limit, len(r.Rows)))] {
		fmt.Fprintln(w, row.Line())
	}
	more(env, len(r.Rows), limit)
	fmt.Fprintln(w)
	return env.ExitCode()
}

// callersOfClass is _callers_of_class: per-method caller counts, then the
// call sites; limit caps both.
func callersOfClass(env *registry.Env, g *callgraph.Graph, classes *index.Object, class string, limit int) {
	w := env.Out
	r := g.CallersOfClass(classes, class)
	if len(r.Ambiguous) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  '%s' matches %d classes ignoring case: %s\n", class, len(r.Ambiguous),
			strings.Join(r.Ambiguous, ", "))
		fmt.Fprintf(w, "  Try: callers %s\n", r.Ambiguous[0])
		fmt.Fprintln(w)
		env.Ambiguous()
		return
	}
	if len(r.Methods) == 0 {
		env.Miss(fmt.Sprintf("callers of any method of '%s'", class), g.Scope(), "callees "+class)
		return
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  CALLERS of %s.*: %d methods with callers, %d call sites\n", r.Class, len(r.Methods), r.Total)
	fmt.Fprintf(w, "  Target module: %s\n", r.TargetModule)
	fmt.Fprintln(w)
	fmt.Fprintln(w, callgraph.MethodsHeader)
	fmt.Fprintln(w, callgraph.MethodsRule)
	for _, m := range head(r.Methods, limit) {
		fmt.Fprintln(w, m.Line())
	}
	if len(r.Methods) > limit {
		fmt.Fprintf(w, "  ... and %d more methods (use -n %d)\n", len(r.Methods)-limit, len(r.Methods))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, callgraph.SitesHeader)
	fmt.Fprintln(w, callgraph.SitesRule)
	for _, s := range head(r.Sites, limit) {
		fmt.Fprintln(w, s.Line())
	}
	if len(r.Sites) > limit {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  ... and %d more call sites (use -n %d to see all, or callers %s <method>)\n",
			len(r.Sites)-limit, len(r.Sites), r.Class)
	}
	fmt.Fprintln(w)
}

func runCallees(env *registry.Env, ns *argparse.Namespace) int {
	class, limit := ns.String("class_name"), ns.Int("n")
	g, classes := load(env)
	if g == nil {
		return env.ExitCode()
	}
	w := env.Out
	if method, _ := ns.Get("method").(string); method != "" {
		r := g.CalleesOf(classes, class, method)
		if len(r.Rows) == 0 {
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  No callees found for '%s'.\n", r.Key)
			if len(r.ClassMethods) > 0 {
				fmt.Fprintf(w, "  %s has %d methods with calls. Try:\n", class, len(r.ClassMethods))
				for _, k := range r.ClassMethods[:min(10, len(r.ClassMethods))] {
					fmt.Fprintf(w, "    callees %s %s\n", class, afterFirstDot(k))
				}
			}
			fmt.Fprintln(w)
			return env.ExitCode()
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  CALLEES of %s: %d calls\n", r.Key, len(r.Rows))
		fmt.Fprintln(w)
		fmt.Fprintln(w, callgraph.CalleesHeader)
		fmt.Fprintln(w, callgraph.CalleesRule)
		for _, row := range r.Rows[:max(0, min(limit, len(r.Rows)))] {
			fmt.Fprintln(w, row.Line())
		}
		more(env, len(r.Rows), limit)
		fmt.Fprintln(w)
		return env.ExitCode()
	}
	r := g.CalleesOfClass(classes, class)
	if len(r.Methods) == 0 {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  No call data found for class '%s'.\n", class)
		if len(r.DidYouMean) > 0 {
			fmt.Fprintf(w, "  Did you mean: %s\n", strings.Join(r.DidYouMean, ", "))
		}
		fmt.Fprintln(w)
		return env.ExitCode()
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  CALLEES of %s (all methods): %d methods, %d total calls\n", class, len(r.Methods), r.Total)
	fmt.Fprintf(w, "  Module: %s\n", r.Module)
	fmt.Fprintln(w)
	for _, m := range r.Methods[:max(0, min(limit, len(r.Methods)))] {
		fmt.Fprintf(w, "  %s (%d calls):\n", m.Method, m.Calls)
		for _, row := range m.Rows {
			fmt.Fprintln(w, row.ArrowLine())
		}
		if m.Calls > 10 {
			fmt.Fprintf(w, "    ... and %d more\n", m.Calls-10)
		}
		fmt.Fprintln(w)
	}
	if len(r.Methods) > limit {
		fmt.Fprintf(w, "  ... and %d more methods (use -n %d to see all)\n", len(r.Methods)-limit, len(r.Methods))
		fmt.Fprintln(w)
	}
	return env.ExitCode()
}

// afterFirstDot is key.split('.', 1)[1].
func afterFirstDot(key string) string { return key[strings.IndexByte(key, '.')+1:] }
