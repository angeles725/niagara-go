package check

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

// The history files of the synthetic stations, built the way the N4 writer lays them out (the
// layouts and their sources are in internal/hdbread: recstore.go for the audit and log record
// store, trend.go for the fixed-length trend table), so the history kinds are tested on bytes
// internal/hdbread decodes, not on structs. Regenerate with COURSE_UPDATE_FIXTURES=1 go test -run
// TestFixtureHistories; the test fails when a committed file drifts from the table.

// hdbEnc is the java.io.DataOutput subset the record store writes.
type hdbEnc struct{ b []byte }

// ts is BAbsTime.encode48, the version-2 timestamp.
func (e *hdbEnc) ts(ms int64) {
	e.b = append(e.b, byte((ms>>40)+16))
	for i := 4; i >= 0; i-- {
		e.b = append(e.b, byte(ms>>(8*i)))
	}
}

// utf is DataOutput.writeUTF: modified UTF-8 of the UTF-16 code units, a 2-byte length first.
func (e *hdbEnc) utf(s string) {
	var body []byte
	for _, c := range utf16.Encode([]rune(s)) {
		switch {
		case c >= 0x0001 && c <= 0x007f:
			body = append(body, byte(c))
		case c > 0x07ff:
			body = append(body, 0xe0|byte(c>>12&0x0f), 0x80|byte(c>>6&0x3f), 0x80|byte(c&0x3f))
		default:
			body = append(body, 0xc0|byte(c>>6&0x1f), 0x80|byte(c&0x3f))
		}
	}
	e.b = binary.BigEndian.AppendUint16(e.b, uint16(len(body)))
	e.b = append(e.b, body...)
}

// auditRec is one BAuditRecord: operation, target, slotName, oldValue, value (user admin).
type auditRec [5]string

// logRec is one BLogRecord: logName, severity, message.
type logRec struct {
	name     string
	severity int32
	message  string
}

// t0 is 2026-10-15T18:42:32.959Z, the paste of the live Lab 10 copies.
const t0 = int64(1792089752959)

// recordStore is a one-page record store (RecordStoreHeader then the page): each record takes
// ceil(len/56) 64-byte blocks, each block led by its blocksInRec and its index.
func recordStore(recs [][]byte) []byte {
	const blockSize = 64
	capacity := blockSize - 8
	blocks := 0
	for _, r := range recs {
		blocks += (len(r) + capacity - 1) / capacity
	}
	page := make([]byte, blocks*blockSize+12)
	at := 0
	for _, rec := range recs {
		n := (len(rec) + capacity - 1) / capacity
		for k := 0; len(rec) > 0; k++ {
			c := min(len(rec), capacity)
			start := 12 + at*blockSize
			binary.BigEndian.PutUint32(page[start:], uint32(n))
			binary.BigEndian.PutUint32(page[start+4:], uint32(k))
			copy(page[start+8:], rec[:c])
			rec = rec[c:]
			at++
		}
	}
	for i, v := range []int{len(recs), 0, at} {
		binary.BigEndian.PutUint32(page[4*i:], uint32(int32(v)))
	}
	out := make([]byte, 256)
	for i, v := range []int{0x0addab01, 1, blockSize, blocks, 0, 0, len(recs)} {
		binary.BigEndian.PutUint32(out[4*i:], uint32(int32(v)))
	}
	return append(out, page...)
}

// trendTable is a one-page fixed table of version-2 NumericTrendRecords (6-byte time, status and
// trend-flags bytes, a float32), one sample a second from t0.
func trendTable(values []float64) []byte {
	const recSize = 12
	pageSize := 8 + recSize*max(len(values), 1)
	out := make([]byte, 64)
	for i, v := range []int{1, pageSize, 1, recSize, len(values), 0, 0} {
		binary.BigEndian.PutUint32(out[4*i:], uint32(int32(v)))
	}
	page := make([]byte, pageSize)
	binary.BigEndian.PutUint32(page, 0)
	binary.BigEndian.PutUint32(page[4:], uint32(int32(len(values)-1)))
	for j, v := range values {
		var e hdbEnc
		e.ts(t0 + int64(j)*1000)
		e.b = append(e.b, 0, 0)
		e.b = binary.BigEndian.AppendUint32(e.b, math.Float32bits(float32(v)))
		copy(page[8+j*recSize:], e.b)
	}
	return append(out, page...)
}

// hdbFile wraps a data section in the .hdb header and its 1600-byte HistoryConfig region.
func hdbFile(id, recordType string, section []byte) []byte {
	xml := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<bajaObjectGraph version=\"4.0\">\n"+
		"<p m=\"h=history\" t=\"h:HistoryConfig\">\n <p n=\"id\" t=\"h:HistoryId\" v=\"%s\"/>\n"+
		" <p n=\"recordType\" t=\"b:TypeSpec\" v=\"%s\"/>\n</p>\n</bajaObjectGraph>\n", id, recordType)
	head := make([]byte, 1600)
	binary.BigEndian.PutUint32(head, 0xA106F11E)
	binary.BigEndian.PutUint32(head[4:], 2)
	binary.BigEndian.PutUint32(head[8:], 1600)
	copy(head[12:], xml)
	return append(head, section...)
}

func auditHDB(station string, recs []auditRec) []byte {
	var out [][]byte
	for i, r := range recs {
		var e hdbEnc
		e.ts(t0 + int64(i)*1000)
		for _, s := range append(r[:], "admin") {
			e.utf(s)
		}
		out = append(out, e.b)
	}
	return hdbFile("/"+station+"/AuditHistory", "history:AuditRecord", recordStore(out))
}

func logHDB(station string, recs []logRec) []byte {
	var out [][]byte
	for i, r := range recs {
		var e hdbEnc
		e.ts(t0 + int64(i)*1000)
		e.utf(r.name)
		e.b = binary.BigEndian.AppendUint32(e.b, uint32(r.severity))
		e.utf(r.message)
		e.utf("")
		out = append(out, e.b)
	}
	return hdbFile("/"+station+"/LogHistory", "history:LogRecord", recordStore(out))
}

func trendHDB(station, point string, values []float64) []byte {
	return hdbFile("/"+station+"/"+point, "history:NumericTrendRecord", trendTable(values))
}

// ramp is a triangle from lo by step, n samples: the live Ramp's samples (offset 50, amplitude 50).
func ramp(lo, step float64, n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = lo + float64(i)*step
	}
	return out
}

// lab10Adds are the audit records of the Lab 10 paste: the copies NumericWritable1..n.
func lab10Adds(n int) []auditRec {
	var out []auditRec
	for i := 1; i <= n; i++ {
		out = append(out, auditRec{"Added", "/Lab10", fmt.Sprintf("NumericWritable%d", i), "", "36,3 {ok} @ 16"})
	}
	return out
}

// fixtureHistories are the history files of the synthetic stations, by path under testdata/stations.
func fixtureHistories() map[string][]byte {
	good := []auditRec{
		{"Added", "/", "Lab10", "", "Folder"},
		{"Changed", "/Lab4/SpaceTemp", "facets", "units=null,precision=1", "units=°C,precision=1"},
		{"Invoked", "/Lab4/SpaceTemp", "set", "24,0 °C {ok} (Out = 24,0 °C {ok} @ def)", "22,0 °C"},
	}
	// The bad station: three copies only, and a set whose Out has no unit (no facets).
	bad := []auditRec{{"Invoked", "/Lab4/SpaceTemp", "set", "24,0 {ok} (Out = 24,0 {ok} @ def)", "22,0"}}
	out := map[string][]byte{
		"good/history/station/seg7/AuditHistory.hdb": auditHDB("GoodStation", append(good, lab10Adds(9)...)),
		"bad/history/station/seg7/AuditHistory.hdb":  auditHDB("BadStation", append(bad, lab10Adds(3)...)),
	}
	for i := 0; i <= 9; i++ {
		point := "NumericWritable"
		if i > 0 {
			point += fmt.Sprint(i)
		}
		out[fmt.Sprintf("good/history/station/seg%d/%s.hdb", i%9, point)] = trendHDB("GoodStation", point, ramp(40, 3.5, 6))
	}
	// The bad station: a point whose history never changed, one that left the Ramp's range.
	out["bad/history/station/seg0/NumericWritable1.hdb"] = trendHDB("BadStation", "NumericWritable1", []float64{0, 0, 0})
	out["bad/history/station/seg1/NumericWritable2.hdb"] = trendHDB("BadStation", "NumericWritable2", ramp(90, 5, 4))
	return out
}

const fixtureStations = "../testdata/stations"

func TestFixtureHistories(t *testing.T) {
	update := os.Getenv("COURSE_UPDATE_FIXTURES") == "1"
	for rel, data := range fixtureHistories() {
		path := filepath.Join(fixtureStations, filepath.FromSlash(rel))
		if update {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, data) {
			t.Errorf("%s drifted from fixtureHistories (regenerate: COURSE_UPDATE_FIXTURES=1 go test -run TestFixtureHistories): %v", rel, err)
		}
	}
	files, _ := filepath.Glob(filepath.Join(fixtureStations, "*", "history", "*", "*", "*.hdb"))
	if len(files) != len(fixtureHistories()) {
		t.Errorf("%d history files under %s, want the %d of fixtureHistories", len(files), fixtureStations, len(fixtureHistories()))
	}
}
