package indexes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// genericPage is a javadoc 8 class page whose title and declaration carry
// type arguments, as javax.baja.fox.FoxConnectionTypeEnum and
// javax.baja.sys.IterableCursor do in 4.15.
func genericPage(kind, title, decl string) string {
	return `<html><head><title>X (x)</title></head><body>` +
		`<div class="subTitle">javax.baja.demo</div>` +
		`<h2 title="` + kind + ` x" class="title">` + kind + ` ` + title + `</h2>` +
		`<pre>` + decl + `</pre><div class="block">Desc.</div></body></html>`
}

// The class name and its EXTENDS keep their type arguments unescaped and
// whole: the regex stopped at the first "<" and left "&lt;" in the page.
// A bound inside the type parameters (<E extends Exception>) is not the
// class's superclass.
func TestParseBajadocGenerics(t *testing.T) {
	for _, c := range []struct {
		name, page, wantName, wantExtends string
	}{
		{"enum", genericPage("Enum", "FoxConnectionTypeEnum",
			`public enum <span class="typeNameLabel">FoxConnectionTypeEnum</span>
extends java.lang.Enum&lt;<a href="F.html" title="enum in x">FoxConnectionTypeEnum</a>&gt;`),
			"FoxConnectionTypeEnum", "java.lang.Enum<FoxConnectionTypeEnum>"},
		{"interface", genericPage("Interface", "IterableCursor&lt;E&gt;",
			`public interface <span class="typeNameLabel">IterableCursor&lt;E&gt;</span>
extends <a href="C.html" title="interface in x">Cursor</a>&lt;E&gt;, java.lang.Iterable&lt;E&gt;`),
			"IterableCursor<E>", "Cursor<E>"},
		{"nested", genericPage("Class", "Pair",
			`public class <span class="typeNameLabel">Pair</span>
extends <a href="M.html">BMap</a>&lt;java.lang.String,java.util.List&lt;<a href="T.html">BTag</a>&gt;&gt;
implements <a href="I.html">BIThing</a>`),
			"Pair", "BMap<java.lang.String,java.util.List<BTag>>"},
		{"bound only", genericPage("Class", "Runner",
			`public interface <span class="typeNameLabel">Runner&lt;E extends java.lang.Exception&gt;</span>`),
			"Runner", ""},
		{"plain", genericPage("Class", "BThing",
			`public class <span class="typeNameLabel">BThing</span>
extends <a href="B.html">BComponent</a>`),
			"BThing", "BComponent"},
	} {
		got := parseBajadocPage(c.page)
		if got == nil {
			t.Fatalf("%s: no class parsed", c.name)
		}
		if got.Name != c.wantName || got.Extends != c.wantExtends {
			t.Errorf("%s: name %q extends %q, want %q, %q", c.name, got.Name, got.Extends, c.wantName, c.wantExtends)
		}
	}
}

// inheritance.json keys a generic class by its erasure and stores the
// erasure of its superclass, so the chain commands find "IterableCursor"
// and walk up to "Cursor" the way they find a non-generic class.
func TestBuildInheritanceErasure(t *testing.T) {
	base := t.TempDir()
	write := func(rel, text string) {
		p := filepath.Join(base, "bajadoc-clean", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	write("x/IterableCursor.txt", "INTERFACE: IterableCursor<E>\nPACKAGE: x\nEXTENDS: Cursor<E>\n\n")
	write("x/Legacy.txt", "CLASS: Legacy&lt;K,V&gt;\nPACKAGE: x\nEXTENDS: java.lang.Enum&lt;\n\n")
	write("x/Pair.txt", "CLASS: Pair\nPACKAGE: x\nEXTENDS: BMap<java.lang.String,java.util.List<BTag>>\n\n")
	write("x/BThing.txt", "CLASS: BThing\nPACKAGE: x\nEXTENDS: BComponent\n\n")
	if _, err := BuildInheritance(base); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(base, "indexes", "inheritance.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		`"IterableCursor": {` + "\n" + `    "extends": "Cursor",`,
		`"Legacy": {` + "\n" + `    "extends": "java.lang.Enum",`,
		`"Pair": {` + "\n" + `    "extends": "BMap",`,
		`"BThing": {` + "\n" + `    "extends": "BComponent",`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("inheritance.json lacks %q:\n%s", want, got)
		}
	}
	if strings.ContainsAny(got, "<>&") {
		t.Errorf("inheritance.json keeps type arguments or entities:\n%s", got)
	}
}
