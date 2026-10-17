package obixnav

import (
	"fmt"
	"strings"
)

// roomAir: true for air defrost rooms; Cuarto3 is resistance (-20 C).
var roomAir = map[int]bool{1: true, 2: true, 3: false, 4: true}

// rtSecs converts PT8H0S / PT30M0S / PT5S to seconds; ok is false when v is not a PT duration.
func rtSecs(v *string) (float64, bool, error) {
	if v == nil || !strings.HasPrefix(*v, "PT") {
		return 0, false, nil
	}
	tot, digits := 0.0, ""
	unit := map[rune]float64{'H': 3600, 'M': 60, 'S': 1}
	for _, ch := range (*v)[2:] {
		switch {
		case ch >= '0' && ch <= '9' || ch == '.':
			digits += string(ch)
		case unit[ch] != 0:
			f := 0.0
			if digits != "" {
				var ok bool
				if f, ok = pyFloat(digits); !ok {
					return 0, false, valueError(fmt.Sprintf("could not convert string to float: %s", pyStrRepr(digits)))
				}
			}
			tot += f * unit[ch]
			digits = ""
		}
	}
	return tot, true, nil
}

var (
	unitSlots    = []string{"evapSetpoint", "evapDifferentialUp", "evapDifferentialDown", "evapCoolOnSensorFault", "freezeProtect", "freezeSetpoint", "freezeDiffStop", "freezeDiffRestart", "hasDefrost", "airDefrost", "fanRunMode", "startDelay"}
	defrostSlots = []string{"mode", "interval", "duration", "terminateOnResistanceTemp", "resistanceTempThreshold", "staggerDelay"}
)

func (r *runner) cmdConfig() error {
	w := r.s.Out
	for _, ri := range r.roomList([]int{1, 2, 3, 4}) {
		cr, err := r.coldRoom(ri)
		air, known := roomAir[ri]
		if err != nil || !known {
			return valueError(fmt.Sprintf("no room %d (rooms 1-4)", ri))
		}
		units, err := r.listUnits(cr)
		if err != nil {
			return err
		}
		kind := "RESISTENCIA (−20)"
		if air {
			kind = "AIRE"
		}
		fmt.Fprintf(w, "\n=== Cuarto%d (%s) — %s ===\n", ri, cr, kind)
		for _, u := range units {
			if err := r.unitConfig(cr, u, air); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *runner) unitConfig(cr, u string, air bool) error {
	w := r.s.Out
	baseU := fmt.Sprintf("/Programacion/%s/%s/", cr, u)
	hasCtrl, err := r.hasChild(cr+"/"+u, "DefrostController")
	if err != nil {
		return err
	}
	var ords []string
	for _, s := range unitSlots {
		ords = append(ords, baseU+s)
	}
	if hasCtrl {
		for _, s := range defrostSlots {
			ords = append(ords, baseU+"DefrostController/"+s)
		}
	}
	v, err := r.c.batch(ords)
	if err != nil {
		return err
	}
	g := func(s string) string { return optStr(v.get(baseU + s).val) }
	gd := func(s string) *string { return v.get(baseU + "DefrostController/" + s).val }
	var flags []string
	du, dd := num(v.get(baseU+"evapDifferentialUp")), num(v.get(baseU+"evapDifferentialDown"))
	if g("hasDefrost") != "true" {
		flags = append(flags, fmt.Sprintf("hasDefrost=%s (debe true)", g("hasDefrost")))
	}
	if want := map[bool]string{true: "true", false: "false"}[air]; g("airDefrost") != want {
		flags = append(flags, fmt.Sprintf("airDefrost=%s (debe %s)", g("airDefrost"), pyBool(air)))
	}
	if du.kind == 'f' && du.f <= 0 {
		flags = append(flags, "evapDifferentialUp=0 (sin histéresis)")
	}
	if dd.kind == 'f' && dd.f <= 0 {
		flags = append(flags, "evapDifferentialDown=0 (sin histéresis)")
	}
	if hasCtrl {
		f, err := defrostFlags(gd, num(v.get(baseU+"DefrostController/resistanceTempThreshold")), air)
		if err != nil {
			return err
		}
		flags = append(flags, f...)
	} else {
		flags = append(flags, "SIN DefrostController per-evap")
	}
	fmt.Fprintf(w, "  -- %s --  sp=%s diffUp=%s diffDn=%s cool=%s | freeze prot=%s sp=%s stop=%s rest=%s | "+
		"hasDef=%s air=%s fan=%s startDelay=%s\n", u, g("evapSetpoint"), g("evapDifferentialUp"),
		g("evapDifferentialDown"), g("evapCoolOnSensorFault"), g("freezeProtect"), g("freezeSetpoint"),
		g("freezeDiffStop"), g("freezeDiffRestart"), g("hasDefrost"), g("airDefrost"), g("fanRunMode"), g("startDelay"))
	if hasCtrl {
		fmt.Fprintf(w, "      defrost: mode=%s interval=%s duration=%s term=%s thr=%s stagger=%s\n", optStr(gd("mode")),
			optStr(gd("interval")), optStr(gd("duration")), optStr(gd("terminateOnResistanceTemp")),
			optStr(gd("resistanceTempThreshold")), optStr(gd("staggerDelay")))
	}
	for _, f := range flags {
		fmt.Fprintln(w, "      ⚠ "+f)
	}
	return nil
}

func defrostFlags(gd func(string) *string, thr value, air bool) ([]string, error) {
	var flags []string
	iv, ivOK, err := rtSecs(gd("interval"))
	if err != nil {
		return nil, err
	}
	dr, drOK, err := rtSecs(gd("duration"))
	if err != nil {
		return nil, err
	}
	term := optStr(gd("terminateOnResistanceTemp"))
	if ivOK && iv <= 0 {
		flags = append(flags, "DefrostController.interval=0")
	}
	if drOK && dr <= 0 {
		flags = append(flags, "DefrostController.duration=0")
	}
	if ivOK && drOK && iv <= dr {
		flags = append(flags, fmt.Sprintf("interval(%s) <= duration(%s) — deshiela más seguido que su propia duración",
			optStr(gd("interval")), optStr(gd("duration"))))
	} else if ivOK && iv < 1800 {
		flags = append(flags, fmt.Sprintf("interval=%s muy corto (<30 min) — ¿typo? (los demás cuartos ~4h)",
			optStr(gd("interval"))))
	}
	if !air && term != "true" {
		flags = append(flags, fmt.Sprintf("terminateOnResistanceTemp=%s (Cuarto3 debe true)", term))
	}
	if air && term == "true" {
		flags = append(flags, "terminateOnResistanceTemp=true (aire no lleva)")
	}
	if !air && thr.kind == 'f' && thr.f <= 0 {
		flags = append(flags, "resistanceTempThreshold=0 (Cuarto3 lo necesita)")
	}
	return flags, nil
}
