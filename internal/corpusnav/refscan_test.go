package corpusnav

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/blockfiles"
	"github.com/angeles725/niagara-go/internal/pyre"
)

// The hand-written scanners of refscan.go are held to the regexes they replace:
// _REF_RE (refRe), _BARE_REF_RE and _CODE_RE (refBareRe and refCodeRe below), over every
// real block of the research corpus (when it is present) and over adversarial strings.

var (
	refBareRe = lazyRe(`(?<![\w/.\[-])B(\d{1,4})(?![\w-])`, 0)
	refCodeRe = lazyRe("```.*?```|`[^`\\n]*`", pyre.DOTALL)
	pyDigitRe = lazyRe(`\d`, 0)
)

// byteOffsets maps a rune index of []rune(text) to its byte offset in text (one rune per
// invalid byte, as the conversion and utf8.DecodeRuneInString both decode it).
func byteOffsets(text string) []int {
	offs := make([]int, 0, len(text)+1)
	for i := range text {
		offs = append(offs, i)
	}
	return append(offs, len(text))
}

// regexRefs is refRe.finditer(text) as refMatch values with byte spans.
func regexRefs(text string) []refMatch {
	offs := byteOffsets(text)
	var out []refMatch
	for _, m := range refRe().FindIter([]rune(text)) {
		rm := refMatch{start: offs[m.Start()], end: offs[m.End()]}
		for g := 1; g <= 3; g++ {
			if s, ok := m.GroupOK(g); ok {
				rm.group, rm.digits = g, s
			}
		}
		out = append(out, rm)
	}
	return out
}

func scannedRefs(text string) []refMatch {
	var out []refMatch
	for m := range scanRefs(text) {
		out = append(out, m)
	}
	return out
}

// regexSubSpans is rx.sub(repl, text) for a fixed replacement.
func regexSubSpans(text string, rx *pyre.Regexp, repl string) string {
	rs := []rune(text)
	var b strings.Builder
	last := 0
	for _, m := range rx.FindIter(rs) {
		b.WriteString(string(rs[last:m.Start()]))
		b.WriteString(repl)
		last = m.End()
	}
	b.WriteString(string(rs[last:]))
	return b.String()
}

func regexBare(text string) []string {
	return refBareRe().FindAll(regexSubSpans(text, refCodeRe(), " "))
}

func scannedBare(text string) []string {
	var out []string
	for n := range bareRefs(stripCode(text)) {
		out = append(out, n)
	}
	return out
}

// checkScanners compares every scanner with its regex on text.
func checkScanners(t *testing.T, name, text string) {
	t.Helper()
	if want, got := regexRefs(text), scannedRefs(text); !slices.Equal(want, got) {
		t.Errorf("%s: scanRefs = %v, refRe = %v (text %q)", name, got, want, clip(text))
	}
	if want, got := regexBare(text), scannedBare(text); !slices.Equal(want, got) {
		t.Errorf("%s: bareRefs = %v, bareRefRe = %v (text %q)", name, got, want, clip(text))
	}
	if utf8.ValidString(text) { // sub() turns invalid bytes into U+FFFD; stripCode keeps them
		if want, got := regexSubSpans(text, refCodeRe(), " "), stripCode(text); want != got {
			t.Errorf("%s: stripCode = %q, codeRe.sub = %q", name, clip(got), clip(want))
		}
	}
}

func clip(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// adversarialPieces are the fragments the random strings are built from: every
// character class the patterns look at, in ASCII and beyond.
var adversarialPieces = []string{
	"[", "]", "B", "b", "Block", "BLOCK", "Bloque", "bloque", "BlOqUe", "bloqu", "lock", "loque", "LoQuE",
	"#", " ", "\t", "\n", "\r", " ", " ", "　", "\x1c", "§", "1", "0", "42", "007", "12345",
	"٣", "３", "𝟘", "²", "x", "_", "é", "ß", "-", "/", ".", "`", "```", "\xff", "\xe2\x82", "mental-model-",
	"[B", "[b", "[Block ", "[Bloque #", "] ", " §", "bloque1", "B12", "B1234", "B99999",
}

var adversarialCases = []string{
	"", "[", "B", "[B]", "[B1]", "[b1]", "[B01]", "[B1 §2]", "[B1  \t§", "[B1 x]", "[B1§]", "[B１]", "[B٣]",
	"[Block 18]", "[block 18]", "[BLOCK\t18]", "[Block18]", "[Block #18]", "[Bloque #12]", "[Bloque ##1]",
	"[Bloque 1 ]", "[Bloque 7]", "[Bloque 7]", "[Blo 1]", "bloque106", "Bloque106", "xbloque1",
	"_bloque1", "-bloque1", "ébloque1", "1bloque1", "bloque1x", "bloque1_", "bloque1-", "bloque1é",
	"bloque١٢", "[bloque1]", "mental-model-bloque5", "[[B1]]", "[B1][B2]", "B[B1]", "bbloque1",
	"B1", "B12", "B1234", "B12345", "xB1", "/B1", ".B1", "[B1", "-B1", " B1-", "B1x", "B1_", "(B12)", "B٣",
	"`B1`", "``B1", "```B1```B2", "```B1", "`a\nB1`", "``` x ``` B3 ``` y", "a`b`c`d`B4", "\xffB1\xff",
	"\xffbloque2", "\xe2bloque2", "é[B3]", "[B3]é",
}

func TestScannersMatchAdversarial(t *testing.T) {
	for i, s := range adversarialCases {
		checkScanners(t, "case "+strconv.Itoa(i), s)
	}
	rng := rand.New(rand.NewPCG(36, 2026))
	for i := range 20000 {
		var b strings.Builder
		for range 1 + rng.IntN(12) {
			b.WriteString(adversarialPieces[rng.IntN(len(adversarialPieces))])
		}
		checkScanners(t, "random", b.String())
		if t.Failed() && i > 0 {
			return // the first failures are enough
		}
	}
}

// TestScannersMatchRealCorpus runs the differential check over every main and sub block
// of the research corpus (read-only). Skipped where the corpus is not checked out.
func TestScannersMatchRealCorpus(t *testing.T) {
	root := os.Getenv("CORPUS_NAV_ROOT")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, "niagara-research")
	}
	main, err := blockfiles.MainBlocks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(main) == 0 {
		t.Skipf("no research corpus at %s", root)
	}
	files := 0
	sub, err := blockfiles.SubBlocks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range append(main, sub...) {
		text, ok := readText(b.Path)
		if !ok {
			continue
		}
		files++
		checkScanners(t, b.Path, text)
	}
	t.Logf("%d blocks compared", files)
}

// TestPyDigitIsReDigit: the digit test of the scanners is re's \d over every code point.
func TestPyDigitIsReDigit(t *testing.T) {
	var all []rune
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if utf8.ValidRune(r) {
			all = append(all, r)
		}
	}
	re := map[rune]bool{}
	for _, m := range pyDigitRe().FindIter(all) {
		re[all[m.Start()]] = true
	}
	for _, r := range all {
		if isPyDigit(r) != re[r] {
			t.Errorf("isPyDigit(%U) = %v, \\d says %v", r, isPyDigit(r), re[r])
		}
	}
}

func FuzzScannersMatchRegex(f *testing.F) {
	for _, s := range adversarialCases {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) { checkScanners(t, "fuzz", s) })
}
