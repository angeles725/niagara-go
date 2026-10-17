package pxrender

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// findShared locates the station shared/ root that image ords resolve against: the first of the
// .px directory and its 7 ancestors that holds Imagenes/, else the parent of the .px directory.
func findShared(px, override string) string {
	if override != "" {
		return override
	}
	abs, err := filepath.Abs(px)
	if err != nil {
		abs = px
	}
	d := filepath.Dir(abs)
	for i := 0; i < 8; i++ {
		if st, err := os.Stat(filepath.Join(d, "Imagenes")); err == nil && st.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return filepath.Dir(filepath.Dir(abs))
}

var mimeTypes = map[string]string{"svg": "image/svg+xml", "jpg": "image/jpeg", "jpeg": "image/jpeg",
	"gif": "image/gif", "png": "image/png"}

// assets embeds referenced images as data: URIs and remembers the refs that did not resolve.
type assets struct {
	shared, organized string
	cache             map[string]string
	missing           map[string]bool
	notImages         []string // refs that resolved to a directory, reported and skipped
}

func newAssets(shared, organized string) *assets {
	return &assets{shared: shared, organized: organized, cache: map[string]string{}, missing: map[string]bool{}}
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// moduleCandidates: the station's bundled copy under shared/px/<mod>/<rel>, then the corpus
// organized/<mod>/<mod>-{wb,rt,ux}/extracted/<rel>, then any artifact of that module.
func (a *assets) moduleCandidates(mod, rel string) []string {
	c := []string{pycompat.PosixJoin(a.shared, "px", mod, rel)}
	if a.organized == "" {
		return c
	}
	for _, kind := range []string{"wb", "rt", "ux"} {
		c = append(c, pycompat.PosixJoin(a.organized, mod, mod+"-"+kind, "extracted", rel))
	}
	matches, _ := filepath.Glob(pycompat.PosixJoin(a.organized, mod, "*", "extracted", rel))
	sort.Strings(matches)
	// filepath.Glob returns cleaned paths, so compare against the cleaned module directory.
	prefix := filepath.Clean(pycompat.PosixJoin(a.organized, mod)) + string(filepath.Separator)
	for _, m := range matches {
		if rest, ok := strings.CutPrefix(filepath.Clean(m), prefix); !ok || !strings.HasPrefix(rest, ".") {
			c = append(c, m) // like Python glob, '*' does not match a hidden artifact directory
		}
	}
	return c
}

func (a *assets) resolve(ref string) string {
	switch {
	case ref == "":
		return ""
	case strings.HasPrefix(ref, "file:^"):
		if p := pycompat.PosixJoin(a.shared, ref[len("file:^"):]); exists(p) {
			return p
		}
	case strings.HasPrefix(ref, "module://"):
		mod, rel, ok := strings.Cut(ref[len("module://"):], "/")
		if !ok {
			return ""
		}
		for _, p := range a.moduleCandidates(mod, rel) {
			if exists(p) {
				return p
			}
		}
	}
	return ""
}

// dataURI returns the embedded image of ref, "" when it does not resolve (recorded as missing).
// A ref that resolves to a directory is recorded as missing and reported in notImages; Python
// aborted the whole render with IsADirectoryError there.
func (a *assets) dataURI(ref string) (string, error) {
	p := a.resolve(ref)
	if p == "" {
		if ref != "" {
			a.missing[ref] = true
		}
		return "", nil
	}
	if uri, ok := a.cache[p]; ok {
		return uri, nil
	}
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		if !a.missing[ref] {
			a.missing[ref] = true
			a.notImages = append(a.notImages, ref+": "+p+" is a directory, not an image (skipped)")
		}
		return "", nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(p[strings.LastIndex(p, ".")+1:])
	mime, ok := mimeTypes[ext]
	if !ok {
		mime = "image/png"
	}
	uri := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b)
	a.cache[p] = uri
	return uri, nil
}

func (a *assets) missingSorted() []string {
	out := make([]string, 0, len(a.missing))
	for m := range a.missing {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
