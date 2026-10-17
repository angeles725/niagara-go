package hdbread

import "sort"

// userMasked replaces a printable run that carries a user identity under --strings --mask.
const userMasked = "<USER-MASKED>"

// valueMasked replaces every other printable run under --strings --mask that is not an
// operation constant at a framed operation field; under --mask it also stands for the --ops
// names that are not auditOperations constants and for the --schema history id.
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
	ids      [][2]int       // identity field contents [start, end) of the framed records
	idIndex  spans          // ids, indexed for the overlap test of maskRun
	opStarts map[int]string // first content byte -> operation of each trusted framed record
}

// spans indexes half-open ranges [start, end) so "does any range start before hi and end after
// lo" costs O(log n): starts sorted, maxEnd[i] the largest end among the first i+1 of them.
type spans struct {
	starts, maxEnd []int
}

func newSpans(ranges [][2]int) spans {
	sorted := append([][2]int(nil), ranges...)
	sort.Slice(sorted, func(a, b int) bool { return sorted[a][0] < sorted[b][0] })
	sp := spans{make([]int, len(sorted)), make([]int, len(sorted))}
	for i, r := range sorted {
		sp.starts[i], sp.maxEnd[i] = r[0], r[1]
		if i > 0 && sp.maxEnd[i-1] > r[1] {
			sp.maxEnd[i] = sp.maxEnd[i-1]
		}
	}
	return sp
}

// any reports whether some range r has r[0] < hi and lo < r[1].
func (sp spans) any(lo, hi int) bool {
	n := sort.SearchInts(sp.starts, hi) // ranges [0, n) start before hi
	return n > 0 && sp.maxEnd[n-1] > lo
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

// planMask frames the audit records of rec exactly as unanchoredOps does (countOps names stay
// in the framing vocabulary). Identity fields (intended difference from Python, whose mask()
// let user names through) are:
//   - the userName (6th string) of every framed record;
//   - in a login-family record (loginOps), also slotName, oldValue and value, and the target
//     unless it starts with '/'.
//
// Empty fields are skipped. A framed record's operation field is trusted (opStarts) unless the
// record starts strictly inside the range [pos, declaredEnd) of an earlier candidate that failed
// to frame: a damaged record must not turn its own userName into an operation position.
func planMask(rec []byte) maskPlan {
	ops, _, _ := countOps(rec)
	vocab := map[string]bool{}
	for op := range ops {
		vocab[op] = true
	}
	framed, failed := frameRecords(rec, vocab)
	inFailed := newSpans(failed)
	p := maskPlan{opStarts: map[int]string{}}
	for _, r := range framed {
		if !inFailed.any(r.pos, r.pos) { // no failed f with f[0] < r.pos < f[1]
			p.opStarts[r.fields[0][0]] = r.op
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
	p.idIndex = newSpans(p.ids)
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

// stringsLines is the --strings output: every printable run, or with doMask its maskRun form.
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

// maskRun is the --mask rule v5 (fail-closed, every section, no audit detection) for the run r
// whose text is s:
//  1. a run overlapping an identity field of a framed record prints userMasked;
//  2. else a run that equals an auditOperations constant, starts at the first content byte of a
//     trusted framed record's operation field (planMask) and equals that operation prints verbatim;
//  3. else the run prints valueMasked.
//
// No role-account exception, no slot paths: nothing else can print verbatim.
func maskRun(p maskPlan, r printableRun, s string) string {
	if p.idIndex.any(r.start, r.end) { // some id with r.start < id[1] && id[0] < r.end
		return userMasked
	}
	if op, ok := p.opStarts[r.start]; ok && s == op && isAuditOperation(s) {
		return s
	}
	return valueMasked
}
