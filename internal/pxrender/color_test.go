package pxrender

import "testing"

func TestCSSColor(t *testing.T) {
	for in, want := range map[string]string{
		"#f8f2f2": "#f8f2f2", "#1A70D0": "#1A70D0", "#ccefefef": "rgba(239,239,239,0.8)",
		"silver": "silver", "black": "black", "": "", "red;color:x": "", "url(x)": "", "#12345": "",
	} {
		if got := cssColor(in); got != want {
			t.Errorf("cssColor(%q) = %q, want %q", in, got, want)
		}
	}
}
