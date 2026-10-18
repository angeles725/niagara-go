package secaudit

// SEC-11 (weak signing keys / FIPS), as tools/lib/secaudit_keytool.py and the SEC-11 block of
// tools/niagara-security-audit.py (niagara-research 526526c72, 85475d27c, bdc5e24a1).

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

// keytool -list -v prints one block per entry, from an "Alias name: <alias>" line to the next (JDK 8
// and JDK 21), with "Entry type: PrivateKeyEntry|trustedCertEntry|SecretKeyEntry" in it. A
// PrivateKeyEntry prints one key line per certificate of its chain ("Certificate chain length: N"),
// the first being the entry's own key; a trustedCertEntry prints one; a SecretKeyEntry none (it has
// no public key). The entry type is the rest of its line (a Windows "\r" stripped): any other text
// is another type.
// A PrivateKeyEntry's "Certificate chain length: N" line, which keytool -list -v (English) always
// prints, with one key line per certificate (a Windows "\r" allowed): JDK 8 and JDK 21 stores with
// chains of 1, 2 and 3 printed exactly N key lines, a trustedCertEntry exactly one (decision D34).
var (
	aliasRE       = regexp.MustCompile(`(?m)^Alias name: `)
	entryTypeRE   = regexp.MustCompile(`(?m)^Entry type: (.*)$`)
	chainLengthRE = regexp.MustCompile(`(?m)^Certificate chain length: ([0-9]+)\r?$`)
)

// sec11EntryNeedsKey lists the entry types keytool prints, and whether each must print a key line.
var sec11EntryNeedsKey = map[string]bool{"PrivateKeyEntry": true, "trustedCertEntry": true, "SecretKeyEntry": false}

// entryBlock is one "Alias name:" block: its entry type (typed false when the block has no "Entry
// type:" line), its number of key lines and its certificate chain length ("" when the block has no
// "Certificate chain length:" line).
type entryBlock struct {
	typ   string
	typed bool
	keys  int
	chain digits
}

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

// parseKeys reads keytool -list -v's stdout: the keys in listing order, the entry count (""
// when the count line is missing, another output format) and the entry blocks in listing order.
func parseKeys(stdout string) (keys []key, entries digits, blocks []entryBlock) {
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
	for _, text := range aliasRE.Split(stdout, -1)[1:] {
		b := entryBlock{keys: len(keyLineRE.FindAllStringIndex(text, -1))}
		if m := entryTypeRE.FindStringSubmatch(text); m != nil {
			b.typ, b.typed = strings.TrimRight(m[1], "\r"), true
		}
		if m := chainLengthRE.FindStringSubmatch(text); m != nil {
			b.chain = toDigits(m[1])
		}
		blocks = append(blocks, b)
	}
	return keys, entries, blocks
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

// entriesText is "1 entry" or "<n> entries", with what before the noun: "1 secret key entry".
func entriesText(n digits, what string) string {
	return string(n) + " " + what + pick(n == "1", "entry", "entries")
}

// sec11Gaps names what keeps a listing from proving every key read, nil when nothing does: no entry
// count, alias blocks other than entries, a PrivateKeyEntry or trustedCertEntry block without a key
// line, a PrivateKeyEntry without its chain length or with fewer key lines than it, a block with
// more key lines than certificates (a trustedCertEntry has one: another format), a block of another
// (or no) entry type. Each block lands in one count at most. Counts only: alias names can be
// sensitive.
func sec11Gaps(entries digits, blocks []entryBlock) []string {
	var gaps []string
	n := toDigits(strconv.Itoa(len(blocks)))
	switch {
	case entries == "":
		gaps = append(gaps, "entry count not found")
	case n != entries:
		gaps = append(gaps, string(n)+pick(n == "1", " alias block", " alias blocks")+" for "+entriesText(entries, ""))
	}
	kinds := []string{"without a key line", "without a certificate chain length", "with unread chain certificates",
		"with more key lines than certificates", "of unknown type"}
	counts := map[string]int{}
	for _, b := range blocks {
		needs, known := sec11EntryNeedsKey[b.typ]
		certs := digits("1")
		if b.typ == "PrivateKeyEntry" {
			certs = b.chain
		}
		lines := toDigits(strconv.Itoa(b.keys))
		switch {
		case !b.typed || !known:
			counts["of unknown type"]++
		case !needs:
		case b.keys == 0:
			counts["without a key line"]++
		case certs == "":
			counts["without a certificate chain length"]++
		case lines.less(certs):
			counts["with unread chain certificates"]++
		case certs.less(lines):
			counts["with more key lines than certificates"]++
		}
	}
	for _, k := range kinds {
		if counts[k] > 0 {
			gaps = append(gaps, entriesText(toDigits(strconv.Itoa(counts[k])), "")+" "+k)
		}
	}
	return gaps
}

// sec11Listing is the verdict and observed text (without the FIPS part) of a listing keytool
// printed with exit 0. No key line where entries need one: SKIP "key sizes not parsed". Else
// sec11Verdict's, plus "N secret key entr(y|ies) (no public key)" (never a verdict of its own); "no
// entries" for an empty store. A PASS whose listing has sec11Gaps is a SKIP naming them: an entry
// whose key line was not read could hold a weak key (niagara-research 4dfd250f6); a weak key FAILs.
func sec11Listing(keys []key, entries digits, blocks []entryBlock) (verdict, observed string) {
	gaps := sec11Gaps(entries, blocks)
	if len(keys) == 0 && entries != "0" && len(gaps) > 0 {
		count := "entry count not found"
		if entries != "" {
			count = entriesText(entries, "")
		}
		return "SKIP", "key sizes not parsed from keytool -list -v (" + count + ")"
	}
	verdict = "PASS"
	var parts []string
	if len(keys) > 0 {
		var o string
		verdict, o = sec11Verdict(keys)
		parts = append(parts, o)
	}
	secrets := 0
	for _, b := range blocks {
		if b.typed && b.typ == "SecretKeyEntry" {
			secrets++
		}
	}
	if secrets > 0 {
		parts = append(parts, entriesText(toDigits(strconv.Itoa(secrets)), "secret key ")+" (no public key)")
	}
	if verdict == "PASS" && len(gaps) > 0 {
		verdict, parts = "SKIP", append(parts, gaps...)
	}
	if len(parts) == 0 {
		return verdict, "no entries"
	}
	return verdict, strings.Join(parts, "; ")
}

// sec11 PASSes only on keys keytool listed, or on a store keytool says is empty: no truststore, a
// failed keytool run, or entries without a parsed "Subject Public Key Algorithm:" line are SKIPs,
// never "no weak key". A key it cannot judge (unknown algorithm, no printed size) SKIPs too, and so
// does an otherwise passing listing an entry of which printed no key line (sec11Listing).
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
	keys, entries, blocks, failure, err := e.keytoolKeysizes(ts)
	if err != nil {
		return fmt.Errorf("keytool: %w", err)
	}
	if failure != "" { // the store did not open with "changeit": its key sizes are unknown, not absent
		rep.add(id, sev, title, "key sizes not read (keytool "+failure+"); "+fips, sec11Secure, "SKIP", block, runNote)
		return nil
	}
	v, observed := sec11Listing(keys, entries, blocks)
	rep.add(id, sev, title, observed+"; "+fips, sec11Secure, v, block, pick(v == "SKIP", runNote, passNote))
	return nil
}
