package greenmaxmatrix

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// navHeaders are the shared nav-bar labels of every panel Px (never a zone).
var navHeaders = map[string]bool{"periferico": true, "calle 60": true, "prolongacion montejo": true,
	"vialidad montejo": true, "horario": true, "iluminacion": true, "iluminación": true}

var (
	pxSkipRE   = regexp.MustCompile(`Vent|Iluminacion`)
	pxNameRE   = regexp.MustCompile(pyRe(`^GreenMAX0*([0-9]+)\s+Tab(AL[0-9]+)`))
	pxTokRE    = regexp.MustCompile(`ord="slot:Relay\$5b([0-9]+)\$5d\$2eBO"|text="(AL[0-9]+-[0-9]+)"`)
	pxTextRE   = regexp.MustCompile(`text="([^"]{2,30})"`)
	digitsRE   = regexp.MustCompile(`^[0-9]+$`)
	pxStatusRE = regexp.MustCompile(`^(?i)(auto|fault|down|overridden|normal|error|manual|green)`)
)

// pxPanel is what the Px files of one GM panel tell: tableros, zone and relay circuit labels.
type pxPanel struct {
	tableros map[string]bool
	zone     string
	labels   map[int]string
}

// pxFiles is glob(px_dir/GreenMAX*.px), sorted.
func pxFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, "GreenMAX") && strings.HasSuffix(n, ".px") && len(n) >= len("GreenMAX.px") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// parsePx mines the panel Px files: GM -> tableros (from the file name), zone and circuit labels.
func parsePx(dir string) (map[int]*pxPanel, error) {
	out := map[int]*pxPanel{}
	for _, base := range pxFiles(dir) {
		if pxSkipRE.MatchString(base) {
			continue
		}
		m := pxNameRE.FindStringSubmatch(base)
		if m == nil {
			continue
		}
		gm, tab := atoi(m[1]), m[2]
		e := out[gm]
		if e == nil {
			e = &pxPanel{tableros: map[string]bool{}, labels: map[int]string{}}
			out[gm] = e
		}
		e.tableros[tab] = true
		data, err := os.ReadFile(filepath.Join(dir, base))
		if err != nil {
			return nil, err
		}
		txt := pycompat.DecodeReplace(data)
		cur, have := 0, false
		for _, t := range pxTokRE.FindAllStringSubmatch(txt, -1) {
			if t[1] != "" {
				cur, have = atoi(t[1]), true
			} else if t[2] != "" && have {
				if _, ok := e.labels[cur]; !ok {
					e.labels[cur] = t[2]
				}
			}
		}
		for _, t := range pxTextRE.FindAllStringSubmatch(txt, -1) {
			tt := pycompat.Strip(pycompat.HTMLUnescape(t[1]))
			low := strings.ToLower(tt)
			if navHeaders[low] || digitsRE.MatchString(tt) || pxStatusRE.MatchString(tt) ||
				strings.Contains(low, "comunicaci") || head(tt, 2) == "AL" {
				continue
			}
			if e.zone == "" {
				e.zone = tt
			} else if !strings.Contains(e.zone, tt) && utf8.RuneCountInString(e.zone) < 40 {
				e.zone += " / " + tt
			}
		}
	}
	return out, nil
}
