package argparse

import (
	"strings"
	"unicode"
)

// A port of the parts of Python's textwrap that argparse uses: wrap() and fill() with the
// default TextWrapper options (break_long_words, break_on_hyphens, drop_whitespace).
// Lengths are counted in code points, like Python len().

func isTWSpace(r rune) bool { // textwrap._whitespace
	return r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r' || r == ' '
}

func isWordChar(r rune) bool { // re \w (Unicode)
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

func isLetter(r rune) bool { // [^\d\W]
	return isWordChar(r) && !unicode.IsDigit(r)
}

func isWordPunct(r rune) bool { // [\w!"\'&.,?]
	return isWordChar(r) || strings.ContainsRune(`!"'&.,?`, r)
}

// splitChunks is TextWrapper._split_chunks: whitespace becomes spaces, then the text is cut
// by wordsep_re into whitespace runs, words, hyphenated word parts and em-dashes.
func splitChunks(text string) [][]rune {
	t := []rune(text)
	for i, r := range t {
		if isTWSpace(r) {
			t[i] = ' '
		}
	}
	at := func(i int) rune {
		if i < 0 || i >= len(t) {
			return 0
		}
		return t[i]
	}
	dashRunWord := func(j int) bool { // (?=-{2,}\w) at j
		k := j
		for k < len(t) && t[k] == '-' {
			k++
		}
		return k-j >= 2 && k < len(t) && isWordChar(t[k])
	}
	var chunks [][]rune
	for i := 0; i < len(t); {
		start := i
		switch {
		case t[i] == ' ':
			for i < len(t) && t[i] == ' ' {
				i++
			}
		case i > 0 && isWordPunct(t[i-1]) && dashRunWord(i): // em-dash between words
			for t[i] == '-' {
				i++
			}
		default:
			j := i + 1 // nws+? : consume one, then try the endings, growing lazily
			for {
				// hyphen inside a compound word: -(?:(?<=lt{2}-)|(?<=lt-lt-))(?=lt-?lt)
				if at(j) == '-' && ((isLetter(at(j-2)) && isLetter(at(j-1))) ||
					(isLetter(at(j-3)) && at(j-2) == '-' && isLetter(at(j-1)))) &&
					isLetter(at(j+1)) && (isLetter(at(j+2)) || (at(j+2) == '-' && isLetter(at(j+3)))) {
					j++
					break
				}
				if j >= len(t) || t[j] == ' ' { // end of word
					break
				}
				if isWordPunct(t[j-1]) && dashRunWord(j) { // before an em-dash
					break
				}
				j++
			}
			i = j
		}
		chunks = append(chunks, t[start:i])
	}
	return chunks
}

func isBlank(chunk []rune) bool {
	for _, r := range chunk {
		if !isPySpace(r) {
			return false
		}
	}
	return true
}

// wrapText is textwrap.wrap(text, width, initial_indent=indent, subsequent_indent=indent).
func wrapText(text string, width int, indent string) []string {
	chunks := splitChunks(text)
	// reversed stack: the next chunk is the last element
	for l, r := 0, len(chunks)-1; l < r; l, r = l+1, r-1 {
		chunks[l], chunks[r] = chunks[r], chunks[l]
	}
	var lines []string
	indentLen := runeLen(indent)
	for len(chunks) > 0 {
		var cur [][]rune
		curLen := 0
		w := width - indentLen
		if isBlank(chunks[len(chunks)-1]) && len(lines) > 0 {
			chunks = chunks[:len(chunks)-1]
		}
		for len(chunks) > 0 {
			l := len(chunks[len(chunks)-1])
			if curLen+l > w {
				break
			}
			cur = append(cur, chunks[len(chunks)-1])
			chunks = chunks[:len(chunks)-1]
			curLen += l
		}
		if len(chunks) > 0 && len(chunks[len(chunks)-1]) > w {
			cur, chunks = handleLongWord(chunks, cur, curLen, w)
			curLen = 0
			for _, c := range cur {
				curLen += len(c)
			}
		}
		if len(cur) > 0 && isBlank(cur[len(cur)-1]) {
			curLen -= len(cur[len(cur)-1])
			cur = cur[:len(cur)-1]
		}
		if len(cur) > 0 {
			var b strings.Builder
			b.WriteString(indent)
			for _, c := range cur {
				b.WriteString(string(c))
			}
			lines = append(lines, b.String())
		}
	}
	return lines
}

// handleLongWord is TextWrapper._handle_long_word.
func handleLongWord(chunks, cur [][]rune, curLen, width int) ([][]rune, [][]rune) {
	spaceLeft := width - curLen
	if width < 1 {
		spaceLeft = 1
	}
	last := chunks[len(chunks)-1]
	if spaceLeft > 0 {
		end := spaceLeft
		if len(last) > spaceLeft {
			hyphen := -1
			for k := spaceLeft - 1; k >= 0; k-- {
				if last[k] == '-' {
					hyphen = k
					break
				}
			}
			if hyphen > 0 {
				for _, r := range last[:hyphen] {
					if r != '-' {
						end = hyphen + 1
						break
					}
				}
			}
		}
		cur = append(cur, last[:end])
		chunks[len(chunks)-1] = last[end:]
	} else if len(cur) == 0 {
		cur = append(cur, last)
		chunks = chunks[:len(chunks)-1]
	}
	return cur, chunks
}

// fillText is textwrap.fill: the wrapped lines joined by newlines.
func fillText(text string, width int, indent string) string {
	return strings.Join(wrapText(text, width, indent), "\n")
}
