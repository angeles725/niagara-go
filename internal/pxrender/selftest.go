package pxrender

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/angeles725/niagara-go/internal/cli"
)

const selftestPx = `<?xml version="1.0" encoding="UTF-8"?>
<px version="1.0"><import/><content><ScrollPane><CanvasPane name="content" viewSize="400.0,300.0">` +
	`<Label layout="10.0,10.0,200.0,30.0" text="Cuarto 1" font="bold 18.0pt Arial"/>` +
	`<ImageButton layout="10.0,50.0,60.0,20.0" text="Auto"/>` +
	`<Picture layout="10.0,80.0,40.0,40.0">` +
	`<ValueBinding ord="slot:Relay$5b1$5d$2eBO"><IBooleanToSimple name="image">` +
	`<Image name="trueValue" value="file:^Imagenes/On.png"/>` +
	`<Image name="falseValue" value="file:^Imagenes/Off.png"/></IBooleanToSimple></ValueBinding>` +
	`</Picture></CanvasPane></ScrollPane></content></px>`

const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

// selftest renders a three-widget Px in a temporary shared/ root, then again against an empty
// root (missing images must not crash). Unlike the Python selftest it removes its temp dirs.
func selftest(s cli.Stdio) int {
	if err := runSelftest(); err != nil {
		fmt.Fprintf(s.Err, "px-render: selftest failed: %v\n", err)
		return cli.ExitNoMatch
	}
	fmt.Fprintln(s.Out, "selftest OK — canvas 400x300, 3 widgets embedded; missing-asset path safe")
	return cli.ExitFound
}

func runSelftest() error {
	d, err := os.MkdirTemp("", "px-render-selftest-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(d)
	png, _ := base64.StdEncoding.DecodeString(onePixelPNG)
	for _, dir := range []string{"Imagenes", "px", "empty"} {
		if err := os.MkdirAll(filepath.Join(d, dir), 0o755); err != nil {
			return err
		}
	}
	for _, nm := range []string{"On.png", "Off.png"} {
		if err := os.WriteFile(filepath.Join(d, "Imagenes", nm), png, 0o644); err != nil {
			return err
		}
	}
	fp := filepath.Join(d, "px", "test.px")
	if err := os.WriteFile(fp, []byte(selftestPx), 0o644); err != nil {
		return err
	}
	out, meta, err := render(Options{Px: fp, OnRatio: 0.7})
	if err != nil {
		return err
	}
	switch {
	case meta.canvasW != "400" || meta.canvasH != "300":
		return fmt.Errorf("canvas %sx%s, want 400x300", meta.canvasW, meta.canvasH)
	case meta.widgets != 3:
		return fmt.Errorf("%d widgets, want 3", meta.widgets)
	case !strings.Contains(out, "Cuarto 1") || !strings.Contains(out, "font-size:18px") || !strings.Contains(out, "Auto"):
		return fmt.Errorf("label or button text missing")
	case len(meta.missing) > 0:
		return fmt.Errorf("missing assets %v", meta.missing)
	case !strings.Contains(out, "data:image/png;base64"):
		return fmt.Errorf("boolean On image not embedded")
	}
	_, meta2, err := render(Options{Px: fp, Shared: filepath.Join(d, "empty"), OnRatio: 0.7})
	if err != nil {
		return err
	}
	if meta2.widgets != 2 || len(meta2.missing) == 0 {
		return fmt.Errorf("missing-asset render: %d widgets, %d missing", meta2.widgets, len(meta2.missing))
	}
	return nil
}
