package hdbread

import (
	"bytes"
	"encoding/binary"
	"regexp"
	"strings"
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

// maskPlan is what --mask needs to know about a record section.
type maskPlan struct {
	audit bool            // an audit section (auditSchema or hasAuditOperation)
	ids   [][2]int        // identity field contents [start, end) of the framed records
	paths [][2]int        // framed target contents [start, end) that start with '/'
	ops   map[string]bool // operation names: auditOperations plus countOps-decoded
}

// auditSchema reports whether the file schema names an audit history: an 'operation' field
// (what --ops requires) or an AuditRecord record type.
func auditSchema(recordType string, fields []string) bool {
	for _, f := range fields {
		if f == "operation" {
			return true
		}
	}
	return strings.HasSuffix(recordType, "AuditRecord")
}

// hasAuditOperation reports whether rec holds an auditOperations name written as writeUTF (a
// 2-byte big-endian length, then the name), framed or not.
func hasAuditOperation(rec []byte) bool {
	for _, op := range auditOperations {
		if bytes.Contains(rec, append(binary.BigEndian.AppendUint16(nil, uint16(len(op))), op...)) {
			return true
		}
	}
	return false
}

// planMask frames the audit records of rec exactly as unanchoredOps does. Identity fields
// (intended difference from Python, whose mask() let user names through) are:
//   - the userName (6th string) of every record;
//   - in a login-family record (loginOps), also slotName, oldValue and value, and the target
//     unless it is a slot path (starts with '/').
//
// Empty fields are skipped.
func planMask(rec []byte, schemaIsAudit bool) maskPlan {
	ops, _, _ := countOps(rec)
	p := maskPlan{audit: schemaIsAudit || hasAuditOperation(rec), ops: map[string]bool{}}
	for op := range ops {
		p.ops[op] = true
	}
	for _, r := range frameRecords(rec, p.ops) {
		if t := r.fields[fieldTarget]; t[0] < t[1] && rec[t[0]] == '/' {
			p.paths = append(p.paths, t)
		}
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
		p.ops[op] = true
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

// stringsLines is the --strings output: every printable run. With doMask, a section that is not
// an audit section keeps Python's mask(). In an audit section (rule v3, an allowlist) a run
//   - overlapping an identity field of a framed record prints userMasked;
//   - else prints verbatim only if it is exactly an operation name (maskPlan ops) or it starts
//     at the first byte of a framed record's slot-path target and ends inside that target;
//   - else prints valueMasked.
//
// No role-account exception applies in an audit section. A slot path must start at a framed
// target because a writeUTF length byte 0x2f reads as '/' in front of any 47-byte string.
func stringsLines(rec []byte, doMask, schemaIsAudit bool) []string {
	var p maskPlan
	if doMask {
		p = planMask(rec, schemaIsAudit)
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

// maskRun applies the --mask rule of stringsLines to the run r whose text is s.
func maskRun(p maskPlan, r printableRun, s string) string {
	if !p.audit {
		return mask(s)
	}
	for _, id := range p.ids {
		if r.start < id[1] && id[0] < r.end {
			return userMasked
		}
	}
	if p.ops[s] {
		return s
	}
	for _, t := range p.paths {
		if r.start == t[0] && r.end <= t[1] {
			return s
		}
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
