package secaudit

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// SEC-11 (weak signing keys / FIPS), ported from niagara-research tools/tests/test_secaudit_sec11.py
// (526526c72, 85475d27c). It PASSes only on keys keytool listed, or on a store keytool says is
// empty; each key is judged by its own algorithm, and a key it cannot judge is a SKIP. The keytool
// lines are the real JDK 8 and JDK 21 texts.

const (
	wantSecure   = "RSA/DSA keys >= 2048 bits, EC keys >= 224 bits; FIPS suppresses RSA-1024 option"
	sec11RunNote = "run: keytool -list -v -keystore <home>/security/truststore.jks with the store password"
	sec11Pass    = "RSA-1024 allowed with click-through warning"

	ktHead     = "Keystore type: JKS\nKeystore provider: SUN\n\n"
	key2048    = "Signature algorithm name: SHA256withRSA\nSubject Public Key Algorithm: 2048-bit RSA key\n"
	key1024    = "Subject Public Key Algorithm: 1024-bit RSA key (weak)\n"
	warn1024   = "Warning:\n<r1024> uses a 1024-bit RSA key which is considered a security risk. This key size will be disabled in a future update.\n"
	keyEC256   = "Signature algorithm name: SHA384withECDSA\nSubject Public Key Algorithm: 256-bit EC (secp256r1) key\n"
	keyEC192   = "Subject Public Key Algorithm: 192-bit EC (secp192r1) key (weak)\n"
	keyEd25519 = "Signature algorithm name: Ed25519\nSubject Public Key Algorithm: 255-bit Ed25519 key\n"
	keyDSA1024 = "Subject Public Key Algorithm: 1024-bit DSA key (weak)\n"
	keyDSA2048 = "Subject Public Key Algorithm: 2048-bit DSA key\n"
	keyNoSize  = "Subject Public Key Algorithm: XDH key of unknown size\n"
	// JDK 21 keytool -list -v of keys made with -keyalg RSASSA-PSS/Ed448/X25519/X448: X25519 and
	// X448 keys print as XDH, told apart only by their size.
	keyPSS1024    = "Subject Public Key Algorithm: 1024-bit RSASSA-PSS key\n"
	keyPSS2048    = "Subject Public Key Algorithm: 2048-bit RSASSA-PSS key\n"
	keyEd448      = "Subject Public Key Algorithm: 448-bit Ed448 key\n"
	keyX25519     = "Subject Public Key Algorithm: 255-bit XDH key\n"
	keyX448       = "Subject Public Key Algorithm: 448-bit XDH key\n"
	keyRSANoSize  = "Subject Public Key Algorithm: RSA key\n"
	keyFoo        = "Subject Public Key Algorithm: 512-bit FOO key\n"
	fipsAbsent    = "; FIPS keystore absent"
	notParsedText = "key sizes not parsed from keytool -list -v ("
)

// sec11Row is the SEC-11 row of a home whose truststore keytool -list -v lists as listing; a nil
// listing is a home without a truststore.
func sec11Row(t *testing.T, listing *string) row {
	t.Helper()
	home := t.TempDir()
	out := ""
	if listing != nil {
		home = homeWithTruststore(t)
		out = *listing
	}
	return rowsByID(t, probeEnv("", out, "", nil), home, "")["SEC-11"]
}

// judged is the SEC-11 row of a store listing keys, with one entry per key line.
func judged(t *testing.T, keys string) row {
	t.Helper()
	return judgedN(t, keys, strings.Count(keys, "Subject Public Key Algorithm:"))
}

// judgedN is the SEC-11 row of a store of entries entries listing keys.
func judgedN(t *testing.T, keys string, entries int) row {
	t.Helper()
	word := "1 entry"
	if entries != 1 {
		word = strconv.Itoa(entries) + " entries"
	}
	l := ktHead + "Your keystore contains " + word + "\n\n" + keys
	return sec11Row(t, &l)
}

func assertSec11(t *testing.T, r row, observed, verdict, note string) {
	t.Helper()
	got := [5]string{r.observed, r.secure, r.verdict, r.block, r.note}
	want := [5]string{observed, wantSecure, verdict, "B113/B392", note}
	if got != want {
		t.Errorf("SEC-11 =\n %q\nwant\n %q", got, want)
	}
}

func TestSec11NoTruststoreIsASkipNotAPass(t *testing.T) {
	assertSec11(t, sec11Row(t, nil), "not checked (no security/truststore.jks)"+fipsAbsent, "SKIP", sec11RunNote)
}

func TestSec11NoTruststoreReportsAPresentFIPSKeystore(t *testing.T) {
	home := t.TempDir()
	secdir := filepath.Join(home, "jre", "lib", "security")
	if err := os.MkdirAll(secdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secdir, "cacerts.bcfks"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := rowsByID(t, probeEnv("", "", "", nil), home, "")["SEC-11"]
	assertSec11(t, r, "not checked (no security/truststore.jks); FIPS keystore present", "SKIP", sec11RunNote)
}

func TestSec11EntriesWithoutAParsedSizeAreASkip(t *testing.T) {
	for _, word := range []string{"1 entry", "3 entries"} {
		l := ktHead + "Your keystore contains " + word + "\n\nAlias name: a\nOwner: CN=a\n"
		assertSec11(t, sec11Row(t, &l), notParsedText+word+")"+fipsAbsent, "SKIP", sec11RunNote)
	}
}

func TestSec11MissingEntryCountWithoutAParsedSizeIsASkip(t *testing.T) {
	l := "Tipo de almacén de claves: JKS\nSu almacén de claves contiene 1 entrada\n"
	assertSec11(t, sec11Row(t, &l), notParsedText+"entry count not found)"+fipsAbsent, "SKIP", sec11RunNote)
}

func TestSec11EmptyStoreIsAnHonestPass(t *testing.T) {
	l := ktHead + "Your keystore contains 0 entries\n\n"
	assertSec11(t, sec11Row(t, &l), "no entries"+fipsAbsent, "PASS", sec11Pass)
}

func TestSec11JudgesEachKeyByItsAlgorithm(t *testing.T) {
	for _, tc := range []struct {
		name, keys, observed, verdict string
	}{
		{"rsa 2048", key2048, "key sizes RSA [2048]", "PASS"},
		{"rsa 1024 and 2048", key2048 + key1024, "key sizes RSA [1024, 2048]; weak RSA [1024]", "FAIL"},
		{"ec 256 alone", keyEC256, "key sizes EC [256]", "PASS"},
		{"rsa 1024 beside ec 256, warning line ignored", keyEC256 + key1024 + warn1024,
			"key sizes EC [256], RSA [1024]; weak RSA [1024]", "FAIL"},
		{"ec below 224", keyEC192 + key2048, "key sizes EC [192], RSA [2048]; weak EC [192]", "FAIL"},
		{"dsa 2048", keyDSA2048, "key sizes DSA [2048]", "PASS"},
		{"dsa 1024", keyDSA1024 + keyDSA2048, "key sizes DSA [1024, 2048]; weak DSA [1024]", "FAIL"},
		{"rsassa-pss 1024", keyPSS1024, "key sizes RSASSA-PSS [1024]; weak RSASSA-PSS [1024]", "FAIL"},
		{"rsassa-pss 2048", keyPSS2048, "key sizes RSASSA-PSS [2048]", "PASS"},
		{"ed25519", keyEd25519, "key sizes Ed25519 [255]", "PASS"},
		{"strong curves", keyEd25519 + keyEd448 + keyX25519 + keyX448,
			"key sizes Ed25519 [255], Ed448 [448], XDH [255, 448]", "PASS"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertSec11(t, judged(t, tc.keys), tc.observed+fipsAbsent, tc.verdict, sec11Pass)
		})
	}
}

// A key SEC-11 cannot judge (unknown algorithm, a threshold algorithm without a size, XDH of no
// curve size) is a SKIP, never a PASS; a weak key still FAILs beside it.
func TestSec11NeverPassesOnAKeyItCannotJudge(t *testing.T) {
	for _, tc := range []struct {
		name, keys              string
		entries                 int
		observed, verdict, note string
	}{
		{"unsized xdh", keyEd25519 + keyNoSize + key2048, 3,
			"key sizes Ed25519 [255], RSA [2048], XDH [size not printed]; not judged: XDH [size not printed]",
			"SKIP", sec11RunNote},
		{"rsa without a size", keyRSANoSize + key2048, 2,
			"key sizes RSA [2048, size not printed]; not judged: RSA [size not printed]", "SKIP", sec11RunNote},
		{"unknown algorithm", keyFoo + key2048, 2,
			"key sizes FOO [512] (unknown algorithm), RSA [2048]; not judged: FOO [512]", "SKIP", sec11RunNote},
		{"weak beside unknown", keyFoo + key1024, 2,
			"key sizes FOO [512] (unknown algorithm), RSA [1024]; weak RSA [1024]; not judged: FOO [512]",
			"FAIL", sec11Pass},
		{"strong beside unknown", keyEd25519 + keyEC256 + keyFoo, 3,
			"key sizes EC [256], Ed25519 [255], FOO [512] (unknown algorithm); not judged: FOO [512]",
			"SKIP", sec11RunNote},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertSec11(t, judgedN(t, tc.keys, tc.entries), tc.observed+fipsAbsent, tc.verdict, tc.note)
		})
	}
}

// keytool -list -v prints one key line per certificate: an entry whose key line was not read
// could hold a weak key, so fewer key lines than entries (or no entry count) is no PASS; a weak
// key still FAILs (niagara-research 4dfd250f6).
func TestSec11UnreadEntriesAreNoPass(t *testing.T) {
	assertSec11(t, judgedN(t, keyEC256, 2), "key sizes EC [256]; only 1 key line for 2 entries"+fipsAbsent,
		"SKIP", sec11RunNote)
	assertSec11(t, judgedN(t, keyEC256+keyEd25519, 3),
		"key sizes EC [256], Ed25519 [255]; only 2 key lines for 3 entries"+fipsAbsent, "SKIP", sec11RunNote)
	assertSec11(t, judgedN(t, key1024+warn1024, 3), "key sizes RSA [1024]; weak RSA [1024]"+fipsAbsent,
		"FAIL", sec11Pass)
	l := ktHead + keyEC256
	assertSec11(t, sec11Row(t, &l), "key sizes EC [256]; entry count not found"+fipsAbsent, "SKIP", sec11RunNote)
	// A count padded with zeros is read as Python's int() does.
	l = ktHead + "Your keystore contains 02 entries\n\n" + keyEC256
	assertSec11(t, sec11Row(t, &l), "key sizes EC [256]; only 1 key line for 2 entries"+fipsAbsent,
		"SKIP", sec11RunNote)
}

// Sizes are compared as numbers of any length, printed without leading zeros (Python int()).
func TestSec11SizesAreReadAsPythonInts(t *testing.T) {
	keys := "Subject Public Key Algorithm: 01024-bit RSA key\n" +
		"Subject Public Key Algorithm: 99999999999999999999999-bit RSA key\n" +
		"Subject Public Key Algorithm: 0255-bit XDH key\n"
	assertSec11(t, judged(t, keys),
		"key sizes RSA [1024, 99999999999999999999999], XDH [255]; weak RSA [1024]"+fipsAbsent, "FAIL", sec11Pass)
}

func TestSec11WarningLineIsNotAKey(t *testing.T) {
	assertSec11(t, judgedN(t, warn1024, 1), notParsedText+"1 entry)"+fipsAbsent, "SKIP", sec11RunNote)
}
