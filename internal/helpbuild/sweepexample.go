package helpbuild

import (
	"errors"
	"strings"

	"github.com/angeles725/niagara-go/internal/help/fts"
	"github.com/angeles725/niagara-go/internal/helpbuild/zipfile"
)

// sweepJar is the jar the sweep reads entries from (lexicons, module
// examples): the entries of one jar are classified one after the other, so
// one jar stays open.
type sweepJar struct {
	path string
	z    *zipfile.File
}

func (s *sweeper) closeJar() {
	if s.jar != nil {
		_ = s.jar.z.Close()
		s.jar = nil
	}
}

// errEntryNotFound is a listed entry the jar no longer holds.
var errEntryNotFound = errors.New("entry not found in the jar")

// jarEntry reads the first entry named name of the jar at path jar (the
// listing cache has no contents).
func (s *sweeper) jarEntry(jar, name string) ([]byte, error) {
	if s.jar == nil || s.jar.path != jar {
		s.closeJar()
		z, err := zipfile.Open(jar)
		if err != nil {
			return nil, err
		}
		s.jar = &sweepJar{jar, z}
	}
	z := s.jar.z
	for i := range z.Infos {
		if z.Infos[i].Name == name {
			return z.ReadEntry(&z.Infos[i], true)
		}
	}
	return nil, errEntryNotFound
}

// example is the sweep verdict of a template of the installed jar at
// install path rel (isModuleExampleEntry): excluded when vendored; indexed
// when its module-examples/ page is in the full-text index; else the sweep
// reads it with moduleExamplePage's rule: excluded when it has no text (the
// extractor's warning names it), UNCLASSIFIED when it has.
func (s *sweeper) example(rel, entry string) (verdict, error) {
	if fts.IsVendoredWeb(entry) {
		return verdict{statusExcluded, "template of a vendored web library (node_modules, rc/vendor, rc/ext, ext)",
			"example-vendored"}, nil
	}
	jarName := pyBasename(rel)
	key := moduleExamplesDir + "/" + strings.TrimSuffix(jarName, ".jar") + "/" + entry
	docs, err := s.indexed.fulltextDocs()
	if err != nil {
		return verdict{}, err
	}
	if docs[key] {
		return verdict{statusIndexed, key, moduleExamplesDir}, nil
	}
	data, err := s.jarEntry(s.installedPath(rel), entry)
	if err != nil {
		return verdict{statusUnclassified, "module example not readable: " + err.Error(), ""}, nil
	}
	if _, why := moduleExamplePage(jarName, entry, data); why != "" {
		return verdict{statusExcluded, "module example without text: " + why + " (" + moduleExamplesScript + " warns)",
			"example-not-text"}, nil
	}
	return verdict{statusUnclassified, "module example is not indexed (" + key + "): run rebuild-all, then find", ""}, nil
}

// installedPath is the path of the installed jar at install path rel.
func (s *sweeper) installedPath(rel string) string {
	for _, j := range s.installed {
		if j.Rel == rel {
			return j.Path
		}
	}
	return rel
}
