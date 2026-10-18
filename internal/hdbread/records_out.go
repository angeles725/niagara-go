package hdbread

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/honest"
)

// Output of --records for LogRecord and AuditRecord histories (Go-only, D34: the Python tool
// froze refusing them). CSV columns follow the config schema after timestamp and millis; a log
// record adds the severity's Level name after severity.
var (
	logColumns   = []string{"logName", "severity", "level", "message", "exception"}
	auditColumns = []string{"operation", "target", "slotName", "oldValue", "value", "userName"}
)

func (l *LogRecord) fields() []string {
	return []string{l.LogName, strconv.FormatInt(int64(l.Severity), 10), l.Level, l.Message, l.Exception}
}

func (a *AuditRecord) fields() []string {
	return []string{a.Operation, a.Target, a.SlotName, a.OldValue, a.Value, a.UserName}
}

// maskRecord is --mask for log/audit records, fail-closed like --strings --mask: a non-empty
// free-text field prints valueMasked, a user identity userMasked (the userName, and in a
// login-family record every field but the operation and a '/' slot-path target). Timestamps,
// severity, level and AuditEvent operation constants print as stored.
func maskRecord(r Record) Record {
	m := func(s, with string) string {
		if s == "" {
			return ""
		}
		return with
	}
	if l := r.Log; l != nil {
		c := *l
		c.LogName, c.Message, c.Exception = m(c.LogName, valueMasked), m(c.Message, valueMasked), m(c.Exception, valueMasked)
		r.Log = &c
	}
	if a := r.Audit; a != nil {
		c := *a
		other := valueMasked
		if loginOps[c.Operation] {
			other = userMasked
		}
		if !isAuditOperation(c.Operation) {
			c.Operation = valueMasked
		}
		if strings.HasPrefix(c.Target, "/") {
			c.Target = valueMasked
		} else {
			c.Target = m(c.Target, other)
		}
		c.SlotName, c.OldValue, c.Value = m(c.SlotName, other), m(c.OldValue, other), m(c.Value, other)
		c.UserName = m(c.UserName, userMasked)
		r.Audit = &c
	}
	return r
}

func recordFields(r Record) []string {
	if r.Log != nil {
		return r.Log.fields()
	}
	return r.Audit.fields()
}

func printVarCSV(w io.Writer, h *History) {
	cols := auditColumns
	if h.RecordType == "history:LogRecord" {
		cols = logColumns
	}
	fmt.Fprintln(w, "timestamp,millis,"+strings.Join(cols, ","))
	for _, r := range h.Records {
		fields := append([]string{r.Timestamp, strconv.FormatInt(r.Millis, 10)}, recordFields(r)...)
		for i := range fields {
			fields[i] = csvField(fields[i])
		}
		fmt.Fprintln(w, strings.Join(fields, ","))
	}
}

// jsonRecord fixes the JSON key order: timestamp, millis, then the record's own fields (their
// json tags are the CSV column names).
type jsonRecord struct {
	Timestamp string `json:"timestamp"`
	Millis    int64  `json:"millis"`
	*LogRecord
	*AuditRecord
}

// printVarJSON prints a JSON list with one-space indentation, the layout of the trend --json.
func printVarJSON(w io.Writer, h *History) error {
	rows := make([]jsonRecord, 0, len(h.Records))
	for _, r := range h.Records {
		rows = append(rows, jsonRecord{r.Timestamp, r.Millis, r.Log, r.Audit})
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	return enc.Encode(rows)
}

// varSummary is the stderr evidence line: decoded and declared counts, span and ordering.
func varSummary(h *History) string {
	line := fmt.Sprintf("records=%d (header declares %d) record_type=%s version=%d", len(h.Records), h.Declared,
		h.RecordType, h.Version)
	if n := len(h.Records); n > 0 {
		back := 0
		for i := 1; i < n; i++ {
			if h.Records[i].Millis < h.Records[i-1].Millis {
				back++
			}
		}
		mono := "yes"
		if back > 0 {
			mono = fmt.Sprintf("no (%d steps back)", back)
		}
		line += fmt.Sprintf(" first=%s last=%s monotonic=%s", h.Records[0].Timestamp, h.Records[n-1].Timestamp, mono)
	}
	return line
}

// runVarRecords is --records for a LogRecord or AuditRecord history.
func runVarRecords(o Options, data []byte, s cli.Stdio) int {
	h, err := DecodeRecords(data)
	if err != nil {
		var exit exitError
		if errors.As(err, &exit) {
			fmt.Fprintln(s.Err, err)
			return cli.ExitNoMatch
		}
		return honest.Fail(s.Err, "hdbread", "--records: "+err.Error(), cli.ExitNoMatch)
	}
	if o.Mask {
		for i, r := range h.Records {
			h.Records[i] = maskRecord(r)
		}
	}
	if o.JSON {
		if err := printVarJSON(s.Out, h); err != nil {
			return honest.Fail(s.Err, "hdbread", "--records --json: "+err.Error(), cli.ExitNoMatch)
		}
	} else {
		printVarCSV(s.Out, h)
	}
	fmt.Fprintf(s.Err, "# %s\n# %s\n", o.Path, varSummary(h))
	if len(h.Records) == 0 {
		return cli.ExitNoMatch
	}
	return cli.ExitFound
}
