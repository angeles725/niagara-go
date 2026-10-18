package check

import "strings"

// checkLab normalises a --lab spelling of the check command: 3, 03, L3 -> "3"; B, LB -> "B";
// ra1 -> "RA1"; exam -> "EXAM".
func checkLab(name string) string {
	n := strings.ToUpper(strings.TrimSpace(name))
	if len(n) > 1 && n[0] == 'L' {
		n = n[1:]
	}
	if n != "" && strings.Trim(n, "0123456789") == "" {
		n = strings.TrimLeft(n, "0")
	}
	return n
}

// Labs lists the labs that have checks, in spec order.
func Labs() []string {
	var labs []string
	seen := map[string]bool{}
	for _, ck := range Spec {
		if !seen[ck.Lab] {
			seen[ck.Lab] = true
			labs = append(labs, ck.Lab)
		}
	}
	return labs
}

// Select returns the spec rows of the given labs (every lab of the TrainingStation when none:
// RA1, RA2 and EXAM run only when named), or the first lab that has no checks.
func Select(labs []string) ([]*Check, string) {
	want := map[string]bool{}
	for _, l := range labs {
		n := checkLab(l)
		found := false
		for _, ck := range Spec {
			found = found || ck.Lab == n
		}
		if !found {
			return nil, l
		}
		want[n] = true
	}
	var out []*Check
	for i := range Spec {
		if want[Spec[i].Lab] || (len(want) == 0 && Role(Spec[i].Lab) == RoleTraining) {
			out = append(out, &Spec[i])
		}
	}
	return out, ""
}
