package check

import "strings"

// checkLab normalises a --lab spelling of the check command: 3, 03, L3 -> "3"; B, LB -> "B".
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

// checkLabs lists the labs that have checks, in spec order.
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

// selectChecks returns the spec rows of the given labs (all when none), or the first lab
// that has no checks.
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
		if len(want) == 0 || want[Spec[i].Lab] {
			out = append(out, &Spec[i])
		}
	}
	return out, ""
}
