package audits

import (
	"reflect"
	"testing"
)

func TestLexiconKeys(t *testing.T) {
	text := "# c\n a = 1\n\nb=2\x0bb=3\r\nnoequals\n=empty\n  a=4 \u2028c = x=y\n"
	want := []string{"a", "b", "b", "a", "c"}
	if got := lexiconKeys(text); !reflect.DeepEqual(got, want) {
		t.Errorf("lexiconKeys = %q, want %q", got, want)
	}
}
