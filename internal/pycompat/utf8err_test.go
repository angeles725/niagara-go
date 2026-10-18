package pycompat

import "testing"

// TestFindUTF8Error pins CPython's bytes.decode("utf-8") UnicodeDecodeError text: the
// offending byte or range, its position, and the reason.
func TestFindUTF8Error(t *testing.T) {
	for _, tt := range []struct {
		name, in, want string
		start, end     int
	}{
		{name: "valid", in: "héllo €"},
		{name: "invalid start byte", in: "ab\x80", start: 2, end: 3,
			want: "'utf-8' codec can't decode byte 0x80 in position 2: invalid start byte"},
		{name: "never-valid lead byte", in: "\xff", start: 0, end: 1,
			want: "'utf-8' codec can't decode byte 0xff in position 0: invalid start byte"},
		{name: "cut lead byte", in: "a\xc3", start: 1, end: 2,
			want: "'utf-8' codec can't decode byte 0xc3 in position 1: unexpected end of data"},
		{name: "cut sequence", in: "\xe2\x82", start: 0, end: 2,
			want: "'utf-8' codec can't decode bytes in position 0-1: unexpected end of data"},
		{name: "bad first continuation", in: "\xe2\x28\xa1", start: 0, end: 1,
			want: "'utf-8' codec can't decode byte 0xe2 in position 0: invalid continuation byte"},
		{name: "bad later continuation", in: "\xf0\x9f\x98\x28", start: 0, end: 3,
			want: "'utf-8' codec can't decode bytes in position 0-2: invalid continuation byte"},
		{name: "overlong", in: "\xe0\x80\x80", start: 0, end: 1,
			want: "'utf-8' codec can't decode byte 0xe0 in position 0: invalid continuation byte"},
		{name: "surrogate", in: "\xed\xa0\x80", start: 0, end: 1,
			want: "'utf-8' codec can't decode byte 0xed in position 0: invalid continuation byte"},
		{name: "beyond U+10FFFF", in: "\xf4\x90\x80\x80", start: 0, end: 1,
			want: "'utf-8' codec can't decode byte 0xf4 in position 0: invalid continuation byte"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := FindUTF8Error([]byte(tt.in))
			if tt.want == "" {
				if e != nil {
					t.Fatalf("valid input: %v", e)
				}
				return
			}
			if e == nil || e.Error() != tt.want || e.Start != tt.start || e.End != tt.end {
				t.Fatalf("got %+v (%v), want [%d,%d) %q", e, e, tt.start, tt.end, tt.want)
			}
		})
	}
}
