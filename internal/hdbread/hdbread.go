// Package hdbread reads Niagara N4 .hdb history files (port of niagara-research tools/hdbread.py).
// The on-disk format and the history-rt file:line of each rule are documented in the Python
// module docstring; this package reproduces its output byte for byte. Decoding LogRecord and
// AuditRecord histories is Go-only (D34): their record-store layout is in recstore.go, the
// library API (ReadRecords, DecodeRecords) in records.go, and its CLI cases in testdata/go.
package hdbread

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/honest"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Options is the parsed command line. The caller validates it (JSON needs Records).
type Options struct {
	Path    string // the .hdb file
	Schema  bool   // history id, record type and field list (the default when nothing else is set)
	Ops     bool   // count audit-log operation strings
	Strings bool   // printable strings of the record region
	Mask    bool   // rule v5 redaction of --strings, --ops names, the --schema history id and log/audit --records
	Records bool   // decode trend, log or audit records: CSV on stdout, summary on stderr
	JSON    bool   // with Records: a JSON list instead of CSV
}

var magic = []byte{0xa1, 0x06, 0xf1, 0x1e}

// exitError is a Python `raise SystemExit("message")`: the message on stderr, exit 1.
type exitError string

func (e exitError) Error() string { return string(e) }

// file is the parsed file header: version, data offset, config XML text and data section.
type file struct {
	version uint32
	offset  uint32
	xml     string
	section []byte
}

// parse splits a .hdb into its header fields; the config ends at dataOffset
// (BFileHistoryTable.java:821-873).
func parse(d []byte) (*file, error) {
	head := d
	if len(head) > 4 {
		head = head[:4]
	}
	if !bytes.Equal(head, magic) {
		return nil, exitError(fmt.Sprintf("not a .hdb (bad magic %x)", head))
	}
	if len(d) < 12 {
		return nil, exitError(fmt.Sprintf("not a .hdb (truncated header, %d bytes)", len(d)))
	}
	ver := binary.BigEndian.Uint32(d[4:8])
	off := binary.BigEndian.Uint32(d[8:12])
	if off < 12 || uint64(off) > uint64(len(d)) {
		return nil, exitError(fmt.Sprintf("not a .hdb (data offset %d outside the %d-byte file)", off, len(d)))
	}
	xml := pycompat.DecodeReplace(d[12:off])
	const closing = "</bajaObjectGraph>"
	if end := strings.Index(xml, closing); end >= 0 {
		xml = xml[:end+len(closing)]
	} else {
		xml = strings.TrimRight(xml, " \x00")
	}
	return &file{version: ver, offset: off, xml: xml, section: d[off:]}, nil
}

var (
	schemaRE     = regexp.MustCompile(`n="schema"[^>]*v="([^"]*)"`)
	recordTypeRE = regexp.MustCompile(`n="recordType"[^>]*v="([^"]*)"`)
	idRE         = regexp.MustCompile(`n="id"[^>]*v="([^"]*)"`)
)

func firstGroup(re *regexp.Regexp, s, missing string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return missing
}

// schemaOf returns the history id, the record type and the schema field names.
func schemaOf(xml string) (id, recordType string, fields []string) {
	if m := schemaRE.FindStringSubmatch(xml); m != nil {
		for _, pair := range strings.Split(m[1], ";") {
			if name, _, ok := strings.Cut(pair, ","); ok {
				fields = append(fields, name)
			}
		}
	}
	return firstGroup(idRE, xml, "?"), firstGroup(recordTypeRE, xml, "?"), fields
}

// errorAccessDenied is the Win32 ERROR_ACCESS_DENIED (EACCES through the C runtime).
const errorAccessDenied = 5

// readFile is Python's open(path, "rb").read(). On Windows the C runtime's open() refuses a
// directory with ERROR_ACCESS_DENIED, where Go opens it and only the read fails
// (ERROR_INVALID_FUNCTION, "Invalid argument"); the error carries Python's code instead.
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil && runtime.GOOS == "windows" && pycompat.IsDir(path) {
		return nil, &fs.PathError{Op: "open", Path: path, Err: syscall.Errno(errorAccessDenied)}
	}
	return data, err
}

// Run executes one hdbread invocation and returns its exit code.
func Run(o Options, s cli.Stdio) int {
	data, err := readFile(o.Path)
	if err != nil {
		return honest.Fail(s.Err, "hdbread", fmt.Sprintf("cannot read %s: %s", o.Path, pycompat.StrerrorOf(err)), honest.ExitInput)
	}
	if o.Records {
		return runRecords(o, data, s)
	}
	f, err := parse(data)
	if err != nil {
		fmt.Fprintln(s.Err, err)
		return cli.ExitNoMatch
	}
	return report(o, f, s)
}

// report prints the --schema, --ops and --strings sections of the parsed file f.
func report(o Options, f *file, s cli.Stdio) int {
	id, rt, fields := schemaOf(f.xml)
	fmt.Fprintf(s.Out, "# %s\n", o.Path)
	fmt.Fprintf(s.Out, "magic=A106F11E version=%d data_offset=%d data_section=%d bytes\n",
		f.version, f.offset, len(f.section))
	if o.Schema || !(o.Ops || o.Strings || o.Mask) {
		if o.Mask { // the history id is free text from the file: rule v5 masks it
			id = valueMasked
		}
		fmt.Fprintf(s.Out, "history_id=%s\n", id)
		fmt.Fprintf(s.Out, "record_type=%s\n", rt)
		fmt.Fprintf(s.Out, "schema (%d fields): %s\n", len(fields), strings.Join(fields, ", "))
	}
	if o.Ops {
		if err := printOps(s, f.section, rt, fields, o.Mask); err != nil {
			fmt.Fprintln(s.Err, err)
			return cli.ExitNoMatch
		}
	}
	if o.Strings {
		for _, line := range stringsLines(f.section, o.Mask) {
			fmt.Fprintln(s.Out, line)
		}
	}
	return cli.ExitFound
}

// runRecords is --records: the library's decode picks the decoder, the CLI only prints.
func runRecords(o Options, data []byte, s cli.Stdio) int {
	d, err := decode(data)
	if err != nil {
		var exit exitError
		if errors.As(err, &exit) {
			fmt.Fprintln(s.Err, err)
			return cli.ExitNoMatch
		}
		return honest.Fail(s.Err, "hdbread", "--records: "+err.Error(), cli.ExitNoMatch)
	}
	if d.store != nil {
		return printVarRecords(o, d.store, s)
	}
	t := d.trend
	if o.JSON {
		printJSON(s.Out, t.records)
	} else {
		printCSV(s.Out, t.records)
	}
	fmt.Fprintf(s.Err, "# %s\n# %s\n", o.Path, trendSummary(t))
	if len(t.records) == 0 {
		return cli.ExitNoMatch
	}
	return cli.ExitFound
}
