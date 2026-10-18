package navbuild

import (
	"reflect"
	"testing"

	"github.com/angeles725/niagara-go/internal/nav/core"
)

func TestParseExtendsImplements(t *testing.T) {
	for _, tc := range []struct {
		in      string
		extends optString
		impl    []string
	}{
		{"<T extends X<T>> extends B<T> implements I, J<K, L>", optString{"B<T>", true}, []string{"I", "J<K, L>"}},
		{"implements A extends B", optString{"", true}, []string{"A extends B"}},
		{"extends Foo,", optString{"Foo", true}, []string{}},
		{"extends ,", optString{}, []string{}},
		{"extendsX implements Y", optString{}, []string{"Y"}},
		{"", optString{}, []string{}},
		{"permits P", optString{}, []string{}},
	} {
		ext, impl := parseExtendsImplements(tc.in)
		if ext != tc.extends || !reflect.DeepEqual(impl, tc.impl) {
			t.Errorf("parseExtendsImplements(%q) = %+v %q, want %+v %q", tc.in, ext, impl, tc.extends, tc.impl)
		}
	}
}

// TestDeclMatchEnd: the header starts at core.Decl.End (m.end()), also when
// the declaration text appears inside an annotation argument first.
func TestDeclMatchEnd(t *testing.T) {
	for line, rest := range map[string]string{
		"public class A extends B {\n":                               " extends B {\n",
		"@Doc(\"public class A\") public class A extends B {\n":      " extends B {\n",
		"\t@X @Y(z) protected static final class A$1 implements C\n": " implements C\n",
		"public record P<T>(T t) {}\n":                               "<T>(T t) {}\n",
		"public\u00a0class NB extends\u00a0S {\n":                    " extends\u00a0S {\n",
		"class A {}\n": " {}\n",
	} {
		d, ok := core.MatchDecl(line)
		if !ok {
			t.Fatalf("no declaration in %q", line)
		}
		if line[d.End:] != rest {
			t.Errorf("MatchDecl(%q).End = %d, want the rest %q", line, d.End, rest)
		}
	}
}
