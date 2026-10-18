package hdbread

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"unicode/utf16"
)

// errShort is a read past the end of a record's bytes (Java: an EOFException).
var errShort = errors.New("past the end of the record")

// fieldReader reads the java.io.DataInput fields of one record (layout in recstore.go). The
// first error sticks; later reads return zero values.
type fieldReader struct {
	b   []byte
	i   int
	err error
}

func (r *fieldReader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n > len(r.b)-r.i {
		r.err = fmt.Errorf("%w: %d bytes needed at offset %d, %d left", errShort, n, r.i, len(r.b)-r.i)
		return nil
	}
	r.i += n
	return r.b[r.i-n : r.i]
}

// millis is BAbsTime.decode (BAbsTime.java:635-650): a first byte 0 means an 8-byte long whose
// second byte is skipped, else the first byte minus 16 is bits 40-47; five more bytes follow.
func (r *fieldReader) millis() int64 {
	first := r.take(1)
	if first == nil {
		return 0
	}
	var ms int64
	if first[0] == 0 {
		hi := r.take(2)
		if hi == nil {
			return 0
		}
		ms = int64(hi[1]) << 40
	} else {
		ms = int64((first[0]-16)&0xff) << 40
	}
	return ms | beUint(r.take(5))
}

func (r *fieldReader) int32() int32 {
	if b := r.take(4); b != nil {
		return int32(binary.BigEndian.Uint32(b))
	}
	return 0
}

// utf is ReadBlock.readUTF (ReadBlock.java:189-246): a byte < 0x80 is one char (0x00 included),
// 110xxxxx 10xxxxxx two, 1110xxxx 10xxxxxx 10xxxxxx three; anything else, or a sequence that
// crosses the declared length, is a UTFDataFormatException. The UTF-16 code units become a Go
// string: a surrogate pair is one rune, a lone surrogate U+FFFD.
func (r *fieldReader) utf() string {
	head := r.take(2)
	if head == nil {
		return ""
	}
	body := r.take(int(binary.BigEndian.Uint16(head)))
	if body == nil {
		return ""
	}
	units := make([]uint16, 0, len(body))
	for i := 0; i < len(body); {
		c := body[i]
		switch {
		case c < 0x80:
			units = append(units, uint16(c))
			i++
		case c>>4 == 12 || c>>4 == 13:
			if i+2 > len(body) || body[i+1]&0xc0 != 0x80 {
				return r.badUTF(i)
			}
			units = append(units, uint16(c&0x1f)<<6|uint16(body[i+1]&0x3f))
			i += 2
		case c>>4 == 14:
			if i+3 > len(body) || body[i+1]&0xc0 != 0x80 || body[i+2]&0xc0 != 0x80 {
				return r.badUTF(i)
			}
			units = append(units, uint16(c&0x0f)<<12|uint16(body[i+1]&0x3f)<<6|uint16(body[i+2]&0x3f))
			i += 3
		default:
			return r.badUTF(i)
		}
	}
	return string(utf16.Decode(units))
}

func (r *fieldReader) badUTF(i int) string {
	r.err = fmt.Errorf("string at offset %d is not modified UTF-8 (byte %d of its text)", r.i, i)
	return ""
}

// logLevels are the java.util.logging.Level constants by intValue: BLogRecord.severityToString
// (BLogRecord.java:167-170) prints Level.parse(severity).getName(), and Level.parse of an
// integer that is no known level makes a level named by that integer.
var logLevels = map[int32]string{1000: "SEVERE", 900: "WARNING", 800: "INFO", 700: "CONFIG",
	500: "FINE", 400: "FINER", 300: "FINEST", -1 << 31: "ALL", 1<<31 - 1: "OFF"}

func levelName(severity int32) string {
	if name, ok := logLevels[severity]; ok {
		return name
	}
	return strconv.FormatInt(int64(severity), 10)
}

// varDecoders decode one record body after its timestamp, by record type.
var varDecoders = map[string]func(r *fieldReader, rec *Record){
	"history:LogRecord": func(r *fieldReader, rec *Record) {
		l := &LogRecord{LogName: r.utf(), Severity: r.int32()}
		l.Message, l.Exception, l.Level = r.utf(), r.utf(), levelName(l.Severity)
		rec.Log = l
	},
	"history:AuditRecord": func(r *fieldReader, rec *Record) {
		rec.Audit = &AuditRecord{Operation: r.utf(), Target: r.utf(), SlotName: r.utf(), OldValue: r.utf(),
			Value: r.utf(), UserName: r.utf()}
	},
}

// decodeStored decodes one stored record and checks it used the blocks it declares: a record
// that ends early leaves whole blocks unread, one that needs more runs past its last block.
func decodeStored(s storedRecord, capacity int, decode func(*fieldReader, *Record)) (Record, error) {
	r := fieldReader{b: s.data}
	var rec Record
	rec.Millis = r.millis()
	decode(&r, &rec)
	where := fmt.Sprintf("record at page %d block %d", s.page, s.block)
	if r.err != nil {
		if errors.Is(r.err, errShort) {
			return rec, formatErr("%s runs past its %d block(s): %v", where, s.blocks, r.err)
		}
		return rec, formatErr("%s: %v", where, r.err)
	}
	if used := (r.i + capacity - 1) / capacity; used != s.blocks {
		return rec, formatErr("%s declares %d blocks but its fields use %d", where, s.blocks, used)
	}
	ts, err := isoUTC(rec.Millis)
	if err != nil {
		return rec, formatErr("%s: %v", where, err)
	}
	rec.Timestamp = ts
	return rec, nil
}
