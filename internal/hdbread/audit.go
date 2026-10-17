package hdbread

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"regexp"
	"sort"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/cli"
)

// auditOperations are the javax.baja.security.AuditEvent operation constants: the names an
// AuditRecord can carry even when no record in this file has a slot-path target.
var auditOperations = []string{"Changed", "Added", "Removed", "Renamed", "Reordered", "Flags Changed",
	"Facets Changed", "Recategorized", "Invoked", "Login", "Logout", "Login Failure", "Logout (Timeout)"}

// auditFields is the number of writeUTF strings BAuditRecord.doWrite writes per record.
const auditFields = 6

func isOpChar(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == ' ' || b == '(' || b == ')'
}

// opAt matches the Python _OP_RE lookahead at offset i: \x00, a length byte 1..64, a capitalised
// word run, a 1..4095 target length and a '/'. It returns the operation and whether its length
// equals the length byte.
func opAt(rec []byte, i int) (op string, counted, ok bool) {
	if i+2 >= len(rec) || rec[i] != 0 || rec[i+1] < 1 || rec[i+1] > 0x40 {
		return "", false, false
	}
	if rec[i+2] < 'A' || rec[i+2] > 'Z' {
		return "", false, false
	}
	j := i + 3
	for j < len(rec) && isOpChar(rec[j]) {
		j++
	}
	if j+2 >= len(rec) || rec[j+2] != '/' {
		return "", false, false
	}
	hi, lo := rec[j], rec[j+1]
	if !(hi == 0 && lo >= 1 || hi >= 1 && hi <= 0x0f) {
		return "", false, false
	}
	s := rec[i+2 : j]
	return string(s), len(s) == int(rec[i+1]), true
}

// countOps returns {operation: count} for every writeUTF operation followed by a '/...' target,
// the names ordered by count descending then name, and the set of anchored offsets.
func countOps(rec []byte) (map[string]int, []string, map[int]bool) {
	ops, anchored := map[string]int{}, map[int]bool{}
	for i := range rec {
		if op, counted, ok := opAt(rec, i); ok && counted {
			ops[op]++
			anchored[i] = true
		}
	}
	names := make([]string, 0, len(ops))
	for n := range ops {
		names = append(names, n)
	}
	sort.Slice(names, func(a, b int) bool {
		if ops[names[a]] != ops[names[b]] {
			return ops[names[a]] > ops[names[b]]
		}
		return names[a] < names[b]
	})
	return ops, names, anchored
}

// validModifiedUTF8 reports whether Python's decode("utf-8", "surrogatepass") accepts b after
// the C0 80 -> 00 replacement.
func validModifiedUTF8(b []byte) bool {
	b = bytes.ReplaceAll(b, []byte{0xc0, 0x80}, []byte{0})
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r == utf8.RuneError && n == 1 {
			if len(b) >= 3 && b[0] == 0xed && b[1] >= 0xa0 && b[1] <= 0xbf && b[2] >= 0x80 && b[2] <= 0xbf {
				n = 3 // a surrogate, accepted by surrogatepass
			} else {
				return false
			}
		}
		b = b[n:]
	}
	return true
}

// recordEnd frames the auditFields writeUTF strings of a record starting at i.
func recordEnd(buf []byte, i int) (int, bool) {
	for k := 0; k < auditFields; k++ {
		if i+2 > len(buf) {
			return 0, false
		}
		end := i + 2 + int(binary.BigEndian.Uint16(buf[i:i+2]))
		if end > len(buf) || !validModifiedUTF8(buf[i+2:end]) {
			return 0, false
		}
		i = end
	}
	return i, true
}

// unanchoredOps counts operations in operation position that countOps did not count: the name
// heads a full six-string audit record that does not lie inside an earlier accepted record.
func unanchoredOps(rec []byte, ops map[string]int, anchored map[int]bool) map[string]int {
	vocab := map[string]bool{}
	for op := range ops {
		vocab[op] = true
	}
	for _, op := range auditOperations {
		vocab[op] = true
	}
	type cand struct {
		pos int
		op  string
	}
	var cands []cand
	for op := range vocab {
		pat := binary.BigEndian.AppendUint16(nil, uint16(len(op)))
		pat = append(pat, op...)
		for from := 0; ; {
			k := bytes.Index(rec[from:], pat)
			if k < 0 {
				break
			}
			cands = append(cands, cand{from + k, op})
			from += k + len(pat)
		}
	}
	sort.Slice(cands, func(a, b int) bool {
		if cands[a].pos != cands[b].pos {
			return cands[a].pos < cands[b].pos
		}
		return cands[a].op < cands[b].op
	})
	extra, coveredTo := map[string]int{}, -1
	for _, c := range cands {
		if c.pos < coveredTo {
			continue
		}
		end, ok := recordEnd(rec, c.pos)
		if !ok {
			continue
		}
		coveredTo = end
		if !anchored[c.pos] {
			extra[c.op]++
		}
	}
	return extra
}

func printOps(s cli.Stdio, rec []byte, rt string, fields []string) error {
	has := false
	for _, f := range fields {
		has = has || f == "operation"
	}
	if !has {
		return exitError(fmt.Sprintf("--ops: schema has no 'operation' field (record_type=%s)", rt))
	}
	ops, names, anchored := countOps(rec)
	extra := unanchoredOps(rec, ops, anchored)
	if len(ops) == 0 && len(extra) == 0 {
		return exitError(fmt.Sprintf("--ops: 0 operation strings decoded from %d record bytes (schema has "+
			"'operation'): the record framing did not match", len(rec)))
	}
	var tail []string
	for op := range extra {
		if _, ok := ops[op]; !ok {
			tail = append(tail, op)
		}
	}
	sort.Strings(tail)
	for _, op := range append(names, tail...) {
		writeOp(s.Out, op, ops[op], extra)
	}
	return nil
}

func writeOp(w io.Writer, op string, n int, extra map[string]int) {
	suffix := ""
	if e, ok := extra[op]; ok {
		suffix = fmt.Sprintf("  (+%d with an empty, non-slot or 4096+ byte target, not counted)", e)
	}
	fmt.Fprintf(w, "op %-14s = %d%s\n", op, n, suffix)
}

// printableRuns returns the runs of 3 or more printable ASCII bytes (Python [ -~]{3,}).
func printableRuns(b []byte) []string {
	var runs []string
	start := -1
	for i := 0; i <= len(b); i++ {
		if i < len(b) && b[i] >= ' ' && b[i] <= '~' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 && i-start >= 3 {
			runs = append(runs, string(b[start:i]))
		}
		start = -1
	}
	return runs
}

var (
	roleRE  = regexp.MustCompile(`^(admin|guest|root|niagarad|station|daemon|sshd)$`)
	valueRE = regexp.MustCompile(`[A-Za-z0-9+/]{16,}`)
)

// mask redacts anything that looks like a credential value; role logins stay.
func mask(s string) string {
	if roleRE.MatchString(s) {
		return s
	}
	if valueRE.MatchString(s) {
		return "<VALUE-MASKED>"
	}
	return s
}
