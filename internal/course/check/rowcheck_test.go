package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every spec row is well formed (validate), and a malformed row is SKIP with the reason, never run.
func TestRowValidation(t *testing.T) {
	for i := range Spec {
		if err := Spec[i].validate(); err != nil {
			t.Errorf("%s: %v", Spec[i].ID, err)
		}
	}
	st, err := Open(writeStation(t), "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		ck   Check
		want string
	}{
		{Check{Kind: kindSlot, Root: rootLabsrc, Type: numericWritable, Slot: "out", Want: "1"}, "bad row: Root labsrc on kind slot"},
		{Check{Kind: kindFileText, Glob: "shared/a", Match: ".", Whole: true, Line: 1}, "bad row: Whole and Line together"},
		{Check{Kind: kindFiles, Root: rootInstall, Glob: "{station}.txt", N: 1}, "bad row: {station} outside the station root"},
		{Check{Kind: kindAnnotation, Root: rootInstall, Glob: "A.java", Fact: factSlots, Want: "x"}, "bad row: Root install on kind annotation"},
		{Check{Kind: kindFiles, Root: "elsewhere", Glob: "a", N: 1}, "bad row: unknown Root elsewhere"},
		{Check{Kind: kindFileText, Glob: "shared/a", Match: "(.)", Names: "C", Op: opMin, N: 1}, "bad row: Names without a Whole file-text row"},
	} {
		if r := st.Run(&c.ck); r.Verdict != Skip || r.Evidence != c.want {
			t.Errorf("%+v: %s %q, want SKIP %q", c.ck, r.Verdict, r.Evidence, c.want)
		}
	}
}

// A station name holding glob or brace characters matches only itself: {station} is replaced after
// the row's own {a,b} groups are expanded. The name uses characters every OS allows in a file name.
func TestStationNameBraces(t *testing.T) {
	dir := writeStation(t)
	bog, err := os.ReadFile(filepath.Join(dir, "config.bog"))
	if err != nil {
		t.Fatal(err)
	}
	name := "Mi{a,b}[ab]"
	if err := os.WriteFile(filepath.Join(dir, "config.bog"), []byte(strings.Replace(string(bog), `v="Mini"`, `v="`+name+`"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{name + "_1.csv", "Mia_1.csv", "Mib_1.csv", "Mi{a,b}a_1.csv", name + "_2.txt"} {
		if err := os.WriteFile(filepath.Join(dir, "shared", f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := Open(dir, "", false)
	if err != nil {
		t.Fatal(err)
	}
	ck := Check{Kind: kindFiles, Glob: "shared/{station}_{1.csv,2.txt}", Op: opEq, N: 2}
	if r := st.Run(&ck); r.Verdict != Pass || r.Evidence != "found 2 shared/"+name+"_{1.csv,2.txt}: "+name+"_1.csv … "+name+"_2.txt" {
		t.Errorf("%s %q", r.Verdict, r.Evidence)
	}
}

// C5.7 reads the dname's attribute names: exactly C, CN, L, O, ST once each; a duplicate, a missing
// one or an OU fails.
func TestDnameAttributes(t *testing.T) {
	for _, c := range []struct {
		dname, verdict string
	}{
		{"C=MX,ST=Chihuahua,L=Chihuahua,O=Veytrix,CN=${alias}", Pass},
		{"CN=${alias}, O=Veytrix, L=City, ST=State, C=XX", Pass},
		{"C=MX,C=US,L=Chihuahua,O=Veytrix,CN=${alias}", Fail},
		{"C=MX,ST=S,L=L,OU=Unit,CN=${alias}", Fail},
		{"C=MX,ST=S,L=L,O=O,OU=Unit,CN=${alias}", Fail},
		{"C=MX,ST=S,L=L,CN=${alias}", Fail},
	} {
		labs := writeTree(t, map[string]string{"certificates/code_signing.xml": "<properties>\n<entry key=\"niagara.signing.dname\">" +
			c.dname + "</entry>\n</properties>\n", "dev/": ""})
		st := &Station{labsrc: filepath.Join(labs, "dev")}
		if r := st.Run(specRow("C5.7")); r.Verdict != c.verdict {
			t.Errorf("%s: %s %q, want %s", c.dname, r.Verdict, r.Evidence, c.verdict)
		}
	}
}

// attrNames splits a DN on the commas between attributes only: an escaped comma (RFC 2253 \,) or a
// comma inside a quoted value is part of the value.
func TestAttrNames(t *testing.T) {
	for in, want := range map[string]string{
		"C=MX,ST=S,L=L,O=O,CN=x":               "C CN L O ST",
		`C=MX,ST=S,L=L,O=Veytrix\, Inc.,CN=x`:  "C CN L O ST",
		`C=MX,ST=S,L=L,O="Veytrix, Inc.",CN=x`: "C CN L O ST",
		`C=MX,ST=S,L=L,O="a\"b, c",CN=x`:       "C CN L O ST",
		`C=MX,ST=S,L=L,O=Veytrix, Inc.,CN=x`:   "C CN Inc. L O ST", // an unescaped comma separates
		`C=MX,ST=S,L=L,O=x\\,OU=y,CN=x`:        "C CN L O OU ST",   // an escaped backslash ends before the comma
	} {
		if got := attrNames(in); got != want {
			t.Errorf("attrNames(%s) = %q, want %q", in, got, want)
		}
	}
}
