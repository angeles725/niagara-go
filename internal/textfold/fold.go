// Package textfold reproduces the Python text normalization of the
// niagara-research tools byte for byte: the four accent folds, str.lower(),
// str.split(), the substring query layer and the textsearch ranking terms.
//
// Go's standard library has no NFKD and ships a different Unicode version than
// the Python the tools run on, so the per-character facts come from unicode.txt,
// generated from that Python by the retired testdata/gen.py. The fold variants and the
// callers that use each are listed in docs/text-variants.md.
package textfold

import "strings"

// Options selects one fold. Every fold decomposes with NFKD and drops the
// combining marks (canonical combining class != 0).
type Options struct {
	// Lower applies Python's str.lower() before decomposing.
	Lower bool
	// ASCII drops every non-ASCII character after decomposing
	// (Python's .encode("ascii", "ignore")).
	ASCII bool
	// DegreeWord replaces U+00B0 by " deg" before decomposing, so "72 °F"
	// becomes "72  degF" (U+2103 and U+2109 still decompose to "°C"/"°F").
	DegreeWord bool
}

// The folds of the Python tools (docs/text-variants.md).
var (
	// Corpus is tools/lib/textsearch.py fold: lower-case, ASCII only.
	Corpus = Options{Lower: true, ASCII: true}
	// ClassRank is module-navigator class_rank.fold: ASCII only, case kept.
	ClassRank = Options{ASCII: true}
	// HelpFulltext is niagara-help fulltext_terms.fold: case and non-ASCII
	// letters kept, degree sign spelled out.
	HelpFulltext = Options{DegreeWord: true}
	// StripAccents is module-navigator corpus.strip_accents: case and
	// non-ASCII letters kept.
	StripAccents = Options{}
)

// Fold returns s folded as o describes. s must be valid UTF-8 (Python text
// never holds an invalid byte; here each one would read as U+FFFD).
func Fold(s string, o Options) string {
	if isASCII(s) {
		if o.Lower {
			return asciiLower(s)
		}
		return s
	}
	if o.DegreeWord {
		s = strings.ReplaceAll(s, "°", " deg")
	}
	if o.Lower {
		s = Lower(s)
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		d, ok := decompose(r)
		if !ok {
			if !o.ASCII || r < 0x80 {
				b.WriteRune(r)
			}
			continue
		}
		if !o.ASCII {
			b.WriteString(d)
			continue
		}
		for i := 0; i < len(d); i++ {
			if d[i] < 0x80 {
				b.WriteByte(d[i])
			}
		}
	}
	return b.String()
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func asciiLower(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

const capitalSigma, smallSigma, finalSigma = 'Σ', 'σ', 'ς'

// Lower is Python's str.lower(): the full lower-case mapping of each rune
// ("İ" -> "i̇"), and capital sigma becomes final sigma at the end of a word
// (Unicode Final_Sigma, as CPython's handle_capital_sigma decides it).
func Lower(s string) string {
	if isASCII(s) {
		return asciiLower(s)
	}
	t := data()
	runes := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		switch {
		case r < 0x80:
			if r >= 'A' && r <= 'Z' {
				r += 'a' - 'A'
			}
			b.WriteByte(byte(r))
		case r == capitalSigma:
			if isFinalSigma(t, runes, i) {
				b.WriteRune(finalSigma)
			} else {
				b.WriteRune(smallSigma)
			}
		default:
			if l, ok := t.lower[r]; ok {
				b.WriteString(l)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// isFinalSigma: \p{cased}\p{case-ignorable}* U+03A3 !(\p{case-ignorable}* \p{cased}).
func isFinalSigma(t *tables, runes []rune, i int) bool {
	j := i - 1
	for j >= 0 && inRanges(t.ignorable, runes[j]) {
		j--
	}
	if j < 0 || !inRanges(t.cased, runes[j]) {
		return false
	}
	j = i + 1
	for j < len(runes) && inRanges(t.ignorable, runes[j]) {
		j++
	}
	return j == len(runes) || !inRanges(t.cased, runes[j])
}

// Fields is Python's str.split() with no argument: runs of str.isspace()
// characters separate the fields; there are no empty fields.
func Fields(s string) []string {
	return strings.FieldsFunc(s, IsSpace)
}

// ParseQuery is textsearch.parse_query: the lower-cased whitespace-separated
// tokens of q ("" and blank queries give none).
func ParseQuery(q string) []string {
	return Fields(Lower(q))
}
