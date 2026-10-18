package pyjson

func hexVal(r rune) (rune, bool) {
	switch {
	case r >= '0' && r <= '9':
		return r - '0', true
	case r >= 'a' && r <= 'f':
		return r - 'a' + 10, true
	case r >= 'A' && r <= 'F':
		return r - 'A' + 10, true
	}
	return 0, false
}

// scanString mirrors scanstring_unicode (strict=True); end is just past the quote.
func (p *jsonScanner) scanString(end int) (string, int) {
	s, begin := p.s, end-1
	var out []rune
	for {
		next := end
		for ; next < len(s); next++ {
			c := s[next]
			if c == '"' || c == '\\' {
				break
			}
			if c <= 0x1F {
				p.fail("Invalid control character at", next)
			}
		}
		if next >= len(s) {
			p.fail("Unterminated string starting at", begin)
		}
		out = append(out, s[end:next]...)
		next++
		if s[next-1] == '"' {
			return WTF8(out), next
		}
		if next == len(s) {
			p.fail("Unterminated string starting at", begin)
		}
		c := s[next]
		if c != 'u' {
			end = next + 1
			esc := map[rune]rune{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}
			r, ok := esc[c]
			if !ok {
				p.fail("Invalid \\escape", end-2)
			}
			out = append(out, r)
			continue
		}
		next++
		end = next + 4
		if end > len(s) {
			p.fail("Invalid \\uXXXX escape", next-1)
		}
		u := p.hex4(next, end)
		next = end
		if u >= 0xD800 && u <= 0xDBFF && end+6 <= len(s) && s[next] == '\\' && s[next+1] == 'u' {
			end += 6
			u2 := p.hex4(next+2, end)
			if u2 >= 0xDC00 && u2 <= 0xDFFF {
				u = 0x10000 + (u-0xD800)<<10 + (u2 - 0xDC00)
			} else {
				end -= 6
			}
		}
		out = append(out, u)
	}
}

func (p *jsonScanner) hex4(from, end int) rune {
	var u rune
	for k := from; k < end; k++ {
		d, ok := hexVal(p.s[k])
		if !ok {
			p.fail("Invalid \\uXXXX escape", end-5)
		}
		u = u<<4 | d
	}
	return u
}
