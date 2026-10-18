package domain

import (
	"fmt"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/index"
	"github.com/angeles725/niagara-go/internal/nav/core"
	"github.com/angeles725/niagara-go/internal/nav/license"
)

// The library half of `drivers` (decision D29): Drivers takes the decoded
// indexes, the corpus root and the license features explicitly, prints
// nothing and returns typed rows; DriverTableHeader, FormatDriverRow and
// FormatAnatomy yield the exact lines the command prints, so the command and
// nsearch show the same text. Loading the indexes, scanning the licensing
// calls (license.Scan of the base directory) and the miss reports belong
// to the command (drivers.go, drivers_print.go).

// DriverRow is one driver network (the dict _anatomy returns).
type DriverRow struct {
	Network, Module, Package, Path string
	Abstract                       bool
	Extends, Family                string
	Devices                        []string
	DevicesFrom                    string // the submodule the devices come from
	DevicesVia                     string // "subclasses" or "getDeviceType()"
	PointDeviceExts                []string
	PointDeviceExtsFrom            string
	ProxyExts                      []string
	ProxyExtsFrom                  string
	Managers, DiscoveryJobs        []string
	License                        []string // "vendor:feature", sorted
	doc                            docTarget
}

// DriverInputs is what Drivers reads.
type DriverInputs struct {
	ClassIndex  *index.Object       // class-index.json: its classes table and _meta
	Inheritance *index.Object       // inheritance.json
	Inventory   *index.Object       // module-inventory.json
	Organized   string              // the corpus root (organized/) the class paths are under
	Features    map[string][]string // per submodule, its "vendor:feature" license calls (FeaturesByModule)
}

// FeaturesByModule is the license features of each submodule, "vendor:feature"
// ("?" for an unresolved vendor), from the licensing calls license.Scan finds.
func FeaturesByModule(calls []license.Call) map[string][]string {
	features := map[string][]string{}
	for _, c := range calls {
		if c.Feature != nil && *c.Feature != "" {
			vendor := "?"
			if c.Vendor != nil && *c.Vendor != "" {
				vendor = *c.Vendor
			}
			features[c.Module] = append(features[c.Module], vendor+":"+*c.Feature)
		}
	}
	return features
}

// Drivers is driver_networks: every driver network (a top-level class of an
// -rt submodule whose chain reaches BDeviceNetwork), by submodule then name,
// and the scope line that counts them.
func Drivers(in DriverInputs) (rows []*DriverRow, scope string) {
	classes := in.ClassIndex.Object("classes")
	if classes == nil {
		classes = index.NewObject()
	}
	cx := newCorpus(classes, in.Inheritance)
	docs := newDocIndex(in.Inventory.Object("modules"))
	subs := append([]string(nil), cx.subs...)
	sort.Strings(subs)
	modules := map[string]bool{}
	for _, sub := range subs {
		if !strings.HasSuffix(sub, "-rt") {
			continue
		}
		items := append([]item(nil), cx.bySub[sub]...)
		sort.SliceStable(items, func(a, b int) bool { return items[a].name < items[b].name })
		for _, it := range items {
			if contains(cx.chain(it.e), "BDeviceNetwork") {
				rows = append(rows, anatomy(cx, it, in.Organized, in.Features, docs))
				modules[sub] = true
			}
		}
	}
	entries := 0
	for _, v := range classes.Values() {
		l, _ := v.([]any)
		entries += len(l)
	}
	scope = fmt.Sprintf("%s driver networks (BDeviceNetwork subclasses) in %s rt submodules of %s "+
		"class entries, %s", core.Commas(int64(len(rows))), core.Commas(int64(len(modules))),
		core.Commas(int64(entries)), core.Built(in.ClassIndex.Object("_meta")))
	return rows, scope
}

// DriversIn is the rows of module: a submodule (modbusTcp-rt) or a module
// directory (modbusTcp).
func DriversIn(rows []*DriverRow, module string) []*DriverRow {
	var out []*DriverRow
	for _, r := range rows {
		if module == r.Module || module == familyDir(r.Path) {
			out = append(out, r)
		}
	}
	return out
}

func optStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// Object is the row as Python's dict, in its key order (the --json network).
func (r *DriverRow) Object() *index.Object {
	o := index.NewObject()
	var exists any
	if r.doc.exists != nil {
		exists = *r.doc.exists
	}
	for _, kv := range []struct {
		k string
		v any
	}{
		{"network", r.Network}, {"module", r.Module}, {"package", r.Package}, {"abstract", r.Abstract},
		{"extends", r.Extends}, {"family", r.Family}, {"devices", r.Devices},
		{"devices_from", r.DevicesFrom}, {"devices_resolved_via", r.DevicesVia},
		{"point_device_exts", r.PointDeviceExts}, {"point_device_exts_from", r.PointDeviceExtsFrom},
		{"proxy_exts", r.ProxyExts}, {"proxy_exts_from", r.ProxyExtsFrom}, {"managers", r.Managers},
		{"discovery_jobs", r.DiscoveryJobs}, {"license_features", r.License},
		{"help_guide_base", optStr(r.doc.base)}, {"help_guide_base_others", r.doc.others},
		{"help_doc_module", optStr(r.doc.target)}, {"help_doc_module_exists", exists},
		{"doc_module", optStr(r.doc.doc)}, {"doc_module_via", optStr(r.doc.via)}, {"path", r.Path},
	} {
		o.Set(kv.k, kv.v)
	}
	return o
}

// first is _first: the first value and how many more.
func first(values []string) string {
	switch len(values) {
	case 0:
		return "-"
	case 1:
		return values[0]
	}
	return fmt.Sprintf("%s (+%d)", values[0], len(values)-1)
}

func docCell(r *DriverRow) string {
	if r.doc.doc != nil && *r.doc.doc != "" {
		if r.doc.via != nil && *r.doc.via == "by name" {
			return *r.doc.doc + " (by name)"
		}
		return *r.doc.doc
	}
	if r.doc.target != nil && *r.doc.target != "" {
		return *r.doc.target + " (missing)"
	}
	return "-"
}

func docLine(r *DriverRow) string {
	if r.doc.base == nil {
		return docCell(r)
	}
	note := ""
	if r.doc.exists != nil && !*r.doc.exists {
		note = fmt.Sprintf("; %s is not in organized/", core.PyStr(optStr(r.doc.target)))
	}
	if len(r.doc.others) > 0 {
		note += "; other submodule lexicons: " + strings.Join(r.doc.others, ", ")
	}
	doc := "-"
	if r.doc.doc != nil && *r.doc.doc != "" {
		doc = *r.doc.doc
	}
	return fmt.Sprintf("%s  [help.guide.base=%s%s]", doc, *r.doc.base, note)
}

func tableLine(cols ...string) string {
	return fmt.Sprintf("  %s %s %s %s %s %s", core.LJust(cols[0], 30), core.LJust(cols[1], 24),
		core.LJust(cols[2], 11), core.LJust(cols[3], 30), core.LJust(cols[4], 24), cols[5])
}

// DriverTableHeader is the column line and the rule of the `drivers` table.
func DriverTableHeader() [2]string {
	return [2]string{tableLine("NETWORK", "MODULE", "FAMILY", "DEVICE", "LICENSE", "DOC"),
		"  " + strings.Repeat("-", 140)}
}

// FormatDriverRow is the `drivers` table row of r: network (" *" when
// abstract), module, family, first device, first license feature, doc module.
func FormatDriverRow(r *DriverRow) string {
	name := r.Network
	if r.Abstract {
		name += " *"
	}
	return tableLine(name, r.Module, r.Family, first(r.Devices), first(r.License), docCell(r))
}

// FormatAnatomy is the `drivers <module>` block of r, one line per entry,
// opening and closing blank lines included.
func FormatAnatomy(r *DriverRow) []string {
	abstract := ""
	if r.Abstract {
		abstract = " (abstract)"
	}
	out := []string{"", fmt.Sprintf("  DRIVER %s%s  (%s, %s)", r.Network, abstract, r.Module, r.Package),
		"  " + strings.Repeat("=", 70),
		fmt.Sprintf("  %s %s (extends %s)", core.LJust("family", 18), r.Family, r.Extends)}
	devicesFrom := r.DevicesFrom
	if r.DevicesVia == "getDeviceType()" {
		devicesFrom = "getDeviceType()"
	}
	for _, f := range []struct {
		label  string
		values []string
		origin string
	}{
		{"devices", r.Devices, devicesFrom}, {"point-device-ext", r.PointDeviceExts, r.PointDeviceExtsFrom},
		{"proxy-ext", r.ProxyExts, r.ProxyExtsFrom}, {"managers", r.Managers, ""},
		{"discovery jobs", r.DiscoveryJobs, ""}, {"license", r.License, ""},
	} {
		values, note := "-", ""
		if len(f.values) > 0 {
			values = strings.Join(f.values, ", ")
			if f.origin != "" && f.origin != r.Module {
				note = "  [from " + f.origin + "]"
			}
		}
		out = append(out, fmt.Sprintf("  %s %s%s", core.LJust(f.label, 18), values, note))
	}
	return append(out, fmt.Sprintf("  %s %s", core.LJust("doc module", 18), docLine(r)),
		fmt.Sprintf("  %s %s", core.LJust("source", 18), r.Path), "")
}
