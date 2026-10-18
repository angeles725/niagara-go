package navbuild

import (
	"reflect"
	"strings"
	"testing"
)

func TestCountBraces(t *testing.T) {
	for _, tc := range []struct {
		line          string
		opens, closes int
	}{
		{"class A { void f() { } }", 2, 2},
		{`String s = "{ \" }"; char c = '{'; char d = '\'';`, 0, 0},
		{"} // { comment", 0, 1},
		{`url = "http://x" + y; {`, 1, 0},
		{`"unterminated { `, 0, 0},
	} {
		if o, c := countBraces(tc.line); o != tc.opens || c != tc.closes {
			t.Errorf("countBraces(%q) = %d, %d; want %d, %d", tc.line, o, c, tc.opens, tc.closes)
		}
	}
}

func TestSplitParams(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"  ", nil},
		{"int a, Map<String, List<Integer>> m", []string{"int a", "Map<String, List<Integer>> m"}},
		{"a,,b, ", []string{"a", "b"}},
		{"Foo>> x, y", []string{"Foo>> x", "y"}}, // depth never goes below 0
	} {
		if got := splitParams(tc.in); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitParams(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractParamType(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"final Map<String, Integer>  m", "Map<String, Integer>"},
		{"@Deprecated int x", "int"},
		{`@SuppressWarnings("a b") List<T>... rest`, "List<T>..."},
		{"@A(x = 1) @B String s", "String"},
		{"@Lone", "@Lone"},
		{"int", "int"},
		// Python raises IndexError on ''.rsplit(None, 1)[0]; the type is empty here.
		{"@A(x y)", ""},
		{"@A)(b) c d", "c"}, // no balanced ')' after the '(': the annotation ends at the space
	} {
		if got := extractParamType(tc.in); got != tc.want {
			t.Errorf("extractParamType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseMethods(t *testing.T) {
	src := strings.Join([]string{
		"public class A {",
		"    public A() {",
		"    }",
		"    static <T> List<T> pick(int a,",
		"            String b) throws X {",
		"        return a.call(b);",
		"    }",
		"    ONE(1),",
		"    int x = f(1);",
		"    abstract void g();",
		"    @interface N { String v() default \"\"; }",
		"void columnZero() {}",
		"}",
	}, "\n")
	var got []string
	for _, m := range parseMethods(src, "File") {
		got = append(got, m.Class+" "+m.IndexedName+" "+m.Signature)
	}
	want := []string{
		"A <init> public A()",
		"A pick static <T> List<T> pick(int, String)",
		// pick's continuation line is skipped, so its "{" is never counted and
		// the "}" closing pick pops A: g falls back to the file's class.
		"File g abstract void g()",
		// a line starting with "@" is never a method, even "@interface N { ... v() ... }"
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseMethods =\n%q\nwant\n%q", got, want)
	}
}

// TestMaxClassTieBreak pins _meta.max_methods_class: the first class, in
// first-seen order, whose count is strictly greater than the running maximum
// after each file (Python rescans every class after each file).
func TestMaxClassTieBreak(t *testing.T) {
	rec := func(class string, names ...string) []methodRecord {
		var out []methodRecord
		for _, n := range names {
			out = append(out, methodRecord{n, class, "{}"})
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		files [][]methodRecord
		max   int
		class string
	}{
		{"equal counts in one file: first seen wins", [][]methodRecord{append(rec("A", "a", "b"), rec("B", "a", "b")...)}, 2, "A"},
		{"within a file first-seen order wins, not the record order", [][]methodRecord{rec("B", "a"), rec("A", "a"), append(rec("A", "b", "c"), rec("B", "b", "c")...)}, 3, "B"},
		{"reaching the maximum later does not take it", [][]methodRecord{append(rec("B", "a"), rec("A", "a")...), rec("A", "b", "c"), rec("B", "b", "c")}, 3, "A"},
		{"a repeated name does not count", [][]methodRecord{rec("A", "a", "a", "a"), rec("B", "a", "b")}, 2, "B"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mi := newMethodIndex()
			for _, f := range tc.files {
				mi.add(f)
			}
			if mi.MaxPerClass != tc.max || mi.MaxClass != tc.class {
				t.Errorf("max = %d (%s), want %d (%s)", mi.MaxPerClass, mi.MaxClass, tc.max, tc.class)
			}
		})
	}
}
