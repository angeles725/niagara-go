// Package greenmaxmatrix is the Go port of niagara-research tools/greenmax-matrix.py: the
// lighting points matrix of a GreenMAX/Niagara site, one row per lighting circuit with the
// command chain traced from the supervisor schedule to the JACE BACnet relay. Read-only.
package greenmaxmatrix

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/bog"
	"github.com/angeles725/niagara-go/internal/cli"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pycsv"
)

// prog is the Python program name, kept for byte-identical usage and error lines.
const prog = "greenmax-matrix.py"

func newParser() *argparse.Parser {
	p := argparse.New(argparse.Config{Prog: prog, Description: "Build the GreenMAX lighting points matrix."})
	p.Add(argparse.Arg{}, "supervisor")
	p.Add(argparse.Arg{}, "jace")
	p.Add(argparse.Arg{}, "px_dir")
	p.Add(argparse.Arg{Default: "matriz-puntos-DRAFT.csv"}, "-o", "--out")
	return p
}

// Main is the greenmax-matrix command line. Exit 0 ok, 1 no GreenMAX panel in either bog (or a
// px_dir that is not a directory), 2 the bogs in swapped order, 3 an unreadable bog.
func Main(args []string, s cli.Stdio) int {
	ns, code, done := newParser().Parse(args, s)
	if done {
		return code
	}
	return run(ns.String("supervisor"), ns.String("jace"), ns.String("px_dir"), ns.String("out"), s)
}

// loadBog is bog-nav's Bog(path): an unreadable input exits 3 with a "bog-nav:" line.
func loadBog(path string, s cli.Stdio) (*bog.Bog, bool) {
	b, err := bog.Load(path)
	if err != nil {
		fmt.Fprintf(s.Err, "bog-nav: %s\n", err)
		return nil, false
	}
	return b, true
}

func scope(b *bog.Bog) string {
	return fmt.Sprintf("searched %s: %d components, %d links", b.SrcPath, b.Comps.Len(), len(b.Links))
}

var fields = []string{"GM", "Tablero", "Zona", "Relay", "DescripcionCircuito", "CircuitLabel_Px", "Schedule",
	"CmdWritable", "SchedLink_in10", "JACE_driven", "JACE_write_prio", "BACnet_device", "BACnet_objectId"}

func run(supPath, jacePath, pxDir, out string, s cli.Stdio) int {
	if st, err := os.Stat(pxDir); err != nil || !st.IsDir() {
		fmt.Fprintf(s.Err, "greenmax-matrix: px_dir is not a directory: %s\n", pxDir)
		return 1
	}
	supBog, ok := loadBog(supPath, s)
	if !ok {
		return bog.ExitBadInput
	}
	jaceBog, ok := loadBog(jacePath, s)
	if !ok {
		return bog.ExitBadInput
	}
	sup, jac := supPanels(supBog), jacePanels(jaceBog)
	if len(sup) == 0 && len(supPanels(jaceBog)) > 0 || len(jac) == 0 && len(jacePanels(supBog)) > 0 {
		fmt.Fprintf(s.Err, "greenmax-matrix: the two bogs look swapped: the GM<n>ilum NiagaraPointFolders "+
			"(supervisor role) are in %s and/or the GM<n>ilum BacnetDevices (JACE role) "+
			"are in %s; usage: greenmax-matrix.py <supervisor.bog> <jace.bog> <px_dir>\n", jacePath, supPath)
		return 2
	}
	px, err := parsePx(pxDir)
	if err != nil {
		// Python ends in a traceback on a Px it cannot open.
		fmt.Fprintf(s.Err, "greenmax-matrix: %s\n", pycompat.OSErrorText(err))
		return cli.ExitCannotSearch
	}
	if len(sup) == 0 && len(jac) == 0 {
		fmt.Fprintf(s.Err, "greenmax-matrix: 0 GreenMAX panels (no GM<n>ilum NiagaraPointFolder / BacnetDevice); "+
			"supervisor %s; jace %s; %d panel(s) in Px %s\n", scope(supBog), scope(jaceBog), len(px), pxDir)
		return 1
	}
	rows := buildRows(sup, jac, px)
	if err := writeCSV(out, rows); err != nil {
		// Python ends in a traceback when the output cannot be written.
		fmt.Fprintf(s.Err, "greenmax-matrix: %s\n", pycompat.OSErrorText(err))
		return cli.ExitCannotSearch
	}
	summary(s.Out, sup, jac, px, rows, out)
	return 0
}

// keys are the keys of m.
func keys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// union is the sorted set of the given keys.
func union(lists ...[]int) []int {
	set := map[int]bool{}
	for _, l := range lists {
		for _, k := range l {
			set[k] = true
		}
	}
	out := keys(set)
	sort.Ints(out)
	return out
}

func gmName(gm int) string { return fmt.Sprintf("GM%02d", gm) }

// buildRows makes one row (fields order) per relay of every panel.
func buildRows(sup map[int]*supPanel, jac map[int]*jacePanel, px map[int]*pxPanel) [][]string {
	var rows [][]string
	for _, gm := range union(keys(sup), keys(jac), keys(px)) {
		sp, jp, pp := sup[gm], jac[gm], px[gm]
		if sp == nil {
			sp = &supPanel{}
		}
		if jp == nil {
			jp = newJace("")
		}
		if pp == nil {
			pp = &pxPanel{}
		}
		tabs := make([]string, 0, len(pp.tableros))
		for t := range pp.tableros {
			tabs = append(tabs, t)
		}
		sort.Strings(tabs)
		pxTab := strings.Join(tabs, "/")
		for _, k := range union(keys(sp.relays), keys(jp.bo)) {
			desc := sp.labels[k]
			if desc == "" {
				desc = pp.labels[k]
			}
			tab := sp.tabs[k]
			if tab == "" {
				tab = pxTab
			}
			if tab == "" {
				tab = "(no Px)"
			}
			sched, cmd := "", ""
			if sp.scheds[k] {
				sched = fmt.Sprintf("R%d/Schedule%d", k, k)
			}
			if c := sp.writables[k]; c != nil {
				cmd = c.Name
			}
			device := jp.device
			if _, ok := jac[gm]; device == "" && ok {
				device = gmName(gm) + "ilum"
			}
			rows = append(rows, []string{gmName(gm), tab, pp.zone, strconv.Itoa(k), desc, pp.labels[k], sched, cmd,
				yesOr(sp.schedLinked[k]), yesOr(jp.feedOK[k]), jp.feedPrio[k], device, jp.bo[k]})
		}
	}
	return rows
}

func yesOr(b bool) string {
	if b {
		return "yes"
	}
	return "?"
}

// writeCSV writes the header and rows with csv.writer (pycsv: QUOTE_MINIMAL, "\r\n").
func writeCSV(path string, rows [][]string) error {
	var b strings.Builder
	w := pycsv.NewWriter(&b)
	for _, r := range append([][]string{fields}, rows...) {
		row := make([]any, len(r))
		for i, f := range r {
			row[i] = f
		}
		_ = w.WriteRow(row) // a strings.Builder never fails
	}
	return os.WriteFile(path, []byte(b.String()), 0o666)
}

// summary prints the totals line and the per-panel table.
func summary(w io.Writer, sup map[int]*supPanel, jac map[int]*jacePanel, px map[int]*pxPanel, rows [][]string, out string) {
	desc := 0
	for _, r := range rows {
		if r[4] != "" {
			desc++
		}
	}
	panels := union(keys(sup), keys(jac))
	fmt.Fprintf(w, "panels=%d  circuits=%d  with-description=%d  -> %s\n", len(panels), len(rows), desc, out)
	fmt.Fprintf(w, "%-5s%-12s%-18s%7s%6s%8s%7s\n", "GM", "Tablero", "Zona", "relays", "desc", "schedOK", "feedOK")
	for _, gm := range panels {
		var gr [][]string
		for _, r := range rows {
			if r[0] == gmName(gm) {
				gr = append(gr, r)
			}
		}
		so, fo, dc := 0, 0, 0
		for _, r := range gr {
			if r[8] == "yes" {
				so++
			}
			if r[9] == "yes" {
				fo++
			}
			if r[4] != "" {
				dc++
			}
		}
		z := ""
		if p := px[gm]; p != nil {
			z = pycompat.Cut(p.zone, 16)
		}
		t := ""
		if len(gr) > 0 {
			t = pycompat.Cut(gr[0][1], 11)
		}
		fmt.Fprintf(w, "%s  %-12s%-18s%7d%6d%8d%7d\n", gmName(gm), t, z, len(gr), dc, so, fo)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
