package pxrender

import "testing"

func TestParseFont(t *testing.T) {
	tests := []struct{ in, weight, style, size, family string }{
		{"bold 22.0pt Times New Roman", "bold", "normal", "22", "Times New Roman"},
		{"12.0pt Arial", "normal", "normal", "12", "Arial"},
		{"italic 9.5pt", "normal", "italic", "9", "Arial"},
		{"bold italic 12.0pt Arial Narrow", "bold", "italic", "12", "Arial Narrow"},
		{"Verdana", "normal", "normal", "13", "Verdana"},
		{"bold 12pt 14.25pt Courier New", "bold", "normal", "14", "Courier New"},
		{"", "normal", "normal", "13", "Arial"},
	}
	for _, tt := range tests {
		f := parseNiagaraFont(tt.in)
		if f.weight != tt.weight || f.style != tt.style || f.size != tt.size || f.family != tt.family {
			t.Errorf("parseNiagaraFont(%q) = %+v, want %s %s %s %s", tt.in, f, tt.weight, tt.style, tt.size, tt.family)
		}
	}
}

func TestCSSFamily(t *testing.T) {
	for in, want := range map[string]string{"Arial": "Arial,Arial,sans-serif",
		"Times New Roman": "'Times New Roman',Arial,sans-serif", "x';y": "Arial,sans-serif"} {
		if got := cssFamily(in); got != want {
			t.Errorf("cssFamily(%q) = %q, want %q", in, got, want)
		}
	}
}
