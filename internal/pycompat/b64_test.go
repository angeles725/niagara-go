package pycompat

import "testing"

func TestB64Decode(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      string
		urlsafe bool
		want    string
	}{
		{"empty", "", false, ""},
		{"full quad", "TWFu", false, "Man"},
		{"padded pair", "TWE=", false, "Ma"},
		{"padded single", "TQ==", false, "M"},
		{"missing padding pair", "TWE", true, "Ma"},
		{"missing padding single", "TQ", true, "M"},
		// CPython 3.15 b64decode raises "Incorrect padding" here; see the B64Decode doc.
		{"standard unpadded tail kept lenient", "TWFuTWE", false, "ManMa"},
		{"extra padding skipped", "TQ====", false, "M"},
		{"padding mid-string skipped", "TW=Fu", false, "Man"},
		{"junk discarded", " T\nW*F u\t", false, "Man"},
		{"non-ASCII bytes discarded", "TW\xc3\xa9Fu", false, "Man"},
		{"standard plus slash", "+/+/", false, "\xfb\xff\xbf"},
		{"standard skips url alphabet", "-_+/+/", false, "\xfb\xff\xbf"},
		{"urlsafe dash underscore", "-_-_", true, "\xfb\xff\xbf"},
		{"urlsafe still takes plus slash", "+/-_", true, "\xfb\xff\xbf"},
		{"standard skips trailing dash", "TWFu-", false, "Man"},
		{"urlsafe missing padding", "-_8", true, "\xfb\xff"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := B64Decode(tt.in, tt.urlsafe)
			if err != nil {
				t.Fatalf("B64Decode(%q, %v) error %v", tt.in, tt.urlsafe, err)
			}
			if string(got) != tt.want {
				t.Errorf("B64Decode(%q, %v) = %q, want %q", tt.in, tt.urlsafe, got, tt.want)
			}
		})
	}
}

func TestB64DecodeLoneTrailingCharacter(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      string
		urlsafe bool
		want    string
	}{
		{"one char", "T", false, "Invalid base64-encoded string: number of data characters (1) cannot be 1 more than a multiple of 4"},
		{"five chars", "TWFuT", false, "Invalid base64-encoded string: number of data characters (5) cannot be 1 more than a multiple of 4"},
		{"padding does not help", "TWFuT===", true, "Invalid base64-encoded string: number of data characters (5) cannot be 1 more than a multiple of 4"},
		{"junk not counted", "T*W*F*u*T*", false, "Invalid base64-encoded string: number of data characters (5) cannot be 1 more than a multiple of 4"},
		{"urlsafe counts dash", "TWFu-", true, "Invalid base64-encoded string: number of data characters (5) cannot be 1 more than a multiple of 4"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := B64Decode(tt.in, tt.urlsafe)
			if err == nil {
				t.Fatalf("B64Decode(%q, %v) = %q, want error", tt.in, tt.urlsafe, got)
			}
			if err.Error() != tt.want {
				t.Errorf("B64Decode(%q, %v) error %q, want %q", tt.in, tt.urlsafe, err, tt.want)
			}
		})
	}
}
