package hdbread

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/cli"
)

// doc is the Python module docstring, the --help description (RawDescriptionHelpFormatter).
const doc = "hdbread.py — read-only reader for Niagara N4 .hdb history files.\n" +
	"\n" +
	"On-disk format, derived from the history-rt code (organized/history/history-rt/vineflower, N4; file:line):\n" +
	"  [0:4]   magic 0xA106F11E (-1593380578)          com/tridium/history/file/BFileHistoryTable.java:63,821\n" +
	"  [4:8]   file/history version (1 or 2)            BFileHistoryTable.java:128-130,821\n" +
	"  [8:12]  dataOffset: start of the data section,   BFileHistoryTable.java:821 (1600), :852-858 (doubled\n" +
	"          1600 unless the config needed more        while the config does not fit), :873 (read back)\n" +
	"  [12:dataOffset]  the HistoryConfig as BOG XML, space/zero padded (BFileHistoryTable.java:125-131,848-850)\n" +
	"  Fixed-length tables (a record type whose isFixedSize() is true: Numeric/Boolean/EnumTrendRecord,\n" +
	"  BNumericTrendRecord.java \"isFixedSize ... return true\") use com/tridium/history/file/fixed:\n" +
	"  [dataOffset : +28]  Header: 7 big-endian ints version(=1), pageSize, pageCount, recSize, capacity,\n" +
	"                      firstPage, lastPage   (fixed/Header.java:149-163); 64 bytes reserved (Header.SIZE)\n" +
	"  page i at dataOffset + 64 + i*pageSize   (fixed/PageManager.java:640-645); a page is first, last\n" +
	"                      (two ints, -1 = empty) then record j at 8 + j*recSize   (fixed/Page.java:38-40,60-66,89-91)\n" +
	"  page order: firstPage, next(i) = i+1 wrapping at pageCount, stop after lastPage\n" +
	"                      (fixed/Header.java next(), fixed/PageManager.java:583-587)\n" +
	"  record: timestamp, then status, trend flags and value (javax/baja/history/BHistoryRecord.java:96-99,\n" +
	"                      BTrendRecord.java:68-76,110-117):\n" +
	"    version 2: BAbsTime.decode48 = BAbsTime.decode: first byte b != 0 -> millis = ((b-16)&0xFF)<<40 | next\n" +
	"               5 bytes; b == 0 -> 8-byte form (baja/javax/baja/sys/BAbsTime.java:635-662); then status\n" +
	"               byte (BStatus bits) and trend-flags byte\n" +
	"    version 1: 8-byte millis (BAbsTime.encode = writeLong, BAbsTime.java:630-632), then one int:\n" +
	"               trendFlags << 24 | status bits (BTrendRecord.java:110-124)\n" +
	"    value: NumericTrendRecord float32 (BNumericTrendRecord.java:77-84), BooleanTrendRecord boolean byte\n" +
	"           (BBooleanTrendRecord.java:88-95), EnumTrendRecord int ordinal (BEnumTrendRecord.java:68-75)\n" +
	"  status names: BStatus support tags 1 disabled .. 128 unackedAlarm, \"{ok}\" when empty\n" +
	"                      (baja/javax/baja/sys/BStatus.java:287-290,354-357,484-491)\n" +
	"  trend flags: 1 start, 2 outOfOrder, 4 hidden, 8 modified, 16 interpolated, 32/64/128 reserved0-2,\n" +
	"               \"{ }\" when empty (javax/baja/history/BTrendFlags.java:155-158,244-251)\n" +
	"A header record size that differs from the size these rules give for the record type and version is\n" +
	"refused (HdbFormatError), never guessed. Audit/log records are variable-length (recstore/): --ops and\n" +
	"--strings walk them heuristically, --records does not decode them.\n" +
	"\n" +
	"SECRETS DISCIPLINE: audit/log records carry operator activity. Use --mask to redact user identities\n" +
	"and long values before printing; cite structure/counts, not specific secret values.\n" +
	"\n" +
	"Usage:\n" +
	"  python3 hdbread.py <file.hdb> [--schema] [--ops] [--strings] [--mask]\n" +
	"  python3 hdbread.py <file.hdb> --records [--json]   trend records as CSV (or JSON) on stdout,\n" +
	"        timestamp (UTC ISO 8601), millis, status, trendFlags, value; summary on stderr\n" +
	"  (no option = --schema). Exit 0 ok, 2 usage or an unreadable file, 1 a non-.hdb file, nothing decoded\n" +
	"  or a record format this reader does not decode.\n" +
	""

func newParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: "hdbread.py", Description: doc,
		Formatter: argparse.RawDescriptionHelpFormatter})
	p.Add(argparse.Arg{Help: "a Niagara .hdb history file"}, "hdb")
	flag := func(name, help string) { p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: help}, name) }
	flag("--schema", "history id, record type and field list (default)")
	flag("--ops", "count audit-log operation strings")
	flag("--strings", "printable strings of the record region")
	flag("--mask", "fail-closed redaction of --strings, --ops and --schema output")
	flag("--records", "decode Numeric/Boolean/EnumTrendRecord rows: CSV on stdout, summary on stderr")
	flag("--json", "with --records: a JSON list instead of CSV")
	return p
}

// Main is the hdbread command line (Python argparse parity).
func Main(args []string, s cli.Stdio) int {
	p := newParser()
	ns, code, done := p.Parse(args, s)
	if done {
		return code
	}
	o := Options{Path: ns.String("hdb"), Schema: ns.Bool("schema"), Ops: ns.Bool("ops"),
		Strings: ns.Bool("strings"), Mask: ns.Bool("mask"), Records: ns.Bool("records"), JSON: ns.Bool("json")}
	if o.JSON && !o.Records {
		return p.Error(s.Err, "--json needs --records")
	}
	return Run(o, s)
}
