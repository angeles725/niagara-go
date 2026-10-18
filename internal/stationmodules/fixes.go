package stationmodules

import (
	"fmt"
	"io"

	"github.com/angeles725/niagara-go/internal/pyjson"
)

// fix is one entry of plan_fixes, rendered in the Python dict's key order.
type fix struct {
	reason, staged, stagedVersion, cp, warn, action, dst, sha string
	hasStaged                                                 bool
}

type fixPlan struct {
	order  []string
	byPart map[string]*fix
}

// planFixes is plan_fixes: a staged jar (and its cp command) for every missing part, copied
// into modules/ with --fix.
func planFixes(missing *missingSet, in *install, doFix bool) (*fixPlan, error) {
	ver := ""
	in.parts.each(func(p *part) {
		if ver == "" && p.vendor == "Tridium" && p.version != "" {
			ver = p.version
		}
	})
	plan := &fixPlan{byPart: map[string]*fix{}}
	for _, name := range sortedKeys(setOfKeys(missing.reason)) {
		f := &fix{reason: missing.reason[name]}
		if cands := in.staged[name]; len(cands) > 0 {
			src, srcVer := locateFix(cands, ver, jarVersion)
			f.hasStaged, f.staged, f.stagedVersion = true, src, srcVer
			f.cp = fmt.Sprintf(`cp "%s" "%s"`, src, pyJoin(in.modulesDir, name+".jar"))
			if ver != "" && srcVer != "" && srcVer != ver {
				f.warn = fmt.Sprintf("staged version %s != install %s", srcVer, ver)
			}
			if doFix {
				dst, action, err := applyFix(name, src, in.modulesDir)
				if err != nil {
					return nil, err
				}
				if f.sha, err = fileSHA256(dst); err != nil {
					return nil, err
				}
				f.action, f.dst = action, dst
			}
		}
		plan.order = append(plan.order, name)
		plan.byPart[name] = f
	}
	return plan, nil
}

const notStaged = "not staged in sw/ — obtain from the source supervisor's modules/ or a matching install"

func (p *fixPlan) json() pyjson.Value {
	out := pyjson.NewObject()
	for _, name := range p.order {
		f := p.byPart[name]
		e := pyjson.NewObject()
		e.Set("reason", pyjson.String(f.reason))
		if f.hasStaged {
			e.Set("staged", pyjson.String(f.staged))
			e.Set("staged_version", pyjson.String(f.stagedVersion))
			e.Set("cp", pyjson.String(f.cp))
			if f.warn != "" {
				e.Set("warn", pyjson.String(f.warn))
			}
			if f.action != "" {
				e.Set("action", pyjson.String(f.action))
				e.Set("dst", pyjson.String(f.dst))
				e.Set("sha256", pyjson.String(f.sha))
			}
		} else {
			e.Set("staged", pyjson.Value{})
			e.Set("note", pyjson.String(notStaged))
		}
		out.Set(name, e)
	}
	return out
}

// printMissing is print_missing_and_fixes.
func printMissing(w io.Writer, missing *missingSet, fixes *fixPlan) {
	if len(missing.order) == 0 {
		fmt.Fprintln(w, "\nOK — all required module-parts are installed. Nothing missing.")
		return
	}
	fmt.Fprintf(w, "\nMISSING module-parts: %d\n", len(missing.order))
	for _, name := range sortedKeys(setOfKeys(missing.reason)) {
		f := fixes.byPart[name]
		fmt.Fprintf(w, "  ✗ %s  (%s)\n", name, f.reason)
		if f.hasStaged {
			vtag := " (v" + f.stagedVersion + ")"
			switch f.action {
			case "copied":
				fmt.Fprintf(w, "      → COPIED%s from %s\n        sha256 %s\n", vtag, f.staged, f.sha)
			case "exists":
				fmt.Fprintf(w, "      → already present at %s\n", f.dst)
			default:
				fmt.Fprintf(w, "      staged in sw/%s: %s\n      fix: %s\n", vtag, f.staged, f.cp)
			}
			if f.warn != "" {
				fmt.Fprintf(w, "      ⚠ %s\n", f.warn)
			}
		} else {
			fmt.Fprintf(w, "      %s\n", notStaged)
		}
	}
	fmt.Fprintln(w, "\nAfter copying jars into modules/, RESTART Workbench, then reopen config.bog.")
}
