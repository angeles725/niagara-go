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
)

const imgNormal = "file:^Imagenes/The Harbor/Buttons/BotonNormal.png"

var (
	pillRE = regexp.MustCompile(pyRe(`<Picture layout="([0-9.]+),([0-9.]+),60\.0,30\.0"[^>]*>\s*` +
		`<ValueBinding ord="slot:Relay\$5b([0-9]+)\$5d\$2eBO">\s*<IBooleanToSimple`))
	hoaRx    = `Relay\$5b[0-9]+\$5d\$2eBO/(?:active|inactive|auto)"`
	popRx    = `PopupBinding\b[^>]*\bord="slot:R[0-9]+"`
	hoaRE    = regexp.MustCompile(hoaRx)
	popRE    = regexp.MustCompile(popRx)
	hoaBtnRE = regexp.MustCompile(pyRe(`(?s)\s*<ImageButton\b[^>]*>\s*<ActionBinding\b[^>]*\bord="slot:` + hoaRx +
		`[^>]*>.*?</ActionBinding>\s*</ImageButton>`))
	popPicRE   = regexp.MustCompile(pyRe(`(?s)\s*<Picture\b[^>]*>\s*<` + popRx + `[^>]*/>\s*</Picture>`))
	viewSizeRE = regexp.MustCompile(`viewSize="([0-9.]+),([0-9.]+)"`)
)

// exitError is a sys.exit(message) of the Python tool: the message on stderr, exit 1.
type exitError struct{ msg string }

func (e *exitError) Error() string { return e.msg }

type point struct{ x, y float64 }

// pyFloat is float() of a "[\d.]+" match.
func pyFloat(s string) (float64, error) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || strings.Count(s, ".") > 1 {
		return 0, &exitError{"greenmax-tablero-gen: could not convert string to float: " + pyStrRepr(s)}
	}
	return v, nil
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
		bumped = "(" + pyRepr(s.bumped[0]) + ", " + strconv.FormatFloat(s.bumped[1], 'f', 0, 64) + ")"
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
		return "", 0, st, &exitError{"NO PILLS FOUND (no Relay[k].BO 60x30 Pictures) — inspect the Px manually"}
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
		return "", 0, st, &exitError{fmt.Sprintf("%d HOA button/calendar popup reference(s) survived the removal "+
			"-- inspect the Px manually", left)}
	}

	var sel strings.Builder
	maxY := math.Inf(-1)
	for _, k := range ks {
		p := pos[k]
		maxY = math.Max(maxY, p.y)
		xi, yi := int(p.x), int(p.y+30)
		fmt.Fprintf(&sel, "    <Picture layout=\"%d.0,%d.0,60.0,22.0\" image=\"%s\" scale=\"fitWidth\" />\n", xi, yi, imgNormal)
		fmt.Fprintf(&sel, "    <Label layout=\"%d.0,%d.0,60.0,20.0\" halign=\"center\" font=\"bold 10.0pt Arial\">"+
			"<ValueBinding ord=\"station:|slot:/Iluminacion/%s/NameMux%d\">"+
			"<ObjectToString name=\"text\" format=\"%%out.value%%\" /></ValueBinding></Label>\n", xi, yi, gm, k)
		fmt.Fprintf(&sel, "    <ImageButton layout=\"%d.0,%d.0,60.0,22.0\" buttonStyle=\"none\">"+
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
			text = strings.ReplaceAll(text, vm[0], fmt.Sprintf(`viewSize="%.1f,%.1f"`, vw, maxy))
			st.bumped = &[2]float64{vh, math.RoundToEven(maxy)}
		}
	}

	if c := strings.Count(text, "</CanvasPane>"); c != 1 {
		return "", 0, st, &exitError{fmt.Sprintf("expected exactly one </CanvasPane>, found %d", c)}
	}
	text = strings.Replace(text, "</CanvasPane>", sel.String()+"  </CanvasPane>", 1)

	if err := checkWellFormed(text); err != nil {
		// Python's minidom raises an ExpatError traceback here.
		return "", 0, st, &exitError{"greenmax-tablero-gen: the generated Px is not well-formed XML: " + err.Error()}
	}
	st.pills = strings.Count(text, "GreenMax On.png")
	st.leds = strings.Count(text, "<IStatusToSimple")
	for _, c := range []struct {
		dst *int
		pat string
	}{{&st.namemux, `/Iluminacion/` + gm + `/NameMux[0-9]+`}, {&st.selrset, `/Iluminacion/` + gm + `/SelR[0-9]+/set`}} {
		re, err := regexp.Compile(c.pat)
		if err != nil {
			return "", 0, st, &exitError{"greenmax-tablero-gen: --gm is not usable in a pattern: " + err.Error()}
		}
		*c.dst = len(re.FindAllStringIndex(text, -1))
	}
	return text, n, st, nil
}

// checkWellFormed is the minidom.parseString check: one root element, matched tags, known
// entities, and only comments, processing instructions or whitespace outside the root.
func checkWellFormed(text string) error {
	d := xml.NewDecoder(strings.NewReader(text))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	depth, roots := 0, 0
	for {
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
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return errors.New("text outside the document element")
			}
		}
	}
	if roots == 0 {
		return errors.New("no element found")
	}
	return nil
}
