package hdbread

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// This file builds synthetic record-store .hdb files the way the N4 writer lays them out
// (recstore.go has the layout and its sources), so the decoder is tested against bytes it did not
// produce itself.

// recEncoder is a java.io.DataOutput subset: BHistoryRecord.write and the doWrite of
// BLogRecord/BAuditRecord.
type recEncoder struct{ b []byte }

// timestamp is BAbsTime.encode48 (version 2) or BAbsTime.encode, a plain long (version 1).
func (e *recEncoder) timestamp(ms int64, version uint32) {
	if version == 1 {
		e.b = binary.BigEndian.AppendUint64(e.b, uint64(ms))
		return
	}
	e.b = append(e.b, byte((ms>>40)+16))
	for i := 4; i >= 0; i-- {
		e.b = append(e.b, byte(ms>>(8*i)))
	}
}

func (e *recEncoder) int32(v int32) { e.b = binary.BigEndian.AppendUint32(e.b, uint32(v)) }

// utf is DataOutput.writeUTF (javax/baja/nre/util/ByteBuffer.writeUTF): modified UTF-8 of the
// UTF-16 code units, U+0000 as C0 80, a 2-byte length first.
func (e *recEncoder) utf(s string) {
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

func logBytes(ms int64, version uint32, logName string, severity int32, message, exception string) []byte {
	var e recEncoder
	e.timestamp(ms, version)
	e.utf(logName)
	e.int32(severity)
	e.utf(message)
	e.utf(exception)
	return e.b
}

func auditBytes(ms int64, version uint32, op, target, slot, oldValue, value, user string) []byte {
	var e recEncoder
	e.timestamp(ms, version)
	for _, s := range []string{op, target, slot, oldValue, value, user} {
		e.utf(s)
	}
	return e.b
}

// storeBuilder is a record store: the 256-byte header and its pages.
type storeBuilder struct {
	blockSize, pageBlocks int
	first, last, count    int
	pages                 []*pageBuilder
}

type pageBuilder struct {
	s                  *storeBuilder
	buf                []byte
	records, first, at int // at is Page.free, the next free block
}

func newStore(blockSize, pageBlocks, pages int) *storeBuilder {
	s := &storeBuilder{blockSize: blockSize, pageBlocks: pageBlocks, first: -1, last: -1}
	for i := 0; i < pages; i++ {
		s.pages = append(s.pages, &pageBuilder{s: s, buf: make([]byte, pageBlocks*blockSize+12), first: -1})
	}
	return s
}

// append is Page.appendRecord: ceil(len/capacity) blocks, each with its blocksInRec and recBlock.
func (p *pageBuilder) append(rec []byte) *pageBuilder {
	capacity := p.s.blockSize - 8
	blocks := (len(rec) + capacity - 1) / capacity
	for k := 0; len(rec) > 0; k++ {
		n := min(len(rec), capacity)
		start := 12 + p.at*p.s.blockSize
		binary.BigEndian.PutUint32(p.buf[start:], uint32(blocks))
		binary.BigEndian.PutUint32(p.buf[start+4:], uint32(k))
		copy(p.buf[start+8:], rec[:n])
		rec = rec[n:]
		p.at++
	}
	p.records++
	if p.first == -1 {
		p.first = 0
	}
	return p
}

// trim is Page.trimFromStart, including its clearBlock that zeroes the first blockSize bytes of
// the page buffer whatever block it is asked to clear (Page.java:195-203).
func (p *pageBuilder) trim() *pageBuilder {
	start := 12 + p.first*p.s.blockSize
	n := int(binary.BigEndian.Uint32(p.buf[start:]))
	clear(p.buf[:p.s.blockSize])
	p.first += n
	if p.records--; p.records == 0 {
		p.first, p.at = -1, 0
	}
	return p
}

// bytes is the data section: header then the pages, each with its recordCount, first and free.
func (s *storeBuilder) bytes() []byte {
	out := make([]byte, 256)
	for i, v := range []int{0x0addab01, 1, s.blockSize, s.pageBlocks, s.first, s.last, s.count} {
		binary.BigEndian.PutUint32(out[4*i:], uint32(int32(v)))
	}
	for _, p := range s.pages {
		for i, v := range []int{p.records, p.first, p.at} {
			binary.BigEndian.PutUint32(p.buf[4*i:], uint32(int32(v)))
		}
		out = append(out, p.buf...)
	}
	return out
}

// hdbFile wraps a data section in the .hdb file header and a 1600-byte config region
// (BFileHistoryTable.java:821-873).
func hdbFile(version uint32, id, recordType, schema string, section []byte) []byte {
	xml := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<bajaObjectGraph version=\"4.0\">\n"+
		"<p m=\"h=history\" t=\"h:HistoryConfig\">\n <p n=\"id\" t=\"h:HistoryId\" v=\"%s\"/>\n"+
		" <p n=\"recordType\" t=\"b:TypeSpec\" v=\"%s\"/>\n <p n=\"schema\" t=\"h:HistorySchema\" v=\"%s\"/>\n"+
		"</p>\n</bajaObjectGraph>\n", id, recordType, schema)
	head := make([]byte, 1600)
	copy(head, magic)
	binary.BigEndian.PutUint32(head[4:], version)
	binary.BigEndian.PutUint32(head[8:], 1600)
	copy(head[12:], xml)
	return append(head, section...)
}

const (
	logSchema = "timestamp,baja:AbsTime;logName,baja:String;severity,baja:Integer;message,baja:String;" +
		"exception,baja:String"
	auditSchema = "timestamp,baja:AbsTime;operation,baja:String;target,baja:String;slotName,baja:String;" +
		"oldValue,baja:String;value,baja:String;userName,baja:String"
)

func logFile(version uint32, s *storeBuilder) []byte {
	return hdbFile(version, "/S/LogHistory", "history:LogRecord", logSchema, s.bytes())
}

func auditFile(version uint32, s *storeBuilder) []byte {
	return hdbFile(version, "/S/AuditHistory", "history:AuditRecord", auditSchema, s.bytes())
}

// t0 is 2026-10-16T18:33:00.000Z.
const t0 = int64(1792175580000)

// fixtures are the synthetic inputs of the Go-only golden cases (testdata/go/inputs), by name.
func fixtures() map[string][]byte {
	out := map[string][]byte{}

	// Three log records on one page: a multi-block exception, a message that needs CSV quoting
	// and non-ASCII text (a NUL char, an accented letter, a supplementary-plane emoji), and a
	// severity that is not a standard java.util.logging.Level.
	s := newStore(64, 8, 1)
	s.first, s.last, s.count = 0, 0, 3
	s.pages[0].append(logBytes(t0, 2, "sys.engine", 800, "Station started", "")).
		append(logBytes(t0+1500, 2, "trafficLightDriver", 1000, "poll failed, retrying \"dev1\"",
			"java.io.IOException: timeout\n\tat a.B.c(B.java:1)\n\tat a.B.d(B.java:2)")).
		append(logBytes(t0+61000, 2, "custom", 850, "café \u0000 \U0001F600", ""))
	out["log-v2.hdb"] = logFile(2, s)

	// An audit ring: three pages, the oldest is page 1 (its first record trimmed, so the page's
	// first block is 1), the newest is page 0: the walk goes 1, 2 and wraps to 0.
	s = newStore(64, 4, 3)
	s.first, s.last, s.count = 1, 0, 4
	s.pages[1].append(auditBytes(t0-120000, 2, "Added", "/", "Old", "", "Folder", "admin")).
		append(auditBytes(t0-60000, 2, "Changed", "/Test/Ramp", "amplitude", "1.0", "2.5", "admin")).trim()
	s.pages[2].append(auditBytes(t0, 2, "Invoked", "/Test/Ramp", "set", "", "", "operator, one")).
		append(auditBytes(t0+1000, 2, "Login", "", "", "", "", "admin"))
	s.pages[0].append(auditBytes(t0+2000, 2, "Flags Changed", "/Test/Limiter/out", "out", "",
		"hidden,summary", "admin"))
	out["audit-ring.hdb"] = auditFile(2, s)

	// Version 1: 8-byte BAbsTime.encode timestamps.
	s = newStore(64, 4, 1)
	s.first, s.last, s.count = 0, 0, 1
	s.pages[0].append(auditBytes(t0, 1, "Removed", "/Test", "Limiter", "", "", "admin"))
	out["audit-v1.hdb"] = auditFile(1, s)

	// A created store that never got a record: first and last page -1.
	out["log-empty.hdb"] = logFile(2, newStore(64, 4, 0))

	// A record that declares one block but whose strings run past it.
	s = newStore(64, 4, 1)
	s.first, s.last, s.count = 0, 0, 1
	rec := auditBytes(t0, 2, "Changed", "/a", strings.Repeat("x", 80), "", "", "admin")
	s.pages[0].append(rec)
	binary.BigEndian.PutUint32(s.pages[0].buf[12:], 1)
	out["audit-overrun.hdb"] = auditFile(2, s)
	return out
}
