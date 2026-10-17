package source

import (
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/codesearch"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// runToken is commands/source.py run_token (tokens.cmd_token): the
// occurrences of an exact token from token-index.db, as a compact table with
// a module summary or, with --context N > 0, as source excerpts.
func runToken(env *registry.Env, ns *argparse.Namespace) int {
	word, module, limit, ctx := ns.String("word"), ns.String("token_module"), ns.Int("n"), ns.Int("token_context")
	db := env.Connect("token-index.db", false)
	if db == nil {
		return env.ExitCode()
	}
	out := env.Stdio.Out
	hits, err := codesearch.Tokens(db, word, module)
	if err != nil {
		env.Unreadable(err)
		return env.ExitCode()
	}
	if len(hits) == 0 {
		tokenMiss(env, word, module)
		return env.ExitCode()
	}
	fmt.Fprintf(out, "\n%s\n\n", codesearch.FormatTokenHeader(word, module, hits))
	if ctx > 0 {
		tokenContext(env, hits, limit, ctx)
	} else {
		tokenCompact(out, hits, limit)
	}
	return env.ExitCode()
}

// tokenMiss is the zero-occurrence path of cmd_token: the miss, then the
// count without the module filter or up to 10 similar tokens.
func tokenMiss(env *registry.Env, word, module string) {
	out := env.Stdio.Out
	db := env.Connect("token-index.db", false)
	similar, err := codesearch.SimilarTokens(db, word) // run in both cases, as in Python
	if err != nil {
		env.Unreadable(err)
		return
	}
	subject := "token '" + word + "'"
	if module != "" {
		subject += " in module '" + module + "'"
	}
	fmt.Fprintln(out)
	env.Miss(subject, "the postings of token-index.db", "")
	if module != "" {
		total, err := codesearch.CountToken(db, word)
		if err != nil {
			env.Unreadable(err)
			return
		}
		if total > 0 {
			fmt.Fprintf(out, "  Found %d occurrences without module filter.\n  Try: token %s\n", total, word)
		}
	} else if len(similar) > 0 {
		fmt.Fprintf(out, "  Similar tokens: %s\n", strings.Join(similar, ", "))
	}
	fmt.Fprintln(out)
}

// tokenCompact is _show_compact: up to limit rows by module, then the top modules.
func tokenCompact(out io.Writer, hits []codesearch.TokenHit, limit int) {
	head := codesearch.TokenTableHeader()
	fmt.Fprintf(out, "%s\n%s\n", head[0], head[1])
	mods := codesearch.ByModule(hits)
	shown := 0
	for _, m := range mods {
		for _, h := range m.Hits {
			if shown >= limit {
				fmt.Fprintf(out, "  ... and %s more (use -n %s to see all)\n\n",
					core.Commas(int64(len(hits)-shown)), core.Commas(int64(len(hits))))
				moduleSummary(out, mods)
				return
			}
			fmt.Fprintln(out, codesearch.FormatTokenRow(h))
			shown++
		}
	}
	fmt.Fprintln(out)
	moduleSummary(out, mods)
}

// moduleSummary is _show_module_summary: the 10 modules with most hits.
func moduleSummary(out io.Writer, mods []codesearch.ModuleHits) {
	fmt.Fprintln(out, "  Top modules:")
	for _, m := range codesearch.TopModules(mods, 10) {
		fmt.Fprintln(out, codesearch.FormatModuleHits(m))
	}
	if len(mods) > 10 {
		fmt.Fprintf(out, "    ... and %d more modules\n", len(mods)-10)
	}
	fmt.Fprintln(out)
}

// tokenContext is _show_with_context: each occurrence with ctx lines around
// it, one header per file; without a corpus it falls back to the compact view.
func tokenContext(env *registry.Env, hits []codesearch.TokenHit, limit, ctx int) {
	out := env.Stdio.Out
	meta, err := codesearch.TokenMeta(env.Connect("token-index.db", false))
	if err != nil {
		env.Unreadable(err)
		return
	}
	root := env.SourceRoot(meta)
	if root == "" {
		fmt.Fprintln(out, "  ERROR: Source root not available. Falling back to compact view.")
		tokenCompact(out, hits, limit)
		return
	}
	shown := 0
	prev := ""
	var lines []string // nil: the current file could not be read
	for i, h := range hits {
		if shown >= limit {
			fmt.Fprintf(out, "  ... and %s more (use -n %s to see all)\n",
				core.Commas(int64(len(hits)-shown)), core.Commas(int64(len(hits))))
			break
		}
		if i == 0 || h.Path != prev {
			lines, err = codesearch.ReadLines(pycompat.PosixJoin(root, h.Path))
			if err != nil {
				lines = nil
			}
			prev = h.Path
			if shown > 0 {
				fmt.Fprintln(out)
			}
			fmt.Fprintf(out, "  --- %s (%s) ---\n", h.Class, h.Module)
		}
		if lines == nil {
			fmt.Fprintf(out, "    %5d: (source not available)\n", h.Line)
			shown++
			continue
		}
		start := max(0, h.Line-1-int64(ctx))
		end := min(int64(len(lines)), h.Line+int64(ctx))
		for j := start; j < end; j++ {
			marker := "    "
			if j+1 == h.Line {
				marker = " >> "
			}
			fmt.Fprintf(out, "  %s%5d: %s\n", marker, j+1, codesearch.Ellipsis(codesearch.RStrip(lines[j]), 100))
		}
		if shown < limit-1 && shown+1 < len(hits) {
			next := hits[shown+1]
			if next.Path != h.Path || next.Line-h.Line > int64(ctx)*2+1 {
				fmt.Fprintln(out)
			}
		}
		shown++
	}
	fmt.Fprintln(out)
}
