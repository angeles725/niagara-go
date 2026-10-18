package key

import "testing"

func TestGroupKeysLeadingZeros(t *testing.T) {
	c, problems := Load("../testdata/root")
	if c == nil {
		t.Fatalf("fixture root: %v", problems)
	}
	for _, in := range []string{"1", "01", "L01", "l001", " 1 "} {
		if got := c.GroupKeys(in); len(got) != 1 || got[0] != "L1" {
			t.Errorf("groupKeys(%q) = %v, want [L1]", in, got)
		}
	}
	if got := c.GroupKeys("0"); got != nil {
		t.Errorf("groupKeys(\"0\") = %v, want none", got)
	}
}
