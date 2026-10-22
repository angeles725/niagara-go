package obixnav

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	linkDisplayRE = regexp.MustCompile(`display="Indirect: ([^"]*)"`)
	linkRE        = regexp.MustCompile(`^\s*(\S+)\.(\S+)\s*->\s*slot:/\S+\.(\S+)\s*$`)
)

// compLinks returns {target slot: source slot} for the baja:Link objects of a component; oBIX
// displays each as 'Indirect: <src>.<sslot> -> slot:/<tgtpath>.<tslot>'.
func (r *runner) compLinks(path string) (map[string]string, error) {
	p := path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasPrefix(p, "/Programacion") && !strings.HasPrefix(p, "/Services") {
		if strings.HasPrefix(p, "/DashboardService") || strings.HasPrefix(p, "/Cuarto") ||
			strings.HasPrefix(p, "/Condensadoras") {
			p = "/Services" + p
		} else {
			p = "/Programacion" + p
		}
	}
	xml, err := r.c.get(p, r.s.Err)
	if err != nil {
		return nil, err
	}
	links := map[string]string{}
	for _, d := range linkDisplayRE.FindAllStringSubmatch(xml, -1) {
		if m := linkRE.FindStringSubmatch(strings.ReplaceAll(d[1], "&#x2192;", "->")); m != nil {
			links[m[3]] = m[2]
		}
	}
	return links, nil
}

// expectSlots are the rt control slots that must have SOME facade link (the operator owns which
// evapM drives each unit, so only a slot with no link at all is flagged).
var expectSlots = []string{"evapSetpoint", "evapDifferentialUp", "evapDifferentialDown", "startDelay",
	"valveMode", "fanMode"}

func (r *runner) cmdWiring() error {
	w := r.s.Out
	for _, ri := range r.roomList(siteRooms(true)) {
		cr, err := r.coldRoom(ri)
		if err != nil {
			return err
		}
		units, err := r.listUnits(cr)
		if err != nil {
			return err
		}
		if len(units) == 0 {
			fmt.Fprintf(w, "Cuarto%d: sin unidades\n", ri)
			continue
		}
		fmap, err := r.detectMap(cr, units)
		if err != nil {
			return err
		}
		umap := map[string]int{}
		pairs := make([]string, len(fmap))
		for i, u := range fmap {
			if u != "" {
				umap[u] = i + 1
			}
			pairs[i] = fmt.Sprintf("evap%d->%s", i+1, orNone(u))
		}
		fmt.Fprintf(w, "\n=== Cuarto%d (%s) — mapeo %s: %s ===\n", ri, cr, mappingWord(fmap), strings.Join(pairs, "  "))
		roomCtrl, per, err := r.defrostLayout(cr, units)
		if err != nil {
			return err
		}
		yes := "no"
		if roomCtrl {
			yes = "SÍ"
		}
		fmt.Fprintf(w, "  defrost: room-level=%s  per-evap=%d/%d\n", yes, len(per), len(units))
		for _, u := range units {
			if err := r.unitWiring(cr, u, umap); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *runner) unitWiring(cr, u string, umap map[string]int) error {
	links, err := r.compLinks(cr + "/" + u)
	if err != nil {
		return err
	}
	var missing []string
	for _, s := range expectSlots {
		if _, ok := links[s]; !ok {
			missing = append(missing, s)
		}
	}
	hasCtrl, err := r.hasChild(cr+"/"+u, "DefrostController")
	if err != nil {
		return err
	}
	if hasCtrl {
		dl, err := r.compLinks(cr + "/" + u + "/DefrostController")
		if err != nil {
			return err
		}
		for _, s := range []string{"interval", "duration"} {
			if _, ok := dl[s]; !ok {
				missing = append(missing, "DefrostController."+s)
			}
		}
	} else {
		missing = append(missing, "SIN DefrostController")
	}
	m := "None"
	if n, ok := umap[u]; ok {
		m = fmt.Sprint(n)
	}
	if len(missing) > 0 {
		fmt.Fprintf(r.s.Out, "  -- %s (dash evap%s) -- FALTAN: %s\n", u, m, strings.Join(missing, ", "))
	} else {
		fmt.Fprintf(r.s.Out, "  -- %s (dash evap%s) -- completo\n", u, m)
	}
	return nil
}
