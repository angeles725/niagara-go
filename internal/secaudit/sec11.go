package secaudit

// SEC-11 (weak signing keys / FIPS), as tools/lib/secaudit_keytool.py and the SEC-11 block of
// tools/niagara-security-audit.py (niagara-research 526526c72, 85475d27c).

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// entryCountRE is keytool -list's entry count line (JDK 8 and later, English locale):
// "Your keystore contains 1 entry".
var entryCountRE = regexp.MustCompile(`Your keystore contains ([0-9]+) entr(?:y|ies)`)

// keyLineRE reads one key per "Subject Public Key Algorithm:" line of keytool -list -v (JDK 8 and
// JDK 21): "2048-bit RSA key", "1024-bit RSA key (weak)", "256-bit EC (secp256r1) key", "255-bit
// Ed25519 key", "1024-bit RSASSA-PSS key" (JDK 11+; the whole hyphenated name is the algorithm). A
// key whose size keytool cannot tell has no "<n>-bit " prefix. The "Warning:" line that repeats a
// weak key's size ("<alias> uses a 1024-bit RSA key ...") is not read.
var keyLineRE = regexp.MustCompile(pycompat.RE2Space(
	`(?m)^\s*Subject Public Key Algorithm:\s*(?:([0-9]+)-bit\s+)?([A-Za-z0-9-]+)`))

// digits is a non-negative decimal integer as Python's str(int(s)) prints it: no leading zeros
// ("0" for zero), any length (Python ints have no size limit). "" stands for no value.
type digits string

func toDigits(s string) digits {
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	return digits(s)
}

// less compares two values numerically: without leading zeros, the shorter one is smaller.
func (d digits) less(o digits) bool {
	if len(d) != len(o) {
		return len(d) < len(o)
	}
	return d < o
}

// key is one "Subject Public Key Algorithm:" line: its algorithm and size ("" when keytool
// printed no size).
type key struct {
	alg  string
	bits digits
}

// parseKeys reads keytool -list -v's stdout: the keys in listing order and the entry count (""
// when the count line is missing, another output format).
func parseKeys(stdout string) (keys []key, entries digits) {
	for _, m := range keyLineRE.FindAllStringSubmatch(stdout, -1) {
		k := key{alg: m[2]}
		if m[1] != "" {
			k.bits = toDigits(m[1])
		}
		keys = append(keys, k)
	}
	if m := entryCountRE.FindStringSubmatch(stdout); m != nil {
		entries = toDigits(m[1])
	}
	return keys, entries
}

// sec11MinBits is the smallest key size, per algorithm, that is not weak (NIST SP 800-57 Part 1:
// 112-bit security is RSA/DSA 2048, EC 224).
var sec11MinBits = map[string]digits{"RSA": "2048", "DSA": "2048", "EC": "224"}

// sec11Family names the keys judged by another algorithm's threshold (JDK 11+: "2048-bit
// RSASSA-PSS key").
var sec11Family = map[string]string{"RSASSA-PSS": "RSA"}

// sec11Strong lists the algorithms strong (>= 128-bit security) at the sizes given, nil for any
// size: judged without a threshold. JDK 21 keytool prints X25519 and X448 keys as "255-bit XDH key"
// and "448-bit XDH key", so an XDH (or EdDSA) key is strong only at a size that names its curve.
var sec11Strong = map[string][]digits{"Ed25519": nil, "Ed448": nil, "X25519": nil, "X448": nil,
	"XDH": {"255", "448"}, "EdDSA": {"255", "448"}}

// sec11Secure is Python's sec11_secure_text(), derived there from SEC11_MIN_BITS.
const sec11Secure = "RSA/DSA keys >= 2048 bits, EC keys >= 224 bits; FIPS suppresses RSA-1024 option"

func family(alg string) string {
	if base, ok := sec11Family[alg]; ok {
		return base
	}
	return alg
}

// judge is "weak", "ok" or "unjudged" for one key: a threshold algorithm (through sec11Family) is
// weak below it and unjudged without a printed size; a sec11Strong algorithm is ok at its sizes;
// anything else is unjudged.
func judge(k key) string {
	if min, ok := sec11MinBits[family(k.alg)]; ok {
		switch {
		case k.bits == "":
			return "unjudged"
		case k.bits.less(min):
			return "weak"
		}
		return "ok"
	}
	if sizes, ok := sec11Strong[k.alg]; ok {
		if sizes == nil {
			return "ok"
		}
		for _, s := range sizes {
			if k.bits == s {
				return "ok"
			}
		}
	}
	return "unjudged"
}

// byAlgorithm is the SEC-11 text of keys: one group per algorithm, sorted by name, with its
// distinct sizes ascending and "size not printed" last; with mark, an algorithm neither under a
// threshold nor strong gets " (unknown algorithm)". [RSA 2048, EC 256, RSA 1024] ->
// "EC [256], RSA [1024, 2048]".
func byAlgorithm(keys []key, mark bool) string {
	sizes := map[string]map[digits]bool{}
	for _, k := range keys {
		if sizes[k.alg] == nil {
			sizes[k.alg] = map[digits]bool{}
		}
		sizes[k.alg][k.bits] = true
	}
	algs := make([]string, 0, len(sizes))
	for a := range sizes {
		algs = append(algs, a)
	}
	sort.Strings(algs) // the names are ASCII: byte order is Python's code point order
	groups := make([]string, 0, len(algs))
	for _, a := range algs {
		var shown []string
		var sorted []digits
		for b := range sizes[a] {
			if b != "" {
				sorted = append(sorted, b)
			}
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].less(sorted[j]) })
		for _, b := range sorted {
			shown = append(shown, string(b))
		}
		if sizes[a][""] {
			shown = append(shown, "size not printed")
		}
		_, threshold := sec11MinBits[family(a)]
		_, strong := sec11Strong[a]
		g := a + " [" + strings.Join(shown, ", ") + "]"
		if mark && !threshold && !strong {
			g += " (unknown algorithm)"
		}
		groups = append(groups, g)
	}
	return strings.Join(groups, ", ")
}

// sec11Verdict is the verdict and observed text (without the FIPS part) of a non-empty key list:
// any weak key FAILs; else any key it cannot judge SKIPs; else PASS. The observed text is "key
// sizes <all>", then "; weak <weak keys>" and "; not judged: <unjudged keys>" when there are any.
func sec11Verdict(keys []key) (verdict, observed string) {
	var weak, unjudged []key
	for _, k := range keys {
		switch judge(k) {
		case "weak":
			weak = append(weak, k)
		case "unjudged":
			unjudged = append(unjudged, k)
		}
	}
	observed = "key sizes " + byAlgorithm(keys, true)
	if len(weak) > 0 {
		observed += "; weak " + byAlgorithm(weak, false)
	}
	if len(unjudged) > 0 {
		observed += "; not judged: " + byAlgorithm(unjudged, false)
	}
	switch {
	case len(weak) > 0:
		return "FAIL", observed
	case len(unjudged) > 0:
		return "SKIP", observed
	}
	return "PASS", observed
}

// incomplete names what a listing of n key lines for entries entries ("" when keytool printed no
// count) left unread.
func incomplete(n int, entries digits) string {
	if entries == "" {
		return "entry count not found"
	}
	return "only " + strconv.Itoa(n) + pick(n == 1, " key line", " key lines") + " for " + string(entries) + " entries"
}

// sec11 PASSes only on keys keytool listed, or on a store keytool says is empty: no truststore, a
// failed keytool run, or entries without a parsed "Subject Public Key Algorithm:" line are SKIPs,
// never "no weak key". A key it cannot judge (unknown algorithm, no printed size) SKIPs too, and so
// does an otherwise passing listing with fewer key lines than entries.
func (e *env) sec11(home, ts string, rep *report) error {
	const (
		id, sev, title, block = "SEC-11", "med", "weak signing keys / FIPS", "B113/B392"
		runNote               = "run: keytool -list -v -keystore <home>/security/truststore.jks with the store password"
		passNote              = "RSA-1024 allowed with click-through warning"
	)
	fips := "FIPS keystore " + pick(pycompat.IsFile(filepath.Join(home, "jre", "lib", "security", "cacerts.bcfks")),
		"present", "absent")
	if !pycompat.IsFile(ts) {
		rep.add(id, sev, title, "not checked (no security/truststore.jks); "+fips, sec11Secure, "SKIP", block, runNote)
		return nil
	}
	keys, entries, failure, err := e.keytoolKeysizes(ts)
	if err != nil {
		return fmt.Errorf("keytool: %w", err)
	}
	switch {
	case failure != "": // the store did not open with "changeit": its key sizes are unknown, not absent
		rep.add(id, sev, title, "key sizes not read (keytool "+failure+"); "+fips, sec11Secure, "SKIP", block, runNote)
	case len(keys) == 0 && entries != "0":
		count := "entry count not found"
		if entries != "" {
			count = string(entries) + pick(entries == "1", " entry", " entries")
		}
		rep.add(id, sev, title, "key sizes not parsed from keytool -list -v ("+count+"); "+fips, sec11Secure,
			"SKIP", block, runNote)
	case len(keys) == 0:
		rep.add(id, sev, title, "no entries; "+fips, sec11Secure, "PASS", block, passNote)
	default:
		v, observed := sec11Verdict(keys)
		if v == "PASS" && (entries == "" || toDigits(strconv.Itoa(len(keys))).less(entries)) {
			// keytool -list -v prints one key line per certificate: an entry whose key line was not
			// read could hold a weak key (niagara-research 4dfd250f6)
			v, observed = "SKIP", observed+"; "+incomplete(len(keys), entries)
		}
		rep.add(id, sev, title, observed+"; "+fips, sec11Secure, v, block, pick(v == "SKIP", runNote, passNote))
	}
	return nil
}
