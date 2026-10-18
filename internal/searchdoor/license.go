package searchdoor

import (
	"fmt"
	"io"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// `license <module|feature>` (searchdoor/license.py): what a module licenses,
// or where a feature is licensed, from all three sources.
//
//	CODE           module_nav license-features --module <m> | license-feature <f>
//	OFFICIAL DOCS  niagara_help find "license <stem>"  (an any-term widening is a zero)
//	OUR BLOCKS     corpus-nav find "license <stem>" --and
//
// A name ending in -rt/-wb/-ux/-se/-doc, or listed by `module_nav modules`, is
// a module; anything else a feature. <stem> drops the module suffix.

const licenseHint = "check each source's scope line above; try the module name (bacnet-rt) or the license " +
	"feature name (bacnet) instead."

var moduleSuffix = re(`-(rt|wb|ux|se|doc)$`)

// resolve is license.resolve: ("module" | "feature", why).
func (d *door) resolve(name, codePath string, timeout int) (string, string) {
	if moduleSuffix.MatchString(name) {
		return "module", "'" + name + "' ends in a module suffix"
	}
	raw := d.backend.Run(codeSource, []string{codePath, "modules"}, float64(timeout))
	if raw.State != OK {
		return "feature", fmt.Sprintf("module list unavailable (%s); '%s' has no module suffix, "+
			"so it is treated as a feature", raw.Reason, name)
	}
	names := moduleNames(raw.Out)
	if names[name] {
		return "module", "'" + name + "' is listed by module_nav modules"
	}
	return "feature", fmt.Sprintf("'%s' is not one of %d module names, so it is treated as a feature", name, len(names))
}

func (d *door) licenseCodeJob(name, kind, codePath string, timeout int, why string) job {
	argv := []string{codePath, "license-feature", name}
	title := "CODE · call sites of feature " + name
	if kind == "module" {
		argv = []string{codePath, "license-features", "--module", name}
		title = "CODE · features module " + name + " requires"
	}
	command := "module_nav " + strings.Join(argv[1:], " ")
	return job{"code", title, command, func() *Outcome {
		raw := d.backend.Run(codeSource, argv, float64(timeout))
		o := newOutcome("code", title, command, raw.State)
		o.Reason, o.MS = raw.Reason, raw.MS
		o.Notes = append(o.Notes, why)
		if raw.State == OK {
			o.Scope, o.Items = parseLicense(raw.Out)
		} else if raw.Out != "" {
			o.Scope = strippedNonblank(raw.Out, scopeLines)
		}
		return o
	}}
}

// license is license.run: the code, official docs and blocks outcomes.
func (d *door) license(w io.Writer, name string, p Paths, limit, timeout int, docVersion string) []*Outcome {
	kind, why := d.resolve(name, p.Code, timeout)
	stem := name
	if m := moduleSuffix.Search(name); kind == "module" && m != nil { // MODULE_SUFFIX.sub("", name)
		r := []rune(name)
		stem = string(r[:m.Start()]) + string(r[m.End():])
	}
	query := "license " + stem
	fmt.Fprintf(w, "\nLicense: %s resolved as a %s   docs/blocks query=%s\n\n", pycompat.StrRepr(name), kind,
		pycompat.StrRepr(query))
	return runJobs([]job{
		d.licenseCodeJob(name, kind, p.Code, timeout, why),
		d.docsJob(p.Docs, query, limit, "", timeout, "OFFICIAL DOCS · license pages", true, docVersion),
		d.blocksJob(p.Blocks, query, limit, "and", timeout, "OUR BLOCKS · licensing research"),
	})
}
