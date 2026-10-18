package check

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/angeles725/niagara-go/internal/course/key"
	"github.com/angeles725/niagara-go/internal/hdbread"
)

// The history kinds (audit, log, trend): records of the station's history files,
// <station>/history/<device>/<segment>/<name>.hdb, decoded by internal/hdbread. They read what the
// station recorded while it ran (who changed, added or invoked what; its log lines; the samples of
// a trend), which the bog does not keep.

// HistMatch selects the records of a history check. The audit and log fields are Go regexps matched
// against the record field ("" = any record).
type HistMatch struct {
	Operation, Target, SlotName, OldValue, Value string // audit
	LogName, Level, Message                      string // log
	// Min, Max bound every value of a trend (unchecked when Max <= Min); a boolean or enum value
	// counts as 0/1 or its ordinal.
	Min, Max float64
}

// recordTypes are the hdbread record types each history kind reads.
var recordTypes = map[string][]string{
	kindAudit: {"history:AuditRecord"},
	kindLog:   {"history:LogRecord"},
	kindTrend: {"history:NumericTrendRecord", "history:BooleanTrendRecord", "history:EnumTrendRecord"},
}

// histField is one audit or log field and the regexp it must match.
type histField struct {
	name string
	re   *regexp.Regexp
}

// histFields compiles the non-empty patterns of the kind's fields, in record order.
func histFields(kind string, m *HistMatch) ([]histField, error) {
	pairs := [][2]string{{"operation", m.Operation}, {"target", m.Target}, {"slotName", m.SlotName},
		{"oldValue", m.OldValue}, {"value", m.Value}}
	if kind == kindLog {
		pairs = [][2]string{{"logName", m.LogName}, {"level", m.Level}, {"message", m.Message}}
	}
	var out []histField
	for _, p := range pairs {
		if p[1] == "" {
			continue
		}
		re, err := regexp.Compile(p[1])
		if err != nil {
			return nil, fmt.Errorf("bad pattern %s: %v", p[1], err)
		}
		out = append(out, histField{p[0], re})
	}
	return out, nil
}

// fieldValue is a record's audit or log field by name ("" for a field the record does not have).
func fieldValue(r hdbread.Record, name string) string {
	a, l := r.Audit, r.Log
	switch {
	case a != nil && name == "operation":
		return a.Operation
	case a != nil && name == "target":
		return a.Target
	case a != nil && name == "slotName":
		return a.SlotName
	case a != nil && name == "oldValue":
		return a.OldValue
	case a != nil && name == "value":
		return a.Value
	case l != nil && name == "logName":
		return l.LogName
	case l != nil && name == "level":
		return l.Level
	case l != nil && name == "message":
		return l.Message
	}
	return ""
}

// describeRecord is a matched record as evidence: its time and fields.
func describeRecord(r hdbread.Record) string {
	if a := r.Audit; a != nil {
		s := fmt.Sprintf("%s %s %s.%s", r.Timestamp, a.Operation, a.Target, a.SlotName)
		if a.Value != "" {
			s += " = " + a.Value
		}
		return s
	}
	if l := r.Log; l != nil {
		return fmt.Sprintf("%s %s %s: %s", r.Timestamp, l.Level, l.LogName, l.Message)
	}
	return r.Timestamp
}

// histories reads every history file matching the check's Glob. A file that cannot be read, or holds
// another record type than the kind reads, is an error: the check cannot run (SKIP).
func (s *Station) histories(ck *Check) (pattern string, rels []string, hs []*hdbread.History, err error) {
	pattern, rels, skip := s.rootFiles(ck) // the histories are always the station's (Root unset)
	if skip != "" {
		return pattern, nil, nil, errors.New(skip)
	}
	for _, rel := range rels {
		h, err := hdbread.ReadRecords(filepath.Join(s.dir, filepath.FromSlash(rel)))
		if err != nil {
			return pattern, nil, nil, fmt.Errorf("cannot read %s: %v", rel, err)
		}
		if !slices.Contains(recordTypes[ck.Kind], h.RecordType) {
			return pattern, nil, nil, fmt.Errorf("%s holds %s records, not %s", rel, h.RecordType, ck.Kind)
		}
		hs = append(hs, h)
	}
	return pattern, rels, hs, nil
}

func (s *Station) checkHistory(ck *Check) (string, string) {
	m := ck.Hist
	if m == nil {
		m = &HistMatch{}
	}
	pattern, rels, hs, err := s.histories(ck)
	if err != nil {
		return Skip, err.Error()
	}
	if len(hs) == 0 {
		return Fail, "no file " + pattern
	}
	if ck.Kind == kindTrend {
		return checkTrend(ck, m, rels, hs)
	}
	fields, err := histFields(ck.Kind, m)
	if err != nil {
		return Skip, err.Error()
	}
	total, n, first := 0, 0, ""
	for _, h := range hs {
		for _, r := range h.Records {
			total++
			if matchAll(r, fields) {
				if n++; n == 1 {
					first = describeRecord(r)
				}
			}
		}
	}
	where := rels[0]
	if len(rels) > 1 {
		where = fmt.Sprintf("%d files %s", len(rels), pattern)
	}
	ok := countOK(ck.Op, n, ck.N)
	ev := fmt.Sprintf("%s: %d of %s match", where, n, key.Plural(total, ck.Kind+" record"))
	if !ok {
		ev += ", want " + want(ck.Op, ck.N)
		if len(fields) > 0 {
			ev += " (" + fieldsText(fields) + ")"
		}
	}
	if first != "" {
		ev += "; first " + first
	}
	return verdict(ok), ev
}

func matchAll(r hdbread.Record, fields []histField) bool {
	for _, f := range fields {
		if !f.re.MatchString(fieldValue(r, f.name)) {
			return false
		}
	}
	return true
}

func fieldsText(fields []histField) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.name + " =~ " + f.re.String()
	}
	return strings.Join(parts, ", ")
}

// trendValue is a trend sample as a number: a float, an enum ordinal, a boolean as 0/1. ok is
// false for any other value: the check cannot read it (SKIP), never a 0 that could pass.
func trendValue(r hdbread.Record) (v float64, ok bool) {
	if r.Trend == nil {
		return 0, false
	}
	switch v := r.Trend.Value.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

// checkTrend: every file holds at least / exactly N records (with Distinct, N distinct values), every
// value within [Min, Max] when the range is set. NaN is never within the range, and is neither a
// distinct value nor a bound of the evidence's min..max.
func checkTrend(ck *Check, m *HistMatch, rels []string, hs []*hdbread.History) (string, string) {
	first, bad := "", ""
	for i, h := range hs {
		seen := map[float64]bool{}
		lo, hi, out := math.Inf(1), math.Inf(-1), ""
		for _, r := range h.Records {
			v, ok := trendValue(r)
			if !ok {
				var val any
				if r.Trend != nil {
					val = r.Trend.Value
				}
				return Skip, fmt.Sprintf("%s: unsupported trend value %T at %s", rels[i], val, r.Timestamp)
			}
			if !math.IsNaN(v) {
				seen[v] = true
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
			if m.Max > m.Min && out == "" && !(v >= m.Min && v <= m.Max) { // NaN is out of range
				out = fmt.Sprintf("%v at %s", v, r.Timestamp)
			}
		}
		n, noun := len(h.Records), "record"
		if ck.Distinct {
			n, noun = len(seen), "distinct value"
		}
		ev := fmt.Sprintf("%s: %s", rels[i], key.Plural(n, noun))
		if lo <= hi { // a value other than NaN
			ev += fmt.Sprintf(", %v..%v", lo, hi)
		}
		if first == "" {
			first = ev
		}
		switch {
		case bad != "":
		case !countOK(ck.Op, n, ck.N):
			bad = ev + ", want " + want(ck.Op, ck.N)
		case out != "":
			bad = fmt.Sprintf("%s, want every value in %v..%v: %s", ev, m.Min, m.Max, out)
		}
	}
	if bad != "" {
		return Fail, bad
	}
	if len(hs) == 1 {
		return Pass, first
	}
	return Pass, fmt.Sprintf("%d of %d files: %s", len(hs), len(hs), first)
}
