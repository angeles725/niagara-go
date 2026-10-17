package bognav

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
)

// Alerton Compass DisplayService PointCache: bacref ORD decoding and freshness
// (bognav/pointcache.py).

// bacnetObjType and bacnetProp decode the BACnet enum subset seen in Alerton PointCache ORDs.
var bacnetObjType = map[string]string{"0": "analog-input", "1": "analog-output", "2": "analog-value",
	"3": "binary-input", "4": "binary-output", "5": "binary-value", "8": "device", "10": "file",
	"13": "multistate-input", "14": "multistate-output", "19": "multistate-value", "20": "trend-log"}

var bacnetProp = map[string]string{"28": "description", "36": "event-state", "70": "model-name",
	"75": "object-identifier", "77": "object-name", "79": "object-type", "81": "out-of-service",
	"85": "present-value", "87": "priority-array", "103": "reliability", "111": "status-flags", "117": "units"}

func getOr(m map[string]string, k, def string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return def
}

// bacrefRE is r'bacref:([a-z]+)=(.+)$': Python's $ also matches before a final "\n".
var bacrefRE = regexp.MustCompile(`bacref:([a-z]+)=([^\n]+)\n?$`)

// decodeBacref is _decode_bacref: (form, objType, objInst, prop, dev) of an Alerton
// DisplayService ORD key. Forms: dopr=<objType>_<objInst>_<propId>_8_<devInst> and
// devobjid=<devInst>_<objType>_<objInst>; `$3a` is ':' and `$3d` is '='.
func decodeBacref(name string) (form, ot, oi, prop, dev string) {
	dec := strings.ReplaceAll(strings.ReplaceAll(name, "$3a", ":"), "$3d", "=")
	m := bacrefRE.FindStringSubmatch(dec)
	if m == nil {
		return
	}
	form = m[1]
	p := strings.Split(m[2], "_")
	switch {
	case form == "dopr" && len(p) == 5:
		return form, p[0], p[1], p[2], p[4]
	case form == "devobjid" && len(p) >= 3:
		return form, p[1], p[2], "", p[0]
	}
	return form, "", "", "", ""
}

func cmdPointcache(e *env) (int, int) {
	var rows []row
	for _, c := range e.bog.Comps.All() {
		if c.Type != "disp:PointCacheObj" && c.SimpleType() != "PointCacheObj" {
			continue
		}
		_, ot, oi, prop, dev := decodeBacref(c.Name)
		poll := ""
		if s, ok := c.Slots.Get("polledTime"); ok {
			poll = ptr(s.Value)
		}
		coordType := ot
		if coordType == "" {
			coordType = "?"
		}
		rows = append(rows, row{
			{"coordinate", fmt.Sprintf("%s:%s@dev%s", getOr(bacnetObjType, ot, coordType), oi, dev)},
			{"device", dev}, {"obj_type", getOr(bacnetObjType, ot, ot)}, {"obj_inst", oi},
			{"property", getOr(bacnetProp, prop, prop)}, {"value", ptr(c.ValueVal)},
			{"polled", poll}, {"year", head(poll, 4)},
		})
	}
	if len(rows) == 0 {
		e.println("(no disp:PointCache in this bog -- not an Alerton Compass station, or no cache; ", e.scope, ")")
		return 0, -1
	}
	newest := ""
	for _, r := range rows {
		if y := r.s("year"); y > newest {
			newest = y
		}
	}
	cutoff := e.str("stale_before")
	if cutoff == "" {
		cutoff = newest
	}
	for i := range rows {
		y := rows[i].s("year")
		fresh := "?"
		switch {
		case y != "" && y < cutoff:
			fresh = "STALE"
		case y != "":
			fresh = "live"
		}
		rows[i] = append(rows[i], kv{"fresh", fresh})
	}
	rows = filterRows(rows, func(r row) bool {
		dev := e.str("device")
		return (dev == "" || r.s("device") == dev) &&
			(!e.flag("stale") || r.s("fresh") == "STALE") && (!e.flag("live") || r.s("fresh") == "live")
	})
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.s("device") != b.s("device") {
			return a.s("device") < b.s("device")
		}
		if a.s("obj_type") != b.s("obj_type") {
			return a.s("obj_type") < b.s("obj_type")
		}
		return instKey(a.s("obj_inst")) < instKey(b.s("obj_inst"))
	})
	if e.flag("summary") && e.isText() {
		var years counter
		fresh := map[string]int{}
		for _, r := range rows {
			if y := r.s("year"); y != "" {
				years.add(y)
			}
			fresh[r.s("fresh")]++
		}
		total := len(rows)
		e.println(fmt.Sprintf("PointCache objects: %d   (stale cutoff year: %s)", total, cutoff))
		ys := append([]string(nil), years.keys...)
		sort.Strings(ys)
		for _, y := range ys {
			e.println(fmt.Sprintf("  polled %s: %d", y, years.n[y]))
		}
		line := fmt.Sprintf("  STALE %d / live %d", fresh["STALE"], fresh["live"])
		if total > 0 {
			line += fmt.Sprintf("  = %.1f%% stale", 100*float64(fresh["STALE"])/float64(total))
		}
		e.println(line)
		return 0, -1
	}
	e.render(rows, func(r row) string {
		return ljust(r.s("coordinate"), 24) + " " + ljust(r.s("property"), 16) + " = " + ljust(head(r.s("value"), 14), 14) +
			" polled " + ljust(head(r.s("polled"), 10), 11) + " " + r.s("fresh")
	}, "(no matching PointCache objects)",
		[]string{"coordinate", "device", "obj_type", "obj_inst", "property", "value", "polled", "fresh"})
	return 0, -1
}

func filterRows(rows []row, keep func(row) bool) []row {
	var out []row
	for _, r := range rows {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

// instKey is int(obj_inst) if obj_inst.isdigit() else 0.
func instKey(s string) int {
	if s == "" {
		return 0
	}
	n, err := argparse.ParseInt(s)
	if err != nil || strings.ContainsAny(s, "+-_ ") {
		return 0
	}
	return n
}
