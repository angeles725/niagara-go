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

// maskPlan is what --mask needs from the framed audit records of a record section.
type maskPlan struct {
	ids     [][2]int        // identity field contents [start, end)
	records [][2]int        // framed record extents [start, end)
	vocab   map[string]bool // candidate operation names: countOps-decoded plus auditOperations
}

// planMask frames the audit records of rec exactly as unanchoredOps does. Identity fields
// (intended difference from Python, whose mask() let user names through) are:
//   - the userName (6th string) of every record;
//   - in a login-family record (loginOps), also slotName, oldValue and value, and the target
//     unless it is a slot path (starts with '/').
//
// Empty fields are skipped.
func planMask(rec []byte) maskPlan {
	ops, _, _ := countOps(rec)
	p := maskPlan{vocab: map[string]bool{}}
	for op := range ops {
		p.vocab[op] = true
	}
	for _, r := range frameRecords(rec, p.vocab) {
		p.records = append(p.records, [2]int{r.pos, r.fields[auditFields-1][1]})
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
				p.ids = append(p.ids, f)
			}
		}
	}
	for _, op := range auditOperations {
		p.vocab[op] = true
	}
	return p
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

// stringsLines is the --strings output: every printable run. With doMask (rule v2, fail-closed):
//   - a run overlapping a user-identity field prints userMasked;
//   - in a section with no framed audit record, any other run goes through mask (old rule);
//   - otherwise a run lying entirely inside a framed record goes through mask, and a run that
//     does not prints valueMasked unless it is exactly a candidate operation name (planMask
//     vocab) or exactly a role account (roleRE), so a record the framing rejected (truncated,
//     invalid modified UTF-8, operation outside the vocabulary) cannot leak its user name.
func stringsLines(rec []byte, doMask bool) []string {
	var p maskPlan
	if doMask {
		p = planMask(rec)
	}
	var lines []string
	for _, r := range printableRuns(rec) {
		s := string(rec[r.start:r.end])
		if doMask {
			s = maskRun(p, r, s)
		}
		lines = append(lines, s)
	}
	return lines
}

// maskRun applies the --mask rule v2 of stringsLines to the run r whose text is s.
func maskRun(p maskPlan, r printableRun, s string) string {
	for _, id := range p.ids {
		if r.start < id[1] && id[0] < r.end {
			return userMasked
		}
	}
	if len(p.records) == 0 {
		return mask(s)
	}
	for _, rr := range p.records {
		if rr[0] <= r.start && r.end <= rr[1] {
			return mask(s)
		}
	}
	if p.vocab[s] || roleRE.MatchString(s) {
		return s
	}
	return valueMasked
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
