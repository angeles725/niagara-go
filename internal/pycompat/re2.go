package pycompat

import "strings"

// RE2Space rewrites a fixed Go regexp pattern so that its \s means Python's (Go's \s is ASCII
// only): `\s` becomes `[` + SpaceClass() + `]` outside a character class and SpaceClass()
// inside one, so it matches exactly the IsSpace code points. Every other
// escape (`\\s` is an escaped backslash and an "s") and POSIX `[:name:]` classes are copied
// unchanged. `\S` panics: none of the ported patterns uses it, and inside a class it has no
// RE2 spelling as a union; a pattern needing it must be written out by hand. \Q...\E is not
// recognised.
func RE2Space(pattern string) string {
	cls := SpaceClass()
	var b strings.Builder
	b.Grow(len(pattern))
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\\' && i+1 < len(pattern):
			i++
			switch pattern[i] {
			case 's':
				if inClass {
					b.WriteString(cls)
				} else {
					b.WriteString("[" + cls + "]")
				}
				continue
			case 'S':
				panic("pycompat.RE2Space: \\S is not supported: " + pattern)
			}
			b.WriteByte(c)
			b.WriteByte(pattern[i])
			continue
		case !inClass && c == '[':
			inClass = true
			b.WriteByte(c)
			// A ']' right after '[' or '[^' is a literal member, not the end of the class.
			if i+1 < len(pattern) && pattern[i+1] == '^' {
				i++
				b.WriteByte('^')
			}
			if i+1 < len(pattern) && pattern[i+1] == ']' {
				i++
				b.WriteByte(']')
			}
			continue
		case inClass && strings.HasPrefix(pattern[i:], "[:"):
			if end := strings.Index(pattern[i+2:], ":]"); end >= 0 {
				b.WriteString(pattern[i : i+2+end+2])
				i += 2 + end + 1
				continue
			}
		case inClass && c == ']':
			inClass = false
		}
		b.WriteByte(c)
	}
	return b.String()
}
