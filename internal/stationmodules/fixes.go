package stationmodules

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"

	"github.com/angeles725/niagara-go/internal/pycompat"
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
	for _, name := range slices.Sorted(maps.Keys(setOfKeys(missing.reason))) {
		f := &fix{reason: missing.reason[name]}
		if cands := in.staged[name]; len(cands) > 0 {
			src, srcVer := locateFix(cands, ver, jarVersion)
			f.hasStaged, f.staged, f.stagedVersion = true, src, srcVer
			dst := pycompat.Join(in.modulesDir, name+".jar")
			if sameBytes(src, dst) {
				// cp would copy the file onto itself: whatever makes the part missing, the
				// staged jar does not change it.
				f.action, f.dst = "identical", dst
				plan.order = append(plan.order, name)
				plan.byPart[name] = f
				continue
			}
			f.cp = fmt.Sprintf(`cp "%s" "%s"`, src, dst)
			if ver != "" && srcVer != "" && srcVer != ver {
				f.warn = fmt.Sprintf("staged version %s != install %s", srcVer, ver)
			}
			if doFix {
				dst, action, err := applyFix(name, src, in.modulesDir)
				if err != nil {
					return nil, err
				}
				if f.sha, err = pycompat.FileSHA256(dst); err != nil {
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
			if f.cp != "" {
				e.Set("cp", pyjson.String(f.cp))
			}
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

// printMissing is print_missing_and_fixes, followed by the station types the installed
// version does not provide (unprovided, nil for doctor).
func printMissing(w io.Writer, missing *missingSet, fixes *fixPlan, unprovided *missingSet) {
	if unprovided != nil && len(unprovided.order) > 0 {
		defer printUnprovided(w, unprovided)
	}
	if len(missing.order) == 0 {
		if unprovided == nil || len(unprovided.order) == 0 {
			fmt.Fprintln(w, "\nOK — all required module-parts are installed. Nothing missing.")
		}
		return
	}
	fmt.Fprintf(w, "\nMISSING module-parts: %d\n", len(missing.order))
	for _, name := range slices.Sorted(maps.Keys(setOfKeys(missing.reason))) {
		f := fixes.byPart[name]
		fmt.Fprintf(w, "  ✗ %s  (%s)\n", name, f.reason)
		if f.hasStaged {
			vtag := " (v" + f.stagedVersion + ")"
			switch f.action {
			case "copied":
				fmt.Fprintf(w, "      → COPIED%s from %s\n        sha256 %s\n", vtag, f.staged, f.sha)
			case "identical":
				fmt.Fprintf(w, "      → identical jar already at %s (copying it changes nothing)\n", f.dst)
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

// printUnprovided lists the station types that no installed part provides although the
// module's rt part is installed: a station saved with another version of the module.
func printUnprovided(w io.Writer, unprovided *missingSet) {
	fmt.Fprintf(w, "\nTYPES NOT IN THE INSTALLED VERSION: %d\n", len(unprovided.order))
	for _, t := range unprovided.order {
		fmt.Fprintf(w, "  ✗ %s  (%s)\n", t, unprovided.reason[t])
	}
}

// sameBytes reports whether a and b are both readable and hold the same bytes.
func sameBytes(a, b string) bool {
	da, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	db, err := os.ReadFile(b)
	return err == nil && bytes.Equal(da, db)
}
