// Package pxrender renders a Niagara N4 Px (.px) as a self-contained HTML page (port of
// niagara-research tools/px-render.py): a CanvasPane of absolutely positioned widgets becomes an
// HTML page with every referenced image embedded as a data: URI. Relay ON/OFF is demo state
// derived from the relay ord (OnRatio, Seed), never live.
package pxrender

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"syscall"

	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/honest"
	"github.com/angeles725/niagara-go/internal/pycompat"
)

// Options is the parsed command line. Empty strings mean "not given" (Python None).
type Options struct {
	Px        string  // the .px file, or "selftest"
	Shared    string  // station shared/ root; auto-detected when empty
	Organized string  // organized/ corpus root for module:// resources; none when empty
	Out       string  // output .html path; stdout when empty
	OnRatio   float64 // demo fraction of relays ON (the command-line default is 0.7)
	Seed      string  // demo-state seed tag (default: px basename)
	Title     string  // page <title>
	Subtitle  string  // note line under the header (raw HTML)
}

// strerror renders an I/O error the way Python's OSError.strerror does.
func strerror(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		if msg := errno.Error(); msg != "" {
			return strings.ToUpper(msg[:1]) + msg[1:]
		}
	}
	return err.Error()
}

func pageHTML(o Options, name, cw, ch, body string) string {
	title := o.Title
	if title == "" {
		title = "Px · " + name
	}
	sub := o.Subtitle
	if sub == "" {
		sub = fmt.Sprintf("Pantalla Px real reproducida desde el config.bog: <b>%s</b>. Plano, botones On/Off, "+
			"LEDs de estado y etiquetas en las mismas posiciones.", pycompat.HTMLEscape(name))
	}
	return fmt.Sprintf("<title>%s</title>\n"+
		"<style>\n"+
		":root{color-scheme:light}\n"+
		"body{margin:0;background:#0b1220;font-family:Arial,Helvetica,sans-serif;}\n"+
		".bar{background:#001D68;color:#fff;padding:10px 18px;font-size:13px;display:flex;gap:12px;align-items:center;}\n"+
		".bar b{font-weight:700}.bar .tag{margin-left:auto;font-size:11px;background:#76B900;color:#0c1207;"+
		"font-weight:700;border-radius:4px;padding:3px 8px;}\n"+
		".note{background:#e9edf3;color:#3a4453;font-size:12px;padding:8px 18px;border-bottom:1px solid #d2d8e2;}\n"+
		".stage{width:100%%;overflow:auto;background:#f4f2ec;}\n"+
		".canvas{position:relative;width:%spx;height:%spx;transform-origin:top left;background:#fff;}\n"+
		".canvas img{-webkit-user-drag:none;user-select:none;}\n"+
		"</style>\n"+
		"<div class=\"bar\"><b>Px actual</b> (Niagara Workbench Px Editor) <span class=\"tag\">RÉPLICA</span></div>\n"+
		"<div class=\"note\">%s</div>\n"+
		"<div class=\"stage\"><div class=\"canvas\" id=\"cv\">\n%s\n</div></div>\n"+
		"<script>function fit(){var cv=document.getElementById(\"cv\");var w=cv.parentElement.clientWidth;"+
		"var s=Math.min(1,w/%s);cv.style.transform=\"scale(\"+s+\")\";cv.parentElement.style.height=(%s*s)+\"px\";}\n"+
		"window.addEventListener(\"resize\",fit);fit();</script>\n",
		pycompat.HTMLEscape(title), cw, ch, sub, body, cw, ch)
}

// Run executes one px-render invocation: 0 rendered, 1 no CanvasPane (or a selftest failure),
// 2 an unreadable or malformed .px or asset.
func Run(o Options, s cli.Stdio) int {
	if o.Px == "selftest" {
		return selftest(s)
	}
	page, res, err := render(o)
	var noCanvas errNoCanvas
	switch {
	case errors.As(err, &noCanvas):
		fmt.Fprintln(s.Err, err)
		return cli.ExitNoMatch
	case err != nil:
		return honest.Fail(s.Err, "px-render", err.Error(), honest.ExitInput)
	}
	if o.Out != "" {
		if err := os.WriteFile(o.Out, []byte(page), 0o666); err != nil {
			return honest.Fail(s.Err, "px-render", fmt.Sprintf("cannot write %s: %s", o.Out, strerror(err)), honest.ExitInput)
		}
		fmt.Fprintf(s.Out, "wrote %s  (%d widgets, %d assets, %.2f MB, canvas %sx%s)\n", o.Out, res.widgets,
			res.assets, float64(len(page))/1048576, res.canvasW, res.canvasH)
		fmt.Fprintf(s.Out, "shared root: %s\n", res.shared)
		if len(res.missing) > 0 {
			fmt.Fprintf(s.Out, "missing assets (%d):\n", len(res.missing))
			for _, m := range res.missing {
				fmt.Fprintf(s.Out, "   %s\n", m)
			}
		}
	} else {
		io.WriteString(s.Out, page)
	}
	for _, n := range res.notImages {
		fmt.Fprintf(s.Err, "px-render: %s\n", n) // a warning: the render goes on
	}
	writeDropped(s.Err, res.dropped)
	return cli.ExitFound
}

func writeDropped(w io.Writer, dropped map[string]int) {
	if len(dropped) == 0 {
		return
	}
	keys := make([]string, 0, len(dropped))
	total := 0
	for k, n := range dropped {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	items := make([]string, len(keys))
	for i, k := range keys {
		items[i] = fmt.Sprintf("%s x%d", k, dropped[k])
	}
	fmt.Fprintf(w, "NOT rendered (%d): %s\n", total, strings.Join(items, ", "))
}
