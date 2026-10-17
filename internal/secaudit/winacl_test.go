package secaudit

import (
	"reflect"
	"strings"
	"testing"
)

const testWinpath = `C:\N\security`

// encode is the inverse of a code page table (test helper; panics on an unmapped rune).
func encode(t *testing.T, text, page string) []byte {
	t.Helper()
	var upper []rune
	for _, cp := range codepages {
		if cp.name == page {
			upper = cp.upper
		}
	}
	var out []byte
	for _, r := range text {
		if r < 0x80 {
			out = append(out, byte(r))
			continue
		}
		i := -1
		for j, u := range upper {
			if u == r {
				i = j
				break
			}
		}
		if i < 0 {
			t.Fatalf("%q has no %s byte", r, page)
		}
		out = append(out, byte(0x80+i))
	}
	return out
}

func loose(t *testing.T, listing string) []string {
	t.Helper()
	aces, unparsed := parseListing(listing, testWinpath)
	if len(unparsed) != 0 {
		t.Fatalf("unparsed ACE lines: %q", unparsed)
	}
	return looseACEs(aces)
}

func TestWriteEquivalentRights(t *testing.T) {
	got := loose(t, testWinpath+" NT AUTHORITY\\SYSTEM:(OI)(CI)(F)\r\n"+
		"             BUILTIN\\Users:(OI)(CI)(IO)(GW)\r\n"+
		"             NT AUTHORITY\\Authenticated Users:(OI)(CI)(IO)(GA)\r\n")
	if want := []string{`BUILTIN\Users`, `NT AUTHORITY\Authenticated Users`}; !reflect.DeepEqual(got, want) {
		t.Errorf("generic rights: %q, want %q", got, want)
	}
	for _, right := range []string{"WD", "AD", "WEA", "WA", "WDAC", "WO", "D", "DE", "DC", "GA", "GW"} {
		if got := loose(t, testWinpath+" Everyone:(OI)(CI)(Rc,S,"+right+")\r\n"); !reflect.DeepEqual(got, []string{"Everyone"}) {
			t.Errorf("%s alone: %q", right, got)
		}
	}
	readOnly := testWinpath + " NT AUTHORITY\\SYSTEM:(OI)(CI)(F)\r\n" +
		"             BUILTIN\\Administrators:(OI)(CI)(IO)(GA)\r\n" +
		"             BUILTIN\\Users:(OI)(CI)(RX)\r\n" +
		"             BUILTIN\\Users:(OI)(CI)(IO)(GR,GE)\r\n" +
		"             Everyone:(Rc,S,REA,RA,X)\r\n"
	if got := loose(t, readOnly); len(got) != 0 {
		t.Errorf("read-only rights: %q", got)
	}
	localized := testWinpath + " NT-AUTORITÄT\\Authentifizierte Benutzer:(OI)(CI)(IO)(GW)\r\n" +
		"             AUTORITE NT\\Utilisateurs authentifiés:(CI)(WD)\r\n" +
		"             Jeder:(WDAC)\r\n             Tout le monde:(WO)\r\n" +
		"             NT AUTHORITY\\Usuarios autenticados:(AD)\r\n" +
		"             BUILTIN\\Usuários:(OI)(CI)(DC)\r\n             Todos:(DE)\r\n"
	if got := loose(t, localized); len(got) != 7 {
		t.Errorf("localized groups: %q", got)
	}
}

func TestMixedCaseRightsAreParsed(t *testing.T) {
	aces, _ := parseListing(testWinpath+" BUILTIN\\Users:(OI)(CI)(Rc,m)\r\n", testWinpath)
	if want := []ace{{`BUILTIN\Users`, []string{"OI", "CI", "RC,M"}}}; !reflect.DeepEqual(aces, want) {
		t.Errorf("aces = %q", aces)
	}
}

func TestDecodeConsole(t *testing.T) {
	texts := []string{
		testWinpath + "\\José\\Línea\\Fábrica BUILTIN\\Administradores:(OI)(CI)(F)\r\n",
		"C:\\Niagara\\ADMINISTRACIÓN BUILTIN\\Usuarios:(OI)(CI)(RX)\r\n",
		"C:\\Users\\José NT AUTHORITY\\SYSTEM:(F)\r\nSe procesó correctamente 1 archivos\r\n",
		"C:\\Planta\\Producción\\Señal NT-AUTORITÄT\\SYSTEM:(F)\r\n",
		"José Línea Fábrica", "José", "C:\\Straße NT AUTHORITY\\SYSTEM:(F)",
		"Se procesó correctamente 1 archivo; Configuración regional",
	}
	for _, text := range texts {
		for _, page := range []string{"cp1252", "cp850"} {
			if page == "cp850" && strings.Contains(text, "ß") {
				continue
			}
			if got := decodeConsole(encode(t, text, page)); got != text {
				t.Errorf("%s %q decoded as %q", page, text, got)
			}
		}
	}
	if plausibility("ADMINISTRACIÓN") <= plausibility("ADMINISTRACIàN") {
		t.Error("a lower letter between two uppers must score lower")
	}
	if got := decodeConsole([]byte("Everyone:(F) \x81\x8d\x90")); got != "Everyone:(F) üìÉ" {
		t.Errorf("bytes undefined in cp1252 still pick cp850: %q", got)
	}
	if got := decodeConsole([]byte("Everyone:(F)")); got != "Everyone:(F)" {
		t.Errorf("ascii: %q", got)
	}
	if got := decodeConsole([]byte("José")); got != "José" {
		t.Errorf("utf-8: %q", got)
	}
	if got := decodeConsole([]byte{0xff, 0xfe, 0xe9, 0x00}); got != "é" {
		t.Errorf("utf-16: %q", got)
	}
	if got := decodeConsole([]byte{0xfe, 0xff, 0x00, 0xe9, 0x00}); got != "é\ufffd" {
		t.Errorf("utf-16 BE odd: %q", got)
	}
}

// Lines holding ":(" that are not a well-formed ACE come back as unparsed (SEC-03 then SKIPs);
// other lines (the summary, blank lines) are ignored. Same result as winacl.parse_listing.
func TestParseListingUnparsedLines(t *testing.T) {
	listing := testWinpath + " NT AUTHORITY\\SYSTEM:(OI)(CI)(F)\r\n" +
		"   Everyone:(OI)(CI)(special access:)\r\n" +
		" odd:(F) junk\r\n" +
		":(F)\r\n" +
		"BUILTIN\\Users:(oi)(ci)(rx,w)\r\n" +
		"\r\n" +
		"Successfully processed 1 files; Failed processing 0 files\r\n"
	aces, unparsed := parseListing(listing, testWinpath)
	wantACEs := []ace{{`NT AUTHORITY\SYSTEM`, []string{"OI", "CI", "F"}}, {`BUILTIN\Users`, []string{"OI", "CI", "RX,W"}}}
	if !reflect.DeepEqual(aces, wantACEs) {
		t.Errorf("aces = %q, want %q", aces, wantACEs)
	}
	wantUnparsed := []string{"Everyone:(OI)(CI)(special access:)", "odd:(F) junk", ":(F)"}
	if !reflect.DeepEqual(unparsed, wantUnparsed) {
		t.Errorf("unparsed = %q, want %q", unparsed, wantUnparsed)
	}
}

// Every code page table covers the 128 bytes 0x80-0xff (a short table would panic or shift every
// later byte); spot checks pin a few well-known positions.
func TestCodepageTablesCoverTheUpperHalf(t *testing.T) {
	spots := map[string]map[byte]rune{
		"cp850":  {0x80: 'Ç', 0xa4: 'ñ', 0xb5: 'Á', 0xe0: 'Ó', 0xff: 0xa0},
		"cp437":  {0x80: 'Ç', 0x9e: '₧', 0xa4: 'ñ', 0xe1: 'ß', 0xff: 0xa0},
		"cp1252": {0x80: '€', 0x81: 0x81, 0x9f: 'Ÿ', 0xa0: 0xa0, 0xff: 'ÿ'},
	}
	if len(codepages) != len(spots) {
		t.Fatalf("%d code pages, want %d", len(codepages), len(spots))
	}
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	for _, cp := range codepages {
		if len(cp.upper) != 128 {
			t.Errorf("%s has %d runes, want 128", cp.name, len(cp.upper))
			continue
		}
		for b, want := range spots[cp.name] {
			if got := cp.upper[b-0x80]; got != want {
				t.Errorf("%s 0x%02x = %U, want %U", cp.name, b, got, want)
			}
		}
		if got := []rune(decodeCodepage(all, cp.upper)); len(got) != 256 || got[0x7f] != 0x7f || got[0x80] != cp.upper[0] {
			t.Errorf("%s decodes 256 bytes to %d runes", cp.name, len(got))
		}
	}
}
