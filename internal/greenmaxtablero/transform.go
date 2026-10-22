package greenmaxtablero

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

const imgNormal = "file:^Imagenes/The Harbor/Buttons/BotonNormal.png"

var (
	pillRE = regexp.MustCompile(pycompat.RE2Space(`<Picture layout="([0-9.]+),([0-9.]+),60\.0,30\.0"[^>]*>\s*` +
		`<ValueBinding ord="slot:Relay\$5b([0-9]+)\$5d\$2eBO">\s*<IBooleanToSimple`))
	hoaRx    = `Relay\$5b[0-9]+\$5d\$2eBO/(?:active|inactive|auto)"`
	popRx    = `PopupBinding\b[^>]*\bord="slot:R[0-9]+"`
	hoaRE    = regexp.MustCompile(hoaRx)
	popRE    = regexp.MustCompile(popRx)
	hoaBtnRE = regexp.MustCompile(pycompat.RE2Space(`(?s)\s*<ImageButton\b[^>]*>\s*<ActionBinding\b[^>]*\bord="slot:` + hoaRx +
		`[^>]*>.*?</ActionBinding>\s*</ImageButton>`))
	popPicRE   = regexp.MustCompile(pycompat.RE2Space(`(?s)\s*<Picture\b[^>]*>\s*<` + popRx + `[^>]*/>\s*</Picture>`))
	viewSizeRE = regexp.MustCompile(`viewSize="([0-9.]+),([0-9.]+)"`)
)

type point struct{ x, y float64 }

// pyFloat is float() of a "[\d.]+" match: a digit string past float64 is inf, as in Python.
func pyFloat(s string) (float64, error) {
	v, err := pycompat.ParseFloat(s)
	if err != nil {
		return 0, cli.SystemExit("greenmax-tablero-gen: could not convert string to float: " + pycompat.StrRepr(s))
	}
	return v, nil
}

// pyInt is str(int(v)): the integer part in full (past int64 too), or Python's OverflowError
// for an infinite v.
func pyInt(v float64) (string, error) {
	switch {
	case math.IsInf(v, 0):
		return "", cli.SystemExit("greenmax-tablero-gen: cannot convert float infinity to integer")
	case math.IsNaN(v):
		return "", cli.SystemExit("greenmax-tablero-gen: cannot convert float NaN to integer")
	}
	s, _ := pycompat.IntOfFloat(v)
	return s, nil
}

// pyFixed1 is f"{v:.1f}": "inf" (not Go's "+Inf") for an infinite v.
func pyFixed1(v float64) string {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return pycompat.FloatRepr(v)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
}

// detectPills maps each circuit to its pill (Relay[k].BO 60x30 Picture) position; a later pill wins.
func detectPills(text string) (map[int]point, error) {
	pos := map[int]point{}
	for _, m := range pillRE.FindAllStringSubmatch(text, -1) {
		x, err := pyFloat(m[1])
		if err != nil {
			return nil, err
		}
		y, err := pyFloat(m[2])
		if err != nil {
			return nil, err
		}
		k, _ := strconv.Atoi(m[3])
		pos[k] = point{x, y}
	}
	return pos, nil
}

// horarioLegend is the read-only "Horarios" legend: a title and 5 rows bound to the shared names.
func horarioLegend(x int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "    <Label layout=\"%d.0,140.0,240.0,18.0\" font=\"bold 12.0pt Arial\" halign=\"left\" text=\"Horarios:\" />\n", x)
	for i, y := range []int{162, 180, 198, 216, 234} {
		fmt.Fprintf(&b, "    <Label layout=\"%d.0,%d.0,250.0,16.0\" font=\"11.0pt Arial\" halign=\"left\">"+
			"<ValueBinding ord=\"station:|slot:/Iluminacion/Horarios/Horario%d_Nombre\">"+
			"<ObjectToString name=\"text\" format=\"%d  %%out.value%%\" /></ValueBinding></Label>\n", x, y, i+1, i+1)
	}
	return b.String()
}

// stats are the counts printed on stderr, in the Python dict order.
type stats struct {
	circuits                  int
	missing                   []int
	hoaRemoved, popupsRemoved int
	pills, leds               int
	namemux, selrset          int
	bumped                    *[2]float64 // (old height, rounded new height)
}

func (s stats) String() string {
	miss := make([]string, len(s.missing))
	for i, k := range s.missing {
		miss[i] = strconv.Itoa(k)
	}
	bumped := "None"
	if s.bumped != nil {
		bumped = "(" + pycompat.FloatRepr(s.bumped[0]) + ", " + strconv.FormatFloat(s.bumped[1], 'f', 0, 64) + ")"
	}
	return fmt.Sprintf("circuits=%d missing=[%s] hoa_removed=%d popups_removed=%d pills=%d leds=%d namemux=%d "+
		"selrset=%d view_bumped=%s", s.circuits, strings.Join(miss, ", "), s.hoaRemoved, s.popupsRemoved,
		s.pills, s.leds, s.namemux, s.selrset, bumped)
}

// transform removes the HOA buttons and calendar popups, appends a name selector under every
// pill (and the legend), grows viewSize when needed and checks the result is well-formed XML.
func transform(text, gm string, legend bool) (string, int, stats, error) {
	var st stats
	pos, err := detectPills(text)
	if err != nil {
		return "", 0, st, err
	}
	if len(pos) == 0 {
		return "", 0, st, cli.SystemExit("NO PILLS FOUND (no Relay[k].BO 60x30 Pictures) — inspect the Px manually")
	}
	ks := make([]int, 0, len(pos))
	for k := range pos {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	n := ks[len(ks)-1]
	st.missing = []int{}
	for k := 1; k <= n; k++ {
		if _, ok := pos[k]; !ok {
			st.missing = append(st.missing, k)
		}
	}
	st.circuits = n
	st.hoaRemoved, st.popupsRemoved = len(hoaRE.FindAllStringIndex(text, -1)), len(popRE.FindAllStringIndex(text, -1))

	text = hoaBtnRE.ReplaceAllLiteralString(text, "")
	text = popPicRE.ReplaceAllLiteralString(text, "")
	if left := len(hoaRE.FindAllStringIndex(text, -1)) + len(popRE.FindAllStringIndex(text, -1)); left > 0 {
		return "", 0, st, cli.SystemExit(fmt.Sprintf("%d HOA button/calendar popup reference(s) survived the removal "+
			"-- inspect the Px manually", left))
	}

	var sel strings.Builder
	maxY := math.Inf(-1)
	for _, k := range ks {
		p := pos[k]
		maxY = math.Max(maxY, p.y)
		xi, err := pyInt(p.x)
		if err != nil {
			return "", 0, st, err
		}
		yi, err := pyInt(p.y + 30)
		if err != nil {
			return "", 0, st, err
		}
		fmt.Fprintf(&sel, "    <Picture layout=\"%s.0,%s.0,60.0,22.0\" image=\"%s\" scale=\"fitWidth\" />\n", xi, yi, imgNormal)
		fmt.Fprintf(&sel, "    <Label layout=\"%s.0,%s.0,60.0,20.0\" halign=\"center\" font=\"bold 10.0pt Arial\">"+
			"<ValueBinding ord=\"station:|slot:/Iluminacion/%s/NameMux%d\">"+
			"<ObjectToString name=\"text\" format=\"%%out.value%%\" /></ValueBinding></Label>\n", xi, yi, gm, k)
		fmt.Fprintf(&sel, "    <ImageButton layout=\"%s.0,%s.0,60.0,22.0\" buttonStyle=\"none\">"+
			"<ActionBinding ord=\"station:|slot:/Iluminacion/%s/SelR%d/set\" "+
			"widgetEvent=\"actionPerformed\" /></ImageButton>\n", xi, yi, gm, k)
	}
	if legend {
		sel.WriteString(horarioLegend(1080))
	}

	maxy := maxY + 30 + 22 + 20
	if vm := viewSizeRE.FindStringSubmatch(text); vm != nil {
		vw, err := pyFloat(vm[1])
		if err != nil {
			return "", 0, st, err
		}
		vh, err := pyFloat(vm[2])
		if err != nil {
			return "", 0, st, err
		}
		if maxy > vh {
			text = strings.ReplaceAll(text, vm[0], `viewSize="`+pyFixed1(vw)+","+pyFixed1(maxy)+`"`)
			st.bumped = &[2]float64{vh, math.RoundToEven(maxy)}
		}
	}

	if c := strings.Count(text, "</CanvasPane>"); c != 1 {
		return "", 0, st, cli.SystemExit(fmt.Sprintf("expected exactly one </CanvasPane>, found %d", c))
	}
	text = strings.Replace(text, "</CanvasPane>", sel.String()+"  </CanvasPane>", 1)

	if err := checkWellFormed(text); err != nil {
		// Python's minidom raises an ExpatError traceback here.
		return "", 0, st, cli.SystemExit("greenmax-tablero-gen: the generated Px is not well-formed XML: " + err.Error())
	}
	st.pills = strings.Count(text, "GreenMax On.png")
	st.leds = strings.Count(text, "<IStatusToSimple")
	for _, c := range []struct {
		dst *int
		pat string
	}{{&st.namemux, `/Iluminacion/` + gm + `/NameMux[0-9]+`}, {&st.selrset, `/Iluminacion/` + gm + `/SelR[0-9]+/set`}} {
		re, err := regexp.Compile(c.pat)
		if err != nil {
			return "", 0, st, cli.SystemExit("greenmax-tablero-gen: --gm is not usable in a pattern: " + err.Error())
		}
		*c.dst = len(re.FindAllStringIndex(text, -1))
	}
	return text, n, st, nil
}

// checkWellFormed is the minidom.parseString check (expat in namespace mode): one root element,
// matched tags, known entities, only comments, processing instructions or XML whitespace outside
// the root, plus the expat rules encoding/xml does not enforce (checkExpatRules).
func checkWellFormed(text string) error {
	if err := checkStructure(text); err != nil {
		return err
	}
	return checkExpatRules(text)
}

// isXMLSpace is the XML S production; Unicode spaces such as U+00A0 are not markup whitespace.
func isXMLSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func checkStructure(text string) error {
	d := xml.NewDecoder(strings.NewReader(text))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	depth, roots := 0, 0
	for first := true; ; first = false {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots > 1 {
					return fmt.Errorf("junk after document element: <%s>", t.Name.Local)
				}
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			s := string(t)
			if first { // expat accepts a byte order mark at the very start
				s = strings.TrimPrefix(s, "\ufeff")
			}
			if depth == 0 && strings.TrimFunc(s, isXMLSpace) != "" {
				return errors.New("text outside the document element")
			}
		}
	}
	if roots == 0 {
		return errors.New("no element found")
	}
	return nil
}

// xmlNamespace is the namespace the xml prefix is always bound to.
const xmlNamespace = "http://www.w3.org/XML/1998/namespace"

var charRefRE = regexp.MustCompile(`&#(?:x([0-9a-fA-F]+)|([0-9]+));`)

// checkExpatRules applies, on an already well-structured document, what expat rejects and
// encoding/xml accepts: duplicate attributes (by name and by namespace + local name), unbound or
// undeclared prefixes, character references to non-characters (surrogates), an XML declaration
// that is not at the start (after an optional byte order mark), a reserved processing-instruction
// target (any case of "xml" other than the declaration), and a <!...> declaration that is not the
// one DOCTYPE before the root element.
func checkExpatRules(text string) error {
	d := xml.NewDecoder(strings.NewReader(text))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	fail := func(format string, args ...any) error {
		line, _ := d.InputPos()
		return fmt.Errorf("XML syntax error on line %d: %s", line, fmt.Sprintf(format, args...))
	}
	start := 0
	if strings.HasPrefix(text, "\ufeff") {
		start = len("\ufeff")
	}
	scopes := []map[string]string{{"xml": xmlNamespace}}
	elements, doctype := 0, false
	for {
		from := int(d.InputOffset())
		tok, err := d.RawToken()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		raw := text[from:d.InputOffset()]
		switch t := tok.(type) {
		case xml.ProcInst:
			switch {
			case strings.EqualFold(t.Target, "xml") && t.Target != "xml":
				return fail("reserved processing-instruction target %q", t.Target)
			case t.Target == "xml" && from != start:
				return fail("XML declaration not at start of entity")
			}
		case xml.Directive:
			if elements > 0 || doctype || !strings.HasPrefix(string(t), "DOCTYPE") {
				return fail("misplaced or invalid <!%s> declaration", pycompat.Cut(string(t), 20))
			}
			doctype = true
		case xml.StartElement:
			elements++
			scope := map[string]string{}
			for k, v := range scopes[len(scopes)-1] {
				scope[k] = v
			}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					if a.Value == "" {
						return fail("must not undeclare prefix %q", a.Name.Local)
					}
					scope[a.Name.Local] = a.Value
				}
			}
			if t.Name.Space != "" && scope[t.Name.Space] == "" {
				return fail("unbound prefix %q", t.Name.Space)
			}
			byName, byExpanded := map[string]bool{}, map[string]bool{}
			for _, a := range t.Attr {
				name := a.Name.Space + ":" + a.Name.Local
				if byName[name] {
					return fail("duplicate attribute %s", strings.TrimPrefix(name, ":"))
				}
				byName[name] = true
				if a.Name.Space == "" || a.Name.Space == "xmlns" {
					continue
				}
				uri := scope[a.Name.Space]
				if uri == "" {
					return fail("unbound prefix %q", a.Name.Space)
				}
				if byExpanded[uri+" "+a.Name.Local] {
					return fail("duplicate attribute {%s}%s", uri, a.Name.Local)
				}
				byExpanded[uri+" "+a.Name.Local] = true
			}
			scopes = append(scopes, scope)
			if err := checkCharRefs(raw, fail); err != nil {
				return err
			}
		case xml.EndElement:
			scopes = scopes[:len(scopes)-1]
		case xml.CharData:
			if !strings.HasPrefix(raw, "<![CDATA[") { // a CDATA section holds no references
				if err := checkCharRefs(raw, fail); err != nil {
					return err
				}
			}
		}
	}
}

// checkCharRefs rejects a character reference to a code point outside the XML Char production.
func checkCharRefs(raw string, fail func(string, ...any) error) error {
	for _, m := range charRefRE.FindAllStringSubmatch(raw, -1) {
		base, digits := 16, m[1]
		if digits == "" {
			base, digits = 10, m[2]
		}
		n, err := strconv.ParseUint(digits, base, 32)
		if err != nil || !isXMLChar(rune(n)) {
			return fail("reference to invalid character number %s", m[0])
		}
	}
	return nil
}

func isXMLChar(r rune) bool {
	return r == 0x9 || r == 0xa || r == 0xd || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd ||
		r >= 0x10000 && r <= 0x10ffff
}
