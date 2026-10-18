package secaudit

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

var keyRingRE = regexp.MustCompile(`KeyRingPermission"[^>]*?(?:name|target)="([^"]*)"`)

// sec15 flags a wildcard KeyRingPermission declared by an unsigned module.
func sec15(home string, rep *report) {
	moddir := filepath.Join(home, "modules")
	if !pycompat.IsDir(moddir) {
		return
	}
	total, wild, wildUnsigned := scanKeyringPerms(moddir)
	risk := len(wildUnsigned) > 0
	rep.add("SEC-15", "med", "KeyRingPermission wildcard in unsigned module",
		strconv.Itoa(len(total))+" modules declare it, "+strconv.Itoa(len(wild))+" wildcard(name=*), "+
			strconv.Itoa(len(wildUnsigned))+" of those UNSIGNED",
		"no wildcard KeyRingPermission in an unsigned/attacker module", verdict(risk), "B114",
		pick(risk, "unsigned wildcard holders: "+strings.Join(wildUnsigned, ", "),
			"all wildcard holders are signed; risk only if an UNSIGNED module (SEC-01/06) declares name=*"))
}

// scanKeyringPerms reads modules/*.jar META-INF/module.xml: the KeyRingPermission holders, the
// wildcard holders and the wildcard holders without a signature file. Unreadable jars are skipped.
func scanKeyringPerms(moddir string) (all, wild, wildUnsigned []string) {
	entries, _ := os.ReadDir(moddir)
	for _, ent := range entries {
		f := ent.Name()
		if !strings.HasSuffix(f, ".jar") {
			continue
		}
		names, xml, ok := readModuleXML(filepath.Join(moddir, f))
		if !ok || !strings.Contains(xml, "KeyRingPermission") {
			continue
		}
		all = append(all, f)
		star := false
		for _, m := range keyRingRE.FindAllStringSubmatch(xml, -1) {
			star = star || m[1] == "*"
		}
		if !star {
			continue
		}
		wild = append(wild, f)
		signed := false
		for _, n := range names {
			u := strings.ToUpper(n)
			if strings.HasPrefix(n, "META-INF/") &&
				(strings.HasSuffix(u, ".RSA") || strings.HasSuffix(u, ".DSA") || strings.HasSuffix(u, ".EC")) {
				signed = true
			}
		}
		if !signed {
			wildUnsigned = append(wildUnsigned, f)
		}
	}
	return all, wild, wildUnsigned
}

// readModuleXML returns the member names and META-INF/module.xml of a jar; ok is false when the
// jar cannot be read or has no module.xml.
func readModuleXML(path string) (names []string, xml string, ok bool) {
	zr, err := zip.OpenReader(path)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, "", false
	}
	defer zr.Close()
	var member *zip.File
	for _, f := range zr.File {
		names = append(names, f.Name)
		if f.Name == "META-INF/module.xml" {
			member = f // Python's getinfo reads the last entry of a name
		}
	}
	if member == nil {
		return nil, "", false
	}
	rc, err := member.Open()
	if err != nil {
		return nil, "", false
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", false
	}
	return names, pycompat.DecodeReplace(data), true
}

// logIOC is one forensic indicator (B112 / B75).
type logIOC struct {
	sev, label string
	re         *regexp.Regexp
}

var logIOCs = []logIOC{
	{"crit", "module-validation-disabled banner", regexp.MustCompile(`(?i)Module validation has been DISABLED`)},
	{"crit", "module validation is disabled", regexp.MustCompile(`(?i)module validation is disabled`)},
	{"high", "unsigned module loaded (No code signers)", regexp.MustCompile(`(?i)No code signers for entry`)},
	{"high", "unsigned BProgram executed", regexp.MustCompile(`(?i)program\.notSigned|is not signed`)},
	{"med", "signature validation failure", regexp.MustCompile(`(?i)failed signature validation|Invalid signature`)},
	{"med", "self-signed cert not permitted", regexp.MustCompile(`(?i)Self signed signing certificate not permitted`)},
	{"med", "cert path validation failure", regexp.MustCompile(`(?i)CERT_PATH_VALIDATION_FAILURE|Error validating cert path`)},
}

var logNameRE = regexp.MustCompile(`(?i)(console|station|nre|daemon).*\.(log|txt)\n?$|\.log(\.[0-9]+)?\n?$`)

// hit is one IOC found in a log line.
type hit struct {
	sev, label, file string
	line             int
	text             string
}

// unreadable is a log that could not be read, with the OS reason.
type unreadable struct{ path, why string }

// scanLogs harvests the IOCs of the log files under root, walked top-down like os.walk
// (a directory's files before its subdirectories; names sorted, symlinked directories not entered).
// A log file that cannot be read lands in bad, a directory that cannot be listed in badDirs (with
// whatever entries were listed before the error still scanned): neither is ever counted as clean.
func scanLogs(root string) (hits []hit, nfiles int, bad, badDirs []unreadable) {
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil { // os.walk's os.scandir error: the Win32 text of FindFirstFileW on Windows
			badDirs = append(badDirs, unreadable{dir, pycompat.WinStrerrorOf(pycompat.WinListdirError(err))})
		}
		var subdirs []string
		for _, ent := range entries {
			p := pycompat.Join(dir, ent.Name())
			if isDirEntry(p, ent) {
				if ent.Type()&os.ModeSymlink == 0 {
					subdirs = append(subdirs, p)
				}
				continue
			}
			if !logNameRE.MatchString(ent.Name()) {
				continue
			}
			found, err := scanFile(p)
			if err != nil {
				bad = append(bad, unreadable{p, pycompat.StrerrorOf(err)})
				continue
			}
			nfiles++
			hits = append(hits, found...)
		}
		for _, d := range subdirs {
			walk(d)
		}
	}
	walk(root)
	return hits, nfiles, bad, badDirs
}

// isDirEntry is DirEntry.is_dir(): a symlink counts as its target.
func isDirEntry(p string, ent os.DirEntry) bool {
	if ent.Type()&os.ModeSymlink != 0 {
		return pycompat.IsDir(p)
	}
	return ent.IsDir()
}

func scanFile(p string) ([]hit, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var found []hit
	for i, line := range pycompat.Lines(pycompat.DecodeReplace(data)) {
		for _, ioc := range logIOCs {
			if ioc.re.MatchString(line) {
				found = append(found, hit{ioc.sev, ioc.label, p, i + 1, pycompat.Cut(pycompat.Strip(line), 160)})
			}
		}
	}
	return found, nil
}

// sortHits orders hits by severity, most severe first, keeping the scan order within one.
func sortHits(hits []hit) []hit {
	out := append([]hit(nil), hits...)
	sort.SliceStable(out, func(i, j int) bool { return sevRank[out[i].sev] > sevRank[out[j].sev] })
	return out
}
