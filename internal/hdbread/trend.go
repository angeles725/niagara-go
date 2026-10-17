package hdbread

import (
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// FormatError is a .hdb whose trend data --records cannot decode (Python HdbFormatError): a record
// type that is not a fixed-length trend record, an unknown header version, or a geometry that does
// not fit.
type FormatError string

func (e FormatError) Error() string { return string(e) }

func formatErr(f string, a ...any) error { return FormatError(fmt.Sprintf(f, a...)) }

var (
	tsBytes   = map[uint32]int{1: 8, 2: 6} // BAbsTime.encode (long) / encode48
	flagBytes = map[uint32]int{1: 4, 2: 2} // v1 combo int / v2 status byte + trend-flags byte
	// valueBytes: record type -> value size (float32, boolean byte, int ordinal).
	valueBytes = map[string]int{"history:NumericTrendRecord": 4, "history:BooleanTrendRecord": 1,
		"history:EnumTrendRecord": 4}
	statusTags = []tag{{1, "disabled"}, {2, "fault"}, {4, "down"}, {8, "alarm"}, {16, "stale"},
		{32, "overridden"}, {64, "null"}, {128, "unackedAlarm"}}
	flagTags = []tag{{1, "start"}, {2, "outOfOrder"}, {4, "hidden"}, {8, "modified"}, {16, "interpolated"},
		{32, "reserved0"}, {64, "reserved1"}, {128, "reserved2"}}
	intervalRE = regexp.MustCompile(`n="interval"[^>]*v="(?:true|false):(\d+)"`)
)

type tag struct {
	bit  uint32
	name string
}

func tags(bits uint32, table []tag, empty string) string {
	var names []string
	for _, t := range table {
		if bits&t.bit != 0 {
			names = append(names, t.name)
		}
	}
	if names == nil {
		return empty
	}
	return "{" + strings.Join(names, ",") + "}"
}

// value is one decoded trend value: a float (Numeric), a bool (Boolean) or an int (Enum).
type value struct {
	kind byte // 'f', 'b' or 'i'
	f    float64
	b    bool
	i    int64
}

type record struct {
	timestamp  string
	millis     int64
	status     string
	trendFlags string
	value      value
}

type trendTable struct {
	recordType string
	version    uint32
	records    []record
	expected   int
	interval   string // the config interval digits, "" when absent
}

// isoUTC formats epoch millis like Python datetime.strftime("%Y-%m-%dT%H:%M:%S.") + "%03dZ";
// years outside 1..9999 are an error where Python raises OverflowError.
func isoUTC(ms int64) (string, error) {
	t := time.UnixMilli(ms).UTC()
	if t.Year() < 1 || t.Year() > 9999 {
		return "", fmt.Errorf("timestamp %d ms is outside the years 1..9999", ms)
	}
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%03dZ", t.Year(), t.Month(), t.Day(), t.Hour(),
		t.Minute(), t.Second(), t.Nanosecond()/1e6), nil
}

// millis48 is BAbsTime.decode of a version-2 timestamp at i -> (millis, bytes used); like the
// Python slices it reads short at the end of the buffer instead of failing.
func millis48(b []byte, i int) (int64, int, error) {
	if i >= len(b) {
		return 0, 0, fmt.Errorf("record at offset %d runs past the data section", i)
	}
	if b[i] == 0 {
		if i+2 >= len(b) {
			return 0, 0, fmt.Errorf("record at offset %d runs past the data section", i)
		}
		return int64(b[i+2])<<40 | beUint(clip(b, i+3, i+8)), 8, nil
	}
	return int64((b[i]-16)&0xff)<<40 | beUint(clip(b, i+1, i+6)), 6, nil
}

func clip(b []byte, i, j int) []byte {
	return b[min(i, len(b)):min(j, len(b))]
}

func beUint(b []byte) int64 {
	var v int64
	for _, c := range b {
		v = v<<8 | int64(c)
	}
	return v
}

// decodeTrend decodes a fixed-length trend history (fixed/Header.java, Page.java, PageManager.java).
func decodeTrend(data []byte) (*trendTable, error) {
	f, err := parse(data)
	if err != nil {
		return nil, err
	}
	rt := firstGroup(recordTypeRE, f.xml, "?")
	vbytes, ok := valueBytes[rt]
	if !ok {
		return nil, formatErr("record type %s is not a fixed-length trend record (decoded: %s)", rt,
			"history:BooleanTrendRecord, history:EnumTrendRecord, history:NumericTrendRecord")
	}
	if _, ok := tsBytes[f.version]; !ok {
		return nil, formatErr("history version %d is neither 1 nor 2", f.version)
	}
	want := int64(tsBytes[f.version] + flagBytes[f.version] + vbytes)
	sect := f.section
	if len(sect) < 28 {
		return nil, formatErr("data section of %d bytes has no fixed-table header", len(sect))
	}
	var h [7]int64
	for k := range h {
		h[k] = int64(int32(binary.BigEndian.Uint32(sect[4*k:])))
	}
	hv, pageSize, pageCount, recSize, first, last := h[0], h[1], h[2], h[3], h[5], h[6]
	if hv != 1 {
		return nil, formatErr("fixed-table header version %d (Header.java reads only 1)", hv)
	}
	if recSize != want {
		return nil, formatErr("header record size %d, but a version-%d %s is %d bytes", recSize, f.version, rt, want)
	}
	if pageSize <= 8 || pageCount < 0 || int64(len(sect)) < 64+pageCount*pageSize {
		return nil, formatErr("pages do not fit: pageSize %d x pageCount %d in a %d-byte data section",
			pageSize, pageCount, len(sect))
	}
	if first >= 0 && !(first < pageCount && 0 <= last && last < pageCount) {
		return nil, formatErr("firstPage %d / lastPage %d outside the %d pages", first, last, pageCount)
	}
	t := &trendTable{recordType: rt, version: f.version}
	if m := intervalRE.FindStringSubmatch(f.xml); m != nil {
		t.interval = strings.TrimLeft(m[1], "0")
		if t.interval == "" {
			t.interval = "0"
		}
	}
	perPage := (pageSize - 8) / recSize
	var order []int64
	if pageCount > 0 && first >= 0 {
		for i, n := first, int64(0); n < pageCount; n++ { // bounded: a corrupt ring cannot loop forever
			order = append(order, i)
			if i == last {
				break
			}
			if i++; i >= pageCount {
				i = 0
			}
		}
	}
	for _, p := range order {
		base := int(64 + p*pageSize)
		fi := int64(int32(binary.BigEndian.Uint32(sect[base:])))
		li := int64(int32(binary.BigEndian.Uint32(sect[base+4:])))
		if fi == -1 {
			continue
		}
		if !(0 <= fi && fi <= li && li < perPage) {
			return nil, formatErr("page %d declares records %d..%d, a page holds %d", p, fi, li, perPage)
		}
		t.expected += int(li - fi + 1)
		for j := fi; j <= li; j++ {
			r, err := decodeRecord(sect, base+8+int(j*recSize), f.version, vbytes, rt)
			if err != nil {
				return nil, err
			}
			t.records = append(t.records, r)
		}
	}
	return t, nil
}

func decodeRecord(sect []byte, r int, ver uint32, vbytes int, rt string) (record, error) {
	var ms int64
	var status, flags uint32
	var k int
	if ver == 1 { // BAbsTime.encode: a plain 8-byte long, never the encode48 form
		ms = int64(binary.BigEndian.Uint64(sect[r:]))
		combo := binary.BigEndian.Uint32(sect[r+8:])
		status, flags, k = combo&0xffffff, combo>>24, r+12
	} else {
		var used int
		var err error
		if ms, used, err = millis48(sect, r); err != nil {
			return record{}, err
		}
		k = r + used
		if k+1 >= len(sect) {
			return record{}, fmt.Errorf("record at offset %d runs past the data section", r)
		}
		status, flags, k = uint32(sect[k]), uint32(sect[k+1]), k+2
	}
	if k+vbytes > len(sect) {
		return record{}, fmt.Errorf("record at offset %d runs past the data section", r)
	}
	raw := sect[k : k+vbytes]
	var v value
	switch rt {
	case "history:NumericTrendRecord":
		v = value{kind: 'f', f: shortFloat32(binary.BigEndian.Uint32(raw))}
	case "history:BooleanTrendRecord":
		v = value{kind: 'b', b: raw[0] != 0}
	default:
		v = value{kind: 'i', i: int64(int32(binary.BigEndian.Uint32(raw)))}
	}
	ts, err := isoUTC(ms)
	if err != nil {
		return record{}, err
	}
	return record{timestamp: ts, millis: ms, status: tags(status, statusTags, "{ok}"),
		trendFlags: tags(flags, flagTags, "{ }"), value: v}, nil
}

// shortFloat32 is the shortest decimal (6..9 significant digits) that reads back to the same
// 32 bits: 36.081306, not 36.0813064575.
func shortFloat32(bits uint32) float64 {
	v := float64(math.Float32frombits(bits))
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	for digits := 6; digits < 10; digits++ {
		short, _ := pycompat.ParseFloat(formatG(v, digits))
		if math.Float32bits(float32(short)) == bits {
			return short
		}
	}
	return v
}

// trendSummary is one line of internal-consistency evidence: count, span, ordering and interval.
func trendSummary(t *trendTable) string {
	recs := t.records
	line := fmt.Sprintf("records=%d (pages declare %d) record_type=%s version=%d", len(recs), t.expected,
		t.recordType, t.version)
	if len(recs) == 0 {
		return line
	}
	back := 0
	for i := 1; i < len(recs); i++ {
		if recs[i].millis < recs[i-1].millis {
			back++
		}
	}
	mono := "yes"
	if back > 0 {
		mono = fmt.Sprintf("no (%d steps back)", back)
	}
	line += fmt.Sprintf(" first=%s last=%s monotonic=%s", recs[0].timestamp, recs[len(recs)-1].timestamp, mono)
	if len(recs) > 1 {
		steps := make([]int64, 0, len(recs)-1)
		for i := 1; i < len(recs); i++ {
			steps = append(steps, recs[i].millis-recs[i-1].millis)
		}
		sort.Slice(steps, func(a, b int) bool { return steps[a] < steps[b] })
		line += fmt.Sprintf(" median_interval_ms=%d", steps[(len(steps)-1)/2]) // statistics.median_low
		if t.interval != "" {
			line += fmt.Sprintf(" (config interval %s)", t.interval)
		}
	}
	return line
}
