package hdbread

import (
	"os"
	"sort"
	"strings"
)

// Record kinds, as Record.Kind returns them.
const (
	KindTrend = "trend" // history:NumericTrendRecord, BooleanTrendRecord, EnumTrendRecord
	KindLog   = "log"   // history:LogRecord
	KindAudit = "audit" // history:AuditRecord
)

// History is a decoded .hdb file: its config identity and its records in storage order (oldest
// first, the order the N4 history cursor returns them).
type History struct {
	ID         string // the config history id, e.g. /TrainingStation/AuditHistory
	RecordType string // the config record type, e.g. history:AuditRecord
	Version    uint32 // the .hdb file version (1 or 2), which picks the timestamp encoding
	Records    []Record
	// Declared is the record count the file states: the record-store header recordCount for
	// log/audit histories, the sum of the page ranges for trend histories.
	Declared int
}

// Record is one history record. Exactly one of Trend, Log and Audit is set, matching Kind.
type Record struct {
	Timestamp string // UTC, ISO 8601 with milliseconds: 2026-10-16T18:33:00.000Z
	Millis    int64  // the BAbsTime epoch milliseconds
	Trend     *TrendRecord
	Log       *LogRecord
	Audit     *AuditRecord
}

// Kind is KindTrend, KindLog or KindAudit.
func (r Record) Kind() string {
	switch {
	case r.Log != nil:
		return KindLog
	case r.Audit != nil:
		return KindAudit
	}
	return KindTrend
}

// TrendRecord is a fixed-length trend sample (layout in main.go doc).
type TrendRecord struct {
	Status     string // BStatus tags, "{ok}" when none: {overridden,stale}
	TrendFlags string // BTrendFlags tags, "{ }" when none
	Value      any    // float64 (Numeric), bool (Boolean) or int64 (Enum ordinal)
}

// LogRecord is a com.tridium.history.log.BLogRecord.
type LogRecord struct {
	LogName   string `json:"logName"`
	Severity  int32  `json:"severity"` // java.util.logging.Level intValue
	Level     string `json:"level"`    // the Level name (SEVERE, WARNING, INFO, ...), or Severity in decimal
	Message   string `json:"message"`
	Exception string `json:"exception"` // the stack trace text, "" when none
}

// AuditRecord is a com.tridium.history.audit.BAuditRecord.
type AuditRecord struct {
	Operation string `json:"operation"` // an AuditEvent operation: Changed, Added, Invoked, Login, ...
	Target    string `json:"target"`    // the component slot path, e.g. /Lab1/Folder
	SlotName  string `json:"slotName"`
	OldValue  string `json:"oldValue"`
	Value     string `json:"value"`
	UserName  string `json:"userName"`
}

// ReadRecords reads and decodes the .hdb file at path. Errors: the os error of an unreadable
// file, or a FormatError (or a not-a-.hdb error) for bytes it cannot decode.
func ReadRecords(path string) (*History, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return DecodeRecords(data)
}

// DecodeRecords decodes the bytes of a .hdb file: fixed-length trend records or record-store
// LogRecord/AuditRecord records. Any other record type is a FormatError.
func DecodeRecords(data []byte) (*History, error) {
	f, err := parse(data)
	if err != nil {
		return nil, err
	}
	id, rt, _ := schemaOf(f.xml)
	if _, ok := valueBytes[rt]; ok {
		return trendHistory(data, id)
	}
	decode, ok := varDecoders[rt]
	if !ok {
		return nil, formatErr("record type %s is not decoded (decoded: %s)", rt, decodedTypes())
	}
	out := &History{ID: id, RecordType: rt, Version: f.version}
	h, err := walkStore(f.section, func(h storeHeader, s storedRecord) error {
		rec, err := decodeStored(s, h.blockSize-blockHeaderSize, decode)
		out.Records = append(out.Records, rec)
		return err
	})
	if err != nil {
		return nil, err
	}
	out.Declared = h.recordCount
	return out, nil
}

func trendHistory(data []byte, id string) (*History, error) {
	t, err := decodeTrend(data)
	if err != nil {
		return nil, err
	}
	out := &History{ID: id, RecordType: t.recordType, Version: t.version, Declared: t.expected}
	for _, r := range t.records {
		v := map[byte]any{'f': r.value.f, 'b': r.value.b, 'i': r.value.i}[r.value.kind]
		out.Records = append(out.Records, Record{Timestamp: r.timestamp, Millis: r.millis,
			Trend: &TrendRecord{Status: r.status, TrendFlags: r.trendFlags, Value: v}})
	}
	return out, nil
}

// decodedTypes lists every record type DecodeRecords decodes, sorted.
func decodedTypes() string {
	var names []string
	for rt := range valueBytes {
		names = append(names, rt)
	}
	for rt := range varDecoders {
		names = append(names, rt)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
