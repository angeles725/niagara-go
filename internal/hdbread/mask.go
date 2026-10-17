package hdbread

import (
	"regexp"
)

// userMasked replaces a printable run that carries a user identity under --strings --mask.
const userMasked = "<USER-MASKED>"

// valueMasked replaces a run that looks like a credential value (Python mask()).
const valueMasked = "<VALUE-MASKED>"

// loginOps are the AuditEvent operations whose other fields describe the session's user.
var loginOps = map[string]bool{"Login": true, "Login Failure": true, "Logout": true, "Logout (Timeout)": true}

// Field indexes of BAuditRecord.doWrite: operation, target, slotName, oldValue, value, userName.
const (
	fieldTarget   = 1
	fieldUserName = 5
)

// identityRanges returns the content [start, end) of every user-identity field of the framed
// audit records in rec (intended difference from Python, whose mask() let user names through):
//   - the userName (6th string) of every record;
//   - in a login-family record (loginOps), also slotName, oldValue and value, and the target
//     unless it is a slot path (starts with '/').
//
// Empty fields are skipped. Records are framed exactly as unanchoredOps frames them.
func identityRanges(rec []byte) [][2]int {
	ops, _, _ := countOps(rec)
	vocab := map[string]bool{}
	for op := range ops {
		vocab[op] = true
	}
	var out [][2]int
	for _, r := range frameRecords(rec, vocab) {
		for k := fieldTarget; k <= fieldUserName; k++ {
			f := r.fields[k]
			if f[0] == f[1] {
				continue
			}
			isID := k == fieldUserName
			if loginOps[r.op] {
				isID = k != fieldTarget || rec[f[0]] != '/'
			}
			if isID {
				out = append(out, f)
			}
		}
	}
	return out
}

// printableRun is one run of 3+ printable ASCII bytes at rec[start:end].
type printableRun struct{ start, end int }

// printableRuns returns the runs of 3 or more printable ASCII bytes (Python [ -~]{3,}).
func printableRuns(b []byte) []printableRun {
	var runs []printableRun
	start := -1
	for i := 0; i <= len(b); i++ {
		if i < len(b) && b[i] >= ' ' && b[i] <= '~' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 && i-start >= 3 {
			runs = append(runs, printableRun{start, i})
		}
		start = -1
	}
	return runs
}

// stringsLines is the --strings output: every printable run; with doMask a run overlapping a
// user-identity field prints userMasked, any other run goes through mask.
func stringsLines(rec []byte, doMask bool) []string {
	var ids [][2]int
	if doMask {
		ids = identityRanges(rec)
	}
	var lines []string
	for _, r := range printableRuns(rec) {
		s := string(rec[r.start:r.end])
		if doMask {
			s = mask(s)
			for _, id := range ids {
				if r.start < id[1] && id[0] < r.end {
					s = userMasked
					break
				}
			}
		}
		lines = append(lines, s)
	}
	return lines
}

var (
	roleRE  = regexp.MustCompile(`^(admin|guest|root|niagarad|station|daemon|sshd)$`)
	valueRE = regexp.MustCompile(`[A-Za-z0-9+/]{16,}`)
)

// mask redacts anything that looks like a credential value; role logins stay (Python mask()).
func mask(s string) string {
	if roleRE.MatchString(s) {
		return s
	}
	if valueRE.MatchString(s) {
		return valueMasked
	}
	return s
}
