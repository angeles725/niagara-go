package pxrender

import (
	"crypto/md5"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyhtml"
	"github.com/angeles725/niagara-go/internal/pypath"
	"github.com/angeles725/niagara-go/internal/textfold"
)

var widgets = map[string]bool{"Label": true, "Picture": true, "BoundLabel": true, "ImageButton": true,
	"BackButton": true}

// result is what render reports besides the page: canvas size, counts, the shared root used,
// unresolved image refs and the positioned elements it did not render.
type result struct {
	canvasW, canvasH string
	widgets          int
	shared           string
	missing          []string
	notImages        []string // refs that resolved to a directory, skipped
	assets           int
	dropped          map[string]int
	omitted          []omit
}

// omit is an element the page does not render: its tag, its "NOT rendered" summary key and, when
// its layout parses, its canvas position (offsets of the enclosing panes included).
type omit struct {
	tag, key string
	lay      layout
	hasLay   bool
}

type layout struct{ x, y, w, h float64 }

// parseLayout reads layout="x,y,w,h"; ok is false for a missing or malformed value.
func parseLayout(s string) (layout, bool) {
	parts := strings.Split(s, ",")
	if s == "" || len(parts) != 4 {
		return layout{}, false
	}
	var v [4]float64
	for i, p := range parts {
		f, err := pycompat.ParseFloat(p)
		if err != nil {
			return layout{}, false
		}
		v[i] = f
	}
	return layout{v[0], v[1], v[2], v[3]}, true
}

// parseFont: 'bold 26.0pt Arial' -> (bold, 26, Arial); '11.0pt Arial' -> (normal, 11, Arial).
func parseFont(s string) (weight, size, family string) {
	weight, size, family = "normal", "13", "Arial"
	if s == "" {
		return
	}
	toks := textfold.Fields(s)
	if len(toks) > 0 && (toks[0] == "bold" || toks[0] == "italic") {
		weight, toks = toks[0], toks[1:]
	}
	for _, tk := range toks {
		if strings.Contains(tk, "pt") {
			size, _, _ = strings.Cut(strings.ReplaceAll(tk, "pt", ""), ".")
		} else {
			family = tk
		}
	}
	return
}

var relayRE = regexp.MustCompile(`\[([0-9]+)\]`)

// relayNum: slot:Relay$5b7$5d$2eBO -> "7" (as Python int() prints it); "" when absent.
func relayNum(ord string) string {
	if ord == "" {
		return ""
	}
	m := relayRE.FindStringSubmatch(strings.ReplaceAll(strings.ReplaceAll(ord, "$5b", "["), "$5d", "]"))
	if m == nil {
		return ""
	}
	if n := strings.TrimLeft(m[1], "0"); n != "" {
		return n
	}
	return "0"
}

func holdsWidgets(e *element) bool {
	for _, c := range e.children {
		if widgets[c.tag] || holdsWidgets(c) {
			return true
		}
	}
	return false
}

type placed struct {
	e      *element
	ox, oy float64
}

// flatten lists every known widget under container with its offset, descending into any *Pane
// and into positioned elements that hold widgets; other positioned elements go to omitted.
func flatten(container *element, ox, oy float64, omitted *[]omit, out []placed) []placed {
	for _, e := range container.children {
		switch {
		case widgets[e.tag]:
			out = append(out, placed{e, ox, oy})
		case strings.HasSuffix(e.tag, "Pane") || e.attr("layout") != "" && holdsWidgets(e):
			lay, ok := parseLayout(e.attr("layout"))
			if !ok {
				lay = layout{}
			}
			out = flatten(e, ox+lay.x, oy+lay.y, omitted, out)
		case e.attr("layout") != "":
			om := omit{tag: e.tag, key: e.tag} // a malformed layout keeps the bare-tag summary key
			if lay, ok := parseLayout(e.attr("layout")); ok {
				om.lay, om.hasLay = layout{lay.x + ox, lay.y + oy, lay.w, lay.h}, true
			}
			*omitted = append(*omitted, om)
		case len(e.children) > 0:
			out = flatten(e, ox, oy, omitted, out)
		}
	}
	return out
}

const ledDiv = `<div style="%sdisplay:grid;place-items:center;"><span style="width:60%%;height:60%%;` +
	`max-width:16px;max-height:16px;border-radius:50%%;background:#9aa4ac;"></span></div>`

type renderer struct {
	a       *assets
	seed    string
	onRatio float64
	parts   []string
}

func (r *renderer) relayOn(k string) bool {
	if k == "" {
		return true
	}
	h := md5.Sum([]byte(r.seed + ":r" + k))
	mod := 0
	for _, b := range h {
		mod = (mod*256 + int(b)) % 1000
	}
	return float64(mod)/1000.0 < r.onRatio
}

func (r *renderer) img(uri, base string) {
	r.parts = append(r.parts, fmt.Sprintf(`<img src="%s" style="%sobject-fit:contain;" alt="">`, uri, base))
}

// binding renders a ValueBinding image toggle: IBooleanToSimple picks the on/off image from the
// demo state; IStatusToSimple shows the ok image when on, else a grey LED.
func (r *renderer) binding(vb *element, base string) error {
	on := r.relayOn(relayNum(vb.attr("ord")))
	if ib := vb.find("IBooleanToSimple"); ib != nil {
		tv, fv := ib.findImage("trueValue"), ib.findImage("falseValue")
		ref := ""
		if tv != nil && fv != nil {
			if ref = fv.attr("value"); on {
				ref = tv.attr("value")
			}
		}
		if ref == "" {
			return nil
		}
		uri, err := r.a.dataURI(ref)
		if uri != "" {
			r.img(uri, base)
		}
		return err
	}
	if st := vb.find("IStatusToSimple"); st != nil {
		uri := ""
		if ok := st.findImage("ok"); ok != nil {
			var err error
			if uri, err = r.a.dataURI(ok.attr("value")); err != nil {
				return err
			}
		}
		if on && uri != "" {
			r.img(uri, base)
		} else {
			r.parts = append(r.parts, fmt.Sprintf(ledDiv, base))
		}
	}
	return nil
}

func (r *renderer) widget(e *element, base string) error {
	switch e.tag {
	case "Label":
		wt, sz, fam := parseFont(e.attr("font"))
		r.parts = append(r.parts, fmt.Sprintf(`<div style="%sdisplay:flex;align-items:center;font-family:%s,Arial,`+
			`sans-serif;font-weight:%s;font-size:%spx;color:#16233a;line-height:1.05;overflow:hidden;`+
			`white-space:pre-wrap;">%s</div>`, base, fam, wt, sz, pycompat.HTMLEscape(pyhtml.Unescape(e.attr("text")))))
	case "Picture":
		if img := e.attr("image"); img != "" {
			uri, err := r.a.dataURI(img)
			if uri != "" {
				r.img(uri, base)
			}
			return err
		}
		if vb := e.find("ValueBinding"); vb != nil {
			return r.binding(vb, base)
		}
	case "BoundLabel":
		if vb := e.find("ValueBinding"); vb != nil {
			return r.binding(vb, base)
		}
		leaf := ""
		if blb := e.find("BoundLabelBinding"); blb != nil {
			segs := strings.Split(strings.TrimRight(blb.attr("ord"), "/"), "/")
			leaf = segs[len(segs)-1]
			leaf = leaf[strings.LastIndex(leaf, ":")+1:]
		}
		r.parts = append(r.parts, fmt.Sprintf(`<div style="%sdisplay:flex;align-items:center;justify-content:center;`+
			`font-family:Arial,sans-serif;font-size:12px;color:#16233a;background:#eef1f5;border:1px solid #cdd4de;`+
			`border-radius:4px;text-align:center;padding:2px;">%s</div>`, base, pycompat.HTMLEscape(leaf)))
	case "ImageButton":
		if txt := pycompat.Strip(e.attr("text")); txt != "" {
			r.parts = append(r.parts, fmt.Sprintf(`<div style="%sdisplay:flex;align-items:center;`+
				`justify-content:center;font-family:Arial,sans-serif;font-size:11px;font-weight:600;color:#00123F;`+
				`background:#eef1f5;border:1px solid #cdd4de;border-radius:4px;">%s</div>`, base, pycompat.HTMLEscape(txt)))
		} else {
			r.parts = append(r.parts, fmt.Sprintf(`<div style="%s" title="control"></div>`, base))
		}
	case "BackButton":
		r.parts = append(r.parts, fmt.Sprintf(`<div style="%sdisplay:grid;place-items:center;color:#00123F;`+
			`font-size:20px;">&lsaquo;</div>`, base))
	}
	return nil
}

// errNoCanvas is the Python SystemExit("no CanvasPane found in <px>"): exit 1.
type errNoCanvas string

func (e errNoCanvas) Error() string { return "no CanvasPane found in " + string(e) }

func render(o Options) (string, *result, error) {
	shared := findShared(o.Px, o.Shared)
	r := &renderer{a: newAssets(shared, o.Organized), seed: o.Seed, onRatio: o.OnRatio}
	if r.seed == "" {
		r.seed, _ = pypath.PosixSplitext(pypath.PosixBasename(o.Px))
	}
	data, err := os.ReadFile(o.Px)
	if err != nil {
		return "", nil, fmt.Errorf("cannot read %s: %s", o.Px, pycompat.StrerrorOf(err))
	}
	root, err := parseXML(data)
	if err != nil {
		return "", nil, fmt.Errorf("cannot parse %s: %v", o.Px, err)
	}
	canvas := root.first("CanvasPane")
	if canvas == nil {
		return "", nil, errNoCanvas(o.Px)
	}
	cw, ch, err := viewSize(canvas)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %v", o.Px, err)
	}
	var omitted []omit
	for _, p := range flatten(canvas, 0, 0, &omitted, nil) {
		lay, ok := parseLayout(p.e.attr("layout"))
		if !ok {
			omitted = append(omitted, omit{tag: p.e.tag, key: p.e.tag + " (no layout)"})
			continue
		}
		base := fmt.Sprintf("position:absolute;left:%spx;top:%spx;width:%spx;height:%spx;",
			pycompat.FormatG(lay.x+p.ox), pycompat.FormatG(lay.y+p.oy), pycompat.FormatG(lay.w), pycompat.FormatG(lay.h))
		if err := r.widget(p.e, base); err != nil {
			return "", nil, err
		}
	}
	dropped := map[string]int{}
	for _, om := range omitted {
		dropped[om.key]++
	}
	title, _ := pypath.PosixSplitext(pypath.PosixBasename(o.Px))
	body := strings.Join(append(append([]string(nil), r.parts...), placeholders(omitted)...), "\n")
	page := pageHTML(o, title, cw, ch, body, omitted)
	return page, &result{canvasW: cw, canvasH: ch, widgets: len(r.parts), shared: shared,
		missing: r.a.missingSorted(), notImages: r.a.notImages, assets: len(r.a.cache), dropped: dropped,
		omitted: omitted}, nil
}

// viewSize is int(float(v)) of each half of viewSize="W,H" (default 1370.0,780.0).
func viewSize(canvas *element) (string, string, error) {
	vs, ok := canvas.get("viewSize")
	if !ok {
		vs = "1370.0,780.0"
	}
	wh := strings.Split(vs, ",")
	if len(wh) != 2 {
		return "", "", fmt.Errorf("viewSize %q is not W,H", vs)
	}
	var out [2]string
	for i, s := range wh {
		f, err := pycompat.ParseFloat(s)
		ok := err == nil
		if ok {
			out[i], ok = pycompat.IntOfFloat(f)
		}
		if !ok {
			return "", "", fmt.Errorf("viewSize %q is not W,H", vs)
		}
	}
	return out[0], out[1], nil
}

// placeholders are dashed boxes at the canvas position of each omitted element, so the page
// shows where something is missing instead of an empty area.
func placeholders(omitted []omit) []string {
	var out []string
	for _, om := range omitted {
		if !om.hasLay {
			continue
		}
		out = append(out, fmt.Sprintf(`<div class="omit" style="position:absolute;left:%spx;top:%spx;width:%spx;`+
			`height:%spx;" title="no reproducido">%s (no reproducido)</div>`, pycompat.FormatG(om.lay.x),
			pycompat.FormatG(om.lay.y), pycompat.FormatG(om.lay.w), pycompat.FormatG(om.lay.h), pycompat.HTMLEscape(om.tag)))
	}
	return out
}

// omittedNotice is the visible list of the elements the page does not render; "" when none.
func omittedNotice(omitted []omit) string {
	if len(omitted) == 0 {
		return ""
	}
	items := make([]string, len(omitted))
	for i, om := range omitted {
		where := "sin layout válido: posición desconocida"
		if om.hasLay {
			where = fmt.Sprintf("en x=%s, y=%s (%s×%s)", pycompat.FormatG(om.lay.x), pycompat.FormatG(om.lay.y),
				pycompat.FormatG(om.lay.w), pycompat.FormatG(om.lay.h))
		}
		items[i] = "<li>" + pycompat.HTMLEscape(om.tag) + " " + where + "</li>"
	}
	return fmt.Sprintf("<div class=\"omitted\"><b>No reproducido (%d):</b> estos elementos del Px no se dibujan "+
		"en esta réplica.<ul>%s</ul></div>\n", len(omitted), strings.Join(items, ""))
}
