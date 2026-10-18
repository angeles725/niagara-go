package secaudit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

var propRE = regexp.MustCompile(pycompat.RE2Space(`^\s*(#?)\s*(niagara\.[\p{L}\p{N}_.]+|program\.[\p{L}\p{N}_.]+)\s*=\s*(.*)`))

// prop is one system.properties value and whether its line was commented out.
type prop struct {
	value     string
	commented bool
}

// readProps reads defaults/system.properties then system.properties; a later line wins.
func readProps(home string) map[string]prop {
	props := map[string]prop{}
	for _, rel := range []string{"defaults/system.properties", "system.properties"} {
		p := filepath.Join(home, rel)
		if !pycompat.IsFile(p) {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range pycompat.Lines(pycompat.DecodeReplace(data)) {
			if m := propRE.FindStringSubmatch(line); m != nil {
				props[m[2]] = prop{pycompat.Strip(m[3]), m[1] == "#"}
			}
		}
	}
	return props
}

// getProp is props.get(key, (None, True)): an absent key reads as unset and commented.
func getProp(props map[string]prop, key string) (value string, set, commented bool) {
	p, ok := props[key]
	if !ok {
		return "", false, true
	}
	return p.value, true, p.commented
}

// sec12Window is the SEC-12 search span, in raw bytes and in unpacked XML characters.
const sec12Window = 4096

var sec12RE = regexp.MustCompile(pycompat.RE2Space(`(?i)EncryptionKeySource\s*["']?\s*[=:.]\s*["']?\s*(none|external|keyring)` +
	`(?:[^\p{L}\p{N}_]|$)`))

var (
	execRE    = regexp.MustCompile(`allowProgramRuntimeExec[^>]*true`)
	enabledRE = regexp.MustCompile(`enabled[^>]*true`)
	licAttrs  = []string{"skipModuleValidation", "smDeveloperMode", "unreleasedSoftware"}
)

func verdict(bad bool) string {
	if bad {
		return "FAIL"
	}
	return "PASS"
}

func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// audit runs every check on home. stationXML is the station's BOG XML ("" without --station);
// rawHead its first sec12Window bytes on disk.
func (e *env) audit(home, stationXML, host string, rawHead []byte, rep *report) error {
	props := readProps(home)

	v, set, com := getProp(props, "niagara.moduleVerificationMode")
	bad := !set || com || strings.ToLower(v) == "low"
	rep.add("SEC-01", "crit", "moduleVerificationMode", pick(v != "", v, "(unset)")+pick(com, " [commented]", ""),
		"high", verdict(bad), "B75/B397",
		"low lets an UNSIGNED module requesting NETWORK_COMMUNICATION load (the 443 incident)")

	ts := pycompat.PosixJoin(home, "security", "truststore.jks")
	d, err := e.keytoolDefault(ts)
	if err != nil {
		return fmt.Errorf("keytool: %w", err)
	}
	if d != nil {
		observed, verd, note := pick(d.opened, "opens with 'changeit'", "custom password"), verdict(d.opened),
			"FS/daemon access -> trust-anchor injection"
		if !d.opened && !d.wrongPass { // keytool failed for another reason: the password was not checked
			observed, verd, note = "not verified (keytool "+d.failure+")", "SKIP",
				"run: keytool -list -keystore <home>/security/truststore.jks"
		}
		rep.add("SEC-02", "crit", "truststore.jks default password", observed, "non-default store password",
			verd, "B392/B397", note)
	}

	if err := e.sec03(home, rep); err != nil {
		return fmt.Errorf("icacls: %w", err)
	}

	if host != "" {
		for _, p := range []struct {
			port int
			name string
		}{{443, "station HTTPS"}, {5011, "platform TLS"}} {
			s, err := e.tlsSubject(host, p.port)
			if err != nil {
				return fmt.Errorf("openssl: %w", err)
			}
			if s != "" {
				def := strings.Contains(s, "ForRecoveryPurposes")
				rep.add("SEC-04:"+strconv.Itoa(p.port), "crit", p.name+" certificate",
					pick(def, "default self-signed (ForRecoveryPurposes)", s), "CA-issued cert, replaced",
					verdict(def), "B397/B156", "default cert = no real TLS trust, MITM-able on mgmt LAN")
			}
		}
	}

	v, _, com = getProp(props, "niagara.commandLinePropertyBlacklist")
	covered := v != "" && !com && strings.Contains(v, "skipModuleValidation") && strings.Contains(v, "ignoreVerificationMode")
	rep.add("SEC-05", "high", "commandLinePropertyBlacklist covers skip levers",
		pick(v == "" || com, "disabled/absent", pick(!covered, "partial", "full")),
		"includes skipModuleValidation + commissioning.ignoreVerificationMode",
		verdict(!covered), "B113", "-Dniagara.classLoader.skipModuleValidation can disable chain validation at launch")

	e.sec06(home, rep)

	v, set, com = getProp(props, "program.requireSigning")
	bad = !set || com || strings.ToLower(v) == "false"
	rep.add("SEC-07", "high", "program.requireSigning (BProgram bytecode)", pick(bad, "false/default", v),
		"true", verdict(bad), "B75", "superuser BProgram runs arbitrary bytecode unsigned")

	if stationXML != "" {
		execOn := execRE.MatchString(stationXML)
		rep.add("SEC-08", "high", "allowProgramRuntimeExec", pick(execOn, "true", "false/default"),
			"false", verdict(execOn), "B75", "Runtime.exec() from programs")
		sysOn := (strings.Contains(stationXML, "SyslogService") || strings.Contains(stationXML, "BSyslogSettings")) &&
			enabledRE.MatchString(stationXML)
		rep.add("SEC-10", "high", "syslog offload to external SIEM", pick(sysOn, "enabled", "disabled/absent"),
			"enabled + serverHost + TLS transport", verdict(!sysOn), "B75/B393/B396",
			"the ONLY tamper-resistance for the (unsigned) local audit/history record")
	} else {
		rep.add("SEC-08", "high", "allowProgramRuntimeExec", "not checked (no --station config.bog)", "-",
			"MANUAL", "B75/B396", "pass --station <config.bog>")
		rep.add("SEC-10", "high", "syslog offload to SIEM", "not checked (no --station config.bog)", "-",
			"MANUAL", "B75/B396", "pass --station <config.bog>")
	}

	if host != "" && e.dial(host, 3011) {
		loopback := host == "127.0.0.1" || host == "localhost" || host == "::1"
		rep.add("SEC-09", pick(loopback, "med", "high"), "platform daemon plaintext port 3011",
			"3011 open"+pick(loopback, " (loopback only, mitigated)", " on reachable iface"),
			"sslOnly=true (5011 only)", "FAIL", "B75/B397", "platform creds in cleartext if used")
	}

	if err := e.sec11(home, ts, rep); err != nil {
		return err
	}
	if stationXML != "" {
		sec12(stationXML, rawHead, rep)
	}
	if host != "" {
		for _, p := range []struct {
			port      int
			name, blk string
		}{{1911, "Fox plaintext", "B397/B134"}, {80, "HTTP plaintext", "B397"}} {
			if e.dial(host, p.port) {
				ps := strconv.Itoa(p.port)
				rep.add("SEC-14:"+ps, "med", p.name+" port "+ps, ps+" open", "TLS-only (4911/443)", "FAIL",
					p.blk, "station comms/UI in cleartext")
			}
		}
	}
	sec15(home, rep)
	rep.add("SEC-16", "med", "local data record integrity (audit/history/backup/.bog)",
		"unsigned by design (architectural)", "syslog offload (SEC-10) = resistance, not evidence",
		"FAIL", "B393/B396", "Niagara signs code, not data; no toggle fixes this - audit via SEC-10")
	return nil
}

func (e *env) sec06(home string, rep *report) {
	licdir := filepath.Join(home, "security", "licenses")
	if !pycompat.IsDir(licdir) {
		return
	}
	hits := map[string]bool{}
	entries, _ := os.ReadDir(licdir)
	for _, ent := range entries {
		f := ent.Name()
		if !strings.HasSuffix(f, ".license") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(licdir, f))
		if err != nil {
			continue
		}
		t := pycompat.DecodeReplace(data)
		for _, attr := range licAttrs {
			re := regexp.MustCompile(pycompat.RE2Space(attr + `\s*=\s*"?true`))
			if re.MatchString(t) || strings.Contains(t, attr) && strings.Contains(t, "developer") {
				hits[f+":"+attr] = true
			}
		}
	}
	sorted := make([]string, 0, len(hits))
	for h := range hits {
		sorted = append(sorted, h)
	}
	sort.Strings(sorted)
	rep.add("SEC-06", "high", "license attributes relaxing signature",
		pick(len(sorted) > 0, strings.Join(sorted, ", "), "none"), "no developer/unreleased/smDeveloper flags",
		verdict(len(sorted) > 0), "B75/B113",
		"developer{skipModuleValidation}/unreleasedSoftware accept self-signed / disable validation")
}

// sec12 looks for a structured EncryptionKeySource marker in the raw header and the XML prologue.
func sec12(stationXML string, rawHead []byte, rep *report) {
	raw := make([]rune, len(rawHead))
	for i, b := range rawHead {
		raw[i] = rune(b) // latin-1
	}
	type place struct {
		name string
		vals []string
	}
	var found []place
	all := map[string]bool{}
	for _, p := range []struct{ name, text string }{{"raw header", string(raw)},
		{"XML prologue", pycompat.Cut(stationXML, sec12Window)}} {
		seen := map[string]bool{}
		for _, m := range sec12RE.FindAllStringSubmatch(p.text, -1) {
			seen[strings.ToLower(m[1])] = true
		}
		if len(seen) == 0 {
			continue
		}
		vals := make([]string, 0, len(seen))
		for v := range seen {
			vals = append(vals, v)
			all[v] = true
		}
		sort.Strings(vals)
		found = append(found, place{p.name, vals})
	}
	encrypted := len(all) > 0 && !all["none"]
	var observed string
	if len(all) == 0 {
		observed = "no EncryptionKeySource=<none|external|keyring> marker in raw header or XML prologue (plaintext)"
	} else {
		parts := make([]string, len(found))
		for i, p := range found {
			parts[i] = "EncryptionKeySource=" + strings.Join(p.vals, "/") + " in " + p.name
		}
		observed = strings.Join(parts, "; ")
		if !encrypted {
			observed += " (none = plaintext)"
		}
	}
	rep.add("SEC-12", "med", "config.bog at-rest encryption", observed, "keyring", verdict(!encrypted), "B114",
		"EncryptionKeySource=none leaves reversible passwords in the clear")
}
