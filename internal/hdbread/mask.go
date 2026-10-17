package hdbread

import (
	"bytes"
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
	audit    bool         // an audit section (auditSchema or hasAuditOperation)
	ids      [][2]int     // identity field contents [start, end) of the framed records
	idValues []string     // the ASCII-lowercased identity field values
	paths    [][2]int     // framed target contents [start, end) that start with '/'
	opStarts map[int]bool // first content byte of each framed record's operation field
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

// hasAuditOperation reports whether rec holds an auditOperations name anywhere, as a plain byte
// substring (rule v4: damaged length prefixes must not turn an audit section into a plain one).
func hasAuditOperation(rec []byte) bool {
	for _, op := range auditOperations {
		if bytes.Contains(rec, []byte(op)) {
			return true
		}
	}
	return false
}

// isAuditOperation reports whether s is one of the auditOperations constants.
func isAuditOperation(s string) bool {
	for _, op := range auditOperations {
		if s == op {
			return true
		}
	}
	return false
}

// asciiLower lowercases A-Z only (Python bytes.lower()).
func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// planMask frames the audit records of rec exactly as unanchoredOps does (countOps names stay
// in the framing vocabulary). Identity fields (intended difference from Python, whose mask()
// let user names through) are:
//   - the userName (6th string) of every record;
//   - in a login-family record (loginOps), also slotName, oldValue and value, and the target
//     unless it is a slot path (starts with '/').
//
// Empty fields are skipped.
func planMask(rec []byte, schemaIsAudit bool) maskPlan {
	ops, _, _ := countOps(rec)
	vocab := map[string]bool{}
	for op := range ops {
		vocab[op] = true
	}
	p := maskPlan{audit: schemaIsAudit || hasAuditOperation(rec), opStarts: map[int]bool{}}
	for _, r := range frameRecords(rec, vocab) {
		p.opStarts[r.fields[0][0]] = true
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
				p.idValues = append(p.idValues, asciiLower(string(rec[f[0]:f[1]])))
			}
		}
	}
	return p
}

// pathNamesIdentity reports whether the slot path s must not print (rule v4): one of its
// '/'-separated segments equals an identity value (ASCII case-insensitive), or a "UserService"
// segment (ASCII case-insensitive) has any segment after it.
func pathNamesIdentity(p maskPlan, s string) bool {
	segs := strings.Split(asciiLower(s), "/")
	for k, seg := range segs {
		if seg == "userservice" && k < len(segs)-1 {
			return true
		}
		for _, id := range p.idValues {
			if seg == id {
				return true
			}
		}
	}
	return false
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
// an audit section keeps Python's mask(). In an audit section (rule v4, an allowlist) a run
//   - overlapping an identity field of a framed record prints userMasked;
//   - else prints verbatim only if it equals an auditOperations constant and starts at the
//     first content byte of a framed record's operation field, or it starts at the first byte
//     of a framed record's slot-path target, ends inside that target and no segment of it names
//     an identity (pathNamesIdentity);
//   - else prints valueMasked.
//
// No role-account exception applies in an audit section. Names decoded by countOps only frame
// records (and feed --ops); they are never allowlisted, since any capitalised writeUTF string
// followed by a '/' string decodes as one. A slot path must start at a framed target because a
// writeUTF length byte 0x2f reads as '/' in front of any 47-byte string.
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
	if p.opStarts[r.start] && isAuditOperation(s) {
		return s
	}
	for _, t := range p.paths {
		if r.start == t[0] && r.end <= t[1] {
			if pathNamesIdentity(p, s) {
				return valueMasked
			}
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
