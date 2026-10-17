package obixnav

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// rooms maps a room number to its ColdRoom container (Cuarto5 is comfort, no per-evap defrost).
var rooms = map[int]string{1: "ColdRoom_1", 2: "ColdRoom_2", 3: "ColdRoom_3", 4: "ColdRoom_4", 5: "ColdRoom_5"}

type slotPair struct{ facade, rt string }

// control and defrost map per-evap facade slots ({M} = dashboard evap index) to rt slots on the
// mapped EvaporatorUnit, in the Python dict order.
var control = []slotPair{{"evap{M}Setpoint", "evapSetpoint"}, {"evap{M}DifferentialUp", "evapDifferentialUp"},
	{"evap{M}DifferentialDown", "evapDifferentialDown"}, {"evap{M}CoolOnSensorFault", "evapCoolOnSensorFault"},
	{"evap{M}StartDelay", "startDelay"}, {"evap{M}FreezeActive", "freezeActive"},
	{"evap{M}FreezeSetpoint", "freezeSetpoint"}, {"evap{M}FreezeDiffStop", "freezeDiffStop"},
	{"evap{M}FreezeDiffRestart", "freezeDiffRestart"},
	{"evap{M}DefrostInterval", "DefrostController/interval"}, {"evap{M}DefrostDuration", "DefrostController/duration"},
	{"evap{M}TerminateOnResistanceTemp", "DefrostController/terminateOnResistanceTemp"},
	{"evap{M}ResistanceTempThreshold", "DefrostController/resistanceTempThreshold"}}

// cond maps the Condensadoras facade to Programacion/CompressorControl.
var cond = []slotPair{{"highPressure", "dischargePressure"}, {"lowPressure1", "suctionPressure"},
	{"lowPressure2", "suctionPressure2"}, {"amps1", "amps1"}, {"amps2", "amps2"}, {"amps3", "amps3"},
	{"hours1", "condenser1Hours"}, {"hours2", "condenser2Hours"}, {"hours3", "condenser3Hours"},
	{"comp1State", "condenser1"}, {"comp2State", "condenser2"}, {"comp3State", "condenser3"}}

var unitRE = regexp.MustCompile(`href="(?:[^"]*/)?(EvaporatorUnit[0-9_]*)/"`)

// listUnits returns the sorted EvaporatorUnit children of a ColdRoom, via one GET.
func (r *runner) listUnits(coldroom string) ([]string, error) {
	xml, err := r.c.get("/Programacion/"+coldroom, r.s.Err)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range unitRE.FindAllStringSubmatch(xml, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out, nil
}

// hasChild reports whether /Programacion/<path> has a direct <child>/ href.
func (r *runner) hasChild(path, child string) (bool, error) {
	xml, err := r.c.get("/Programacion/"+path, r.s.Err)
	if err != nil {
		return false, err
	}
	return regexp.MustCompile(`href="(?:[^"]*/)?` + regexp.QuoteMeta(child) + `/"`).MatchString(xml), nil
}

// valueError is a Python ValueError that ends the run (Python: a traceback, exit 1).
type valueError string

func (e valueError) Error() string { return string(e) }

// detectMap maps dashboard evapM (facade evapTempM) to the physical unit whose coilTemp equals it.
func (r *runner) detectMap(coldroom string, units []string) ([]string, error) {
	n := len(units)
	var roomI int
	fmt.Sscan(coldroom[strings.Index(coldroom, "_")+1:], &roomI)
	var ords []string
	for m := 1; m <= n; m++ {
		ords = append(ords, fmt.Sprintf("/Services/DashboardService/Cuarto%d/evapTemp%d", roomI, m))
	}
	for _, u := range units {
		ords = append(ords, fmt.Sprintf("/Programacion/%s/%s/coilTemp", coldroom, u))
	}
	vals, err := r.c.batch(ords)
	if err != nil {
		return nil, err
	}
	fmap := make([]string, n) // index m-1; "" = no match (Python None)
	for m := 1; m <= n; m++ {
		fv := num(vals.get(strings.ReplaceAll(ords[m-1], "/config", "")))
		for _, u := range units {
			uv := num(vals.get(fmt.Sprintf("/Programacion/%s/%s/coilTemp", coldroom, u)))
			if fv.kind == 'n' || uv.kind == 'n' {
				continue
			}
			for _, v := range []value{fv, uv} {
				if v.kind == 's' {
					return nil, valueError(fmt.Sprintf("could not convert string to float: %s", pycompat.StrRepr(v.s)))
				}
			}
			if math.Abs(fv.f-uv.f) < 1e-6 {
				fmap[m-1] = u
				break
			}
		}
	}
	return fmap, nil
}

func crossed(fmap []string) bool {
	for i, u := range fmap {
		if u != "" && u != fmt.Sprintf("EvaporatorUnit_%d", i+1) && u != "EvaporatorUnit" {
			return true
		}
	}
	return false
}

func mappingWord(fmap []string) string {
	if crossed(fmap) {
		return "CRUZADO"
	}
	return "directo"
}

func orNone(s string) string {
	if s == "" {
		return "None"
	}
	return s
}

// defrostLayout returns whether the room has a DefrostController and which units carry their own.
func (r *runner) defrostLayout(cr string, units []string) (bool, []string, error) {
	roomCtrl, err := r.hasChild(cr, "DefrostController")
	if err != nil {
		return false, nil, err
	}
	per := []string{}
	for _, u := range units {
		ok, err := r.hasChild(cr+"/"+u, "DefrostController")
		if err != nil {
			return false, nil, err
		}
		if ok {
			per = append(per, u)
		}
	}
	return roomCtrl, per, nil
}

func (r *runner) roomList(defaults []int) []int {
	if r.o.Room != 0 {
		return []int{r.o.Room}
	}
	return defaults
}

func (r *runner) coldRoom(ri int) (string, error) {
	cr, ok := rooms[ri]
	if !ok {
		return "", valueError(fmt.Sprintf("no room %d (rooms 1-5)", ri))
	}
	return cr, nil
}

func (r *runner) cmdMap() error {
	w := r.s.Out
	for _, ri := range r.roomList([]int{1, 2, 3, 4, 5}) {
		cr, err := r.coldRoom(ri)
		if err != nil {
			return err
		}
		units, err := r.listUnits(cr)
		if err != nil {
			return err
		}
		if len(units) == 0 {
			fmt.Fprintf(w, "Cuarto%d (%s): sin EvaporatorUnit legibles\n", ri, cr)
			continue
		}
		fmap, err := r.detectMap(cr, units)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "\n=== Cuarto%d (%s) — %d evaporador(es) — mapeo: %s ===\n", ri, cr, len(units), mappingWord(fmap))
		for i, u := range fmap {
			if u == "" {
				u = "??"
			}
			fmt.Fprintf(w, "  dash evap%d  ->  %s\n", i+1, u)
		}
		roomCtrl, per, err := r.defrostLayout(cr, units)
		if err != nil {
			return err
		}
		present := "no"
		if roomCtrl {
			present = "PRESENTE"
		}
		fmt.Fprintf(w, "  defrost: room-level DefrostController=%s | per-evap=%d/%d %s\n", present, len(per),
			len(units), pycompat.Repr(per))
		if err := r.compareRoom(w, ri, cr, fmap); err != nil {
			return err
		}
	}
	return nil
}

// compareRoom batch-reads every control/defrost slot pair under the mapping: OK, DIFF or MISSING.
func (r *runner) compareRoom(w io.Writer, ri int, cr string, fmap []string) error {
	type pair struct {
		m            int
		fslot, rslot string
		fOrd, rOrd   string
	}
	var ords []string
	var meta []pair
	for i, u := range fmap {
		if u == "" {
			continue
		}
		for _, sp := range control {
			fslot := strings.ReplaceAll(sp.facade, "{M}", fmt.Sprint(i+1))
			p := pair{i + 1, fslot, sp.rt, fmt.Sprintf("/Services/DashboardService/Cuarto%d/%s", ri, fslot),
				fmt.Sprintf("/Programacion/%s/%s/%s", cr, u, sp.rt)}
			ords = append(ords, p.fOrd, p.rOrd)
			meta = append(meta, p)
		}
	}
	vals, err := r.c.batch(ords)
	if err != nil {
		return err
	}
	var miss []string
	for _, p := range meta {
		fv, rv := vals.get(p.fOrd), vals.get(p.rOrd)
		switch {
		case fv.tag == "err" && rv.tag == "err":
			continue // slot family absent (e.g. no resistance defrost)
		case rv.tag == "err":
			miss = append(miss, fmt.Sprintf("  MISSING rt   %s (facade=%s, rt slot ausente)", p.rslot, pycompat.OptStr(fv.val)))
			continue
		case fv.tag == "err":
			miss = append(miss, "  MISSING fac  "+p.fslot)
			continue
		}
		fn, rn := num(fv), num(rv)
		same := fn.str() == rn.str() || fn.kind == 'f' && rn.kind == 'f' && math.Abs(fn.f-rn.f) < 1e-6
		if !same {
			miss = append(miss, fmt.Sprintf("  DIFF  evap%d %s=%s  !=  rt %s=%s", p.m, p.fslot, pycompat.OptStr(fv.val),
				p.rslot, pycompat.OptStr(rv.val)))
		}
	}
	if len(miss) > 0 {
		fmt.Fprintln(w, " Diferencias/faltantes:")
		fmt.Fprintln(w, strings.Join(miss, "\n"))
	} else {
		fmt.Fprintln(w, " Todos los slots de control/defrost COINCIDEN facade<->rt.")
	}
	return nil
}

func (r *runner) cmdCond() error {
	var ords []string
	for _, p := range cond {
		ords = append(ords, "/Services/DashboardService/Condensadoras/"+p.facade)
	}
	for _, p := range cond {
		ords = append(ords, "/Programacion/CompressorControl/"+p.rt)
	}
	vals, err := r.c.batch(ords)
	if err != nil {
		return err
	}
	fmt.Fprintln(r.s.Out, "=== Condensadoras facade vs CompressorControl ===")
	for _, p := range cond {
		fv := vals.get("/Services/DashboardService/Condensadoras/" + p.facade)
		rv := vals.get("/Programacion/CompressorControl/" + p.rt)
		tag := "DIFF"
		sameVal := fv.val == nil && rv.val == nil || fv.val != nil && rv.val != nil && *fv.val == *rv.val
		switch {
		case sameVal && fv.tag != "err":
			tag = "OK"
		case fv.tag == "err" || rv.tag == "err":
			tag = "MISSING"
		}
		fmt.Fprintf(r.s.Out, "  %-8s %-14s facade=%s  rt=%s  [%s]\n", tag, p.facade, pycompat.OptStr(fv.val), pycompat.OptStr(rv.val), p.rt)
	}
	return nil
}
