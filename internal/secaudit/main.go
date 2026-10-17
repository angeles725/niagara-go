// Package secaudit is the Go port of niagara-research tools/niagara-security-audit.py (with
// tools/lib/winacl.py): a read-only station/platform security-posture auditor for Niagara N4.
package secaudit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
)

// prog is the Python program name, kept for byte-identical usage and error lines.
const prog = "niagara-security-audit.py"

func newParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: prog})
	p.Add(argparse.Arg{}, "niagara_home")
	p.Add(argparse.Arg{}, "--station")
	p.Add(argparse.Arg{}, "--host")
	p.Add(argparse.Arg{Metavar: "DIR",
		Help: "forensic mode: harvest signature/verification IOCs from a log tree (B112)"}, "--scan-logs")
	p.Add(argparse.Arg{Action: argparse.StoreTrue}, "--json")
	return p
}

// Main is the niagara-security-audit command line.
func Main(args []string, s cli.Stdio) int {
	ns, code, done := newParser().Parse(args, s)
	if done {
		return code
	}
	return run(defaultEnv(), ns.String("niagara_home"), ns.String("station"), ns.String("host"),
		ns.String("scan_logs"), ns.Bool("json"), s)
}

func run(e *env, home, station, host, scanDir string, asJSON bool, s cli.Stdio) int {
	if scanDir != "" {
		return runScanLogs(scanDir, asJSON, s)
	}
	if !isDir(home) {
		fmt.Fprintf(s.Err, "ERROR: not a directory: %s\n", home)
		return 1
	}
	stationXML, rawHead := "", []byte(nil)
	if station != "" {
		var err error
		if stationXML, rawHead, err = readStation(station); err != nil {
			fmt.Fprintf(s.Err, "ERROR: --station: %s\n", err)
			return cli.ExitCannotSearch
		}
	}
	rep := &report{}
	if err := e.audit(home, stationXML, host, rawHead, rep); err != nil {
		// Python ends in a traceback when keytool cannot be started.
		fmt.Fprintf(s.Err, "%s: %s\n", prog, err)
		return cli.ExitCannotSearch
	}
	rep.dump(s.Out, asJSON)
	return cli.ExitFound
}

// readStation returns the BOG XML of the station file and its first sec12Window raw bytes.
func readStation(path string) (string, []byte, error) {
	text, err := bog.ReadText(path)
	if err != nil {
		return "", nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", nil, errors.New(pyOSError(err))
	}
	defer f.Close()
	buf := make([]byte, sec12Window)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", nil, errors.New(pyOSError(err))
	}
	return text, buf[:n], nil
}

func runScanLogs(dir string, asJSON bool, s cli.Stdio) int {
	if !isDir(dir) {
		fmt.Fprintf(s.Err, "ERROR: --scan-logs: not a directory: %s\n", dir)
		return cli.ExitCannotSearch
	}
	hits, nfiles, bad, badDirs := scanLogs(dir)
	if len(bad) > 0 {
		fmt.Fprintf(s.Err, "WARNING: --scan-logs: %d unreadable log file(s) not scanned:\n", len(bad))
		for _, u := range bad {
			fmt.Fprintf(s.Err, "  %s: %s\n", u.path, u.why)
		}
	}
	if len(badDirs) > 0 { // a directory os.walk would skip in silence is a blind spot too
		fmt.Fprintf(s.Err, "WARNING: --scan-logs: %d unreadable directory(ies) not scanned:\n", len(badDirs))
		for _, u := range badDirs {
			fmt.Fprintf(s.Err, "  %s: %s\n", u.path, u.why)
		}
	}
	if nfiles == 0 {
		fmt.Fprintf(s.Err, "ERROR: --scan-logs: 0 readable log files (*.log, console/station/nre/daemon "+
			"*.txt) under %s (%d unreadable)\n", dir, len(bad)+len(badDirs))
		return cli.ExitCannotSearch
	}
	if asJSON {
		objs := make([]string, len(hits))
		for i, h := range hits {
			objs[i] = jsonObject([][2]string{{"sev", pyJSONString(h.sev)}, {"ioc", pyJSONString(h.label)},
				{"file", pyJSONString(h.file)}, {"line", strconv.Itoa(h.line)}, {"text", pyJSONString(h.text)}})
		}
		fmt.Fprintln(s.Out, jsonList(objs))
	} else {
		fmt.Fprintf(s.Out, "\n  Log-IOC harvest — %d indicator(s) in %d log file(s) under %s\n\n", len(hits), nfiles, dir)
		for _, h := range sortHits(hits) {
			fmt.Fprintf(s.Out, "  %s[%s]%s %s  %s:%d\n         %s\n", color[h.sev], h.sev, color["off"],
				h.label, h.file, h.line, h.text)
		}
		switch {
		case len(hits) == 0 && len(badDirs) > 0:
			fmt.Fprintf(s.Out, "  no signature-bypass / unsigned-load IOCs in the readable files; "+
				"%d unreadable file(s) and %d unreadable directory(ies) not scanned (incomplete, not clean).\n",
				len(bad), len(badDirs))
		case len(hits) == 0 && len(bad) > 0:
			fmt.Fprintf(s.Out, "  no signature-bypass / unsigned-load IOCs in the readable files; "+
				"%d unreadable file(s) not scanned (incomplete, not clean).\n", len(bad))
		case len(hits) == 0:
			fmt.Fprintln(s.Out, "  no signature-bypass / unsigned-load IOCs found (clean).")
		}
		fmt.Fprintln(s.Out)
	}
	if len(bad) > 0 || len(badDirs) > 0 { // a partial scan is never clean: the exit says incomplete
		return cli.ExitCannotSearch
	}
	return cli.ExitFound
}

// toWinpath maps a WSL /mnt/<drive>/... path to its Windows form; "" for any other path.
func toWinpath(p string) string {
	m := winpathRE.FindStringSubmatch(p)
	if m == nil {
		return ""
	}
	return string(m[1][0]-'a'+'A') + ":\\" + replaceSlashes(m[2])
}

var winpathRE = regexp.MustCompile(`^/mnt/([a-z])/(.*)`)

func replaceSlashes(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c == '/' {
			b[i] = '\\'
		}
	}
	return string(b)
}
