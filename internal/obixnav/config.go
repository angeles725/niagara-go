package obixnav

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

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
				var err error
				if f, err = pycompat.ParseFloat(digits); err != nil {
					return 0, false, valueError(fmt.Sprintf("could not convert string to float: %s", pycompat.StrRepr(digits)))
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
	defrostRooms := siteRooms(true)
	for _, ri := range r.roomList(defrostRooms) {
		rm, ok := findRoom(ri)
		if !ok || rm.defrost == noDefrost {
			return valueError(fmt.Sprintf("no room %d (%s)", ri, roomsText(defrostRooms)))
		}
		units, err := r.listUnits(rm.coldRoom)
		if err != nil {
			return err
		}
		kind := "RESISTENCIA (−20)"
		if rm.defrost == airDefrost {
			kind = "AIRE"
		}
		fmt.Fprintf(w, "\n=== Cuarto%d (%s) — %s ===\n", ri, rm.coldRoom, kind)
		for _, u := range units {
			if err := r.unitConfig(rm, u); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *runner) unitConfig(rm room, u string) error {
	w := r.s.Out
	cr, air := rm.coldRoom, rm.defrost == airDefrost
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
	g := func(s string) string { return pycompat.OptStr(v.get(baseU + s).val) }
	gd := func(s string) *string { return v.get(baseU + "DefrostController/" + s).val }
	var flags []string
	du, dd := num(v.get(baseU+"evapDifferentialUp")), num(v.get(baseU+"evapDifferentialDown"))
	if g("hasDefrost") != "true" {
		flags = append(flags, fmt.Sprintf("hasDefrost=%s (debe true)", g("hasDefrost")))
	}
	if want := map[bool]string{true: "true", false: "false"}[air]; g("airDefrost") != want {
		flags = append(flags, fmt.Sprintf("airDefrost=%s (debe %s)", g("airDefrost"), pycompat.Repr(air)))
	}
	if du.kind == 'f' && du.f <= 0 {
		flags = append(flags, "evapDifferentialUp=0 (sin histéresis)")
	}
	if dd.kind == 'f' && dd.f <= 0 {
		flags = append(flags, "evapDifferentialDown=0 (sin histéresis)")
	}
	if hasCtrl {
		f, err := defrostFlags(gd, num(v.get(baseU+"DefrostController/resistanceTempThreshold")), rm)
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
		fmt.Fprintf(w, "      defrost: mode=%s interval=%s duration=%s term=%s thr=%s stagger=%s\n", pycompat.OptStr(gd("mode")),
			pycompat.OptStr(gd("interval")), pycompat.OptStr(gd("duration")), pycompat.OptStr(gd("terminateOnResistanceTemp")),
			pycompat.OptStr(gd("resistanceTempThreshold")), pycompat.OptStr(gd("staggerDelay")))
	}
	for _, f := range flags {
		fmt.Fprintln(w, "      ⚠ "+f)
	}
	return nil
}

func defrostFlags(gd func(string) *string, thr value, rm room) ([]string, error) {
	var flags []string
	air := rm.defrost == airDefrost
	iv, ivOK, err := rtSecs(gd("interval"))
	if err != nil {
		return nil, err
	}
	dr, drOK, err := rtSecs(gd("duration"))
	if err != nil {
		return nil, err
	}
	term := pycompat.OptStr(gd("terminateOnResistanceTemp"))
	if ivOK && iv <= 0 {
		flags = append(flags, "DefrostController.interval=0")
	}
	if drOK && dr <= 0 {
		flags = append(flags, "DefrostController.duration=0")
	}
	if ivOK && drOK && iv <= dr {
		flags = append(flags, fmt.Sprintf("interval(%s) <= duration(%s) — deshiela más seguido que su propia duración",
			pycompat.OptStr(gd("interval")), pycompat.OptStr(gd("duration"))))
	} else if ivOK && iv < 1800 {
		flags = append(flags, fmt.Sprintf("interval=%s muy corto (<30 min) — ¿typo? (los demás cuartos ~4h)",
			pycompat.OptStr(gd("interval"))))
	}
	if !air && term != "true" {
		flags = append(flags, fmt.Sprintf("terminateOnResistanceTemp=%s (Cuarto%d debe true)", term, rm.num))
	}
	if air && term == "true" {
		flags = append(flags, "terminateOnResistanceTemp=true (aire no lleva)")
	}
	if !air && thr.kind == 'f' && thr.f <= 0 {
		flags = append(flags, fmt.Sprintf("resistanceTempThreshold=0 (Cuarto%d lo necesita)", rm.num))
	}
	return flags, nil
}
