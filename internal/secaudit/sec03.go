package secaudit

import (
	"fmt"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

const sec03Run = `run: icacls <home>\security`

// sec03 checks the security/ ACLs with icacls (a home under a WSL /mnt/<drive> mount only).
// Any listing that is missing, empty or only partly understood is a SKIP, never a PASS. The error
// is an interrupt that came while icacls ran.
func (e *env) sec03(home string, rep *report) error {
	secdir := pycompat.Join(home, "security")
	if !pycompat.IsDir(secdir) {
		skipRow(rep, "SEC-03", "crit", "security/ filesystem ACLs", "no security/ directory", "Admin/SYSTEM only", "B316/B113", sec03Run)
		return nil
	}
	skip := func(observed string) {
		rep.add("SEC-03", "crit", "security/ filesystem ACLs", observed, "Admin/SYSTEM only", "SKIP", "B316/B113", sec03Run)
	}
	winpath := e.winpath(secdir)
	icacls := ""
	if winpath != "" {
		icacls = e.findIcacls()
	}
	switch {
	case winpath != "" && icacls == "":
		skip("not checked: icacls.exe not on PATH nor at " + strings.Join(icaclsPaths, " / "))
	case icacls != "":
		listing, why, err := e.runIcacls(icacls, winpath)
		if err != nil {
			return err
		}
		if why != "" {
			skip(fmt.Sprintf("not checked: %s %s", icacls, why))
			return nil
		}
		aces, unparsed := parseListing(listing, winpath)
		switch {
		case len(aces) == 0:
			skip(fmt.Sprintf("not checked: %s printed no ACE lines: %s", icacls, pycompat.StrRepr(pycompat.Cut(pycompat.Strip(listing), 80))))
		case len(unparsed) > 0:
			skip(fmt.Sprintf("not checked: %d ACE line(s) not understood: %s", len(unparsed), pycompat.StrRepr(pycompat.Cut(unparsed[0], 80))))
		default:
			loose := looseACEs(aces)
			rep.add("SEC-03", "crit", "security/ filesystem ACLs",
				pick(len(loose) > 0, "Modify/Write for "+strings.Join(loose, ", "), "restricted"),
				"Admin/SYSTEM only", verdict(len(loose) > 0), "B316/B113",
				"any authenticated user can plant a trust anchor / license")
		}
	default:
		rep.add("SEC-03", "crit", "security/ filesystem ACLs", "not checked (need icacls)",
			"Admin/SYSTEM only", "MANUAL", "B316/B113", sec03Run)
	}
	return nil
}
