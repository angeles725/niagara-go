package check

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/course/key"
)

// File roots: the directory a files or file-text row reads (Check.Root). The station directory is
// the default; a row over the Niagara install (defaults/system.properties) or the participant's lab
// sources (the dev home: its Gradle files, module-include.xml, and ../certificates beside it, where
// the course's `--profile-path ..\certificates\code_signing.xml` puts the signing profile) names its
// root. Install and lab-source rows are offline evidence: no station holds those files.
const (
	rootStation = ""        // the station directory ({station} = the station name)
	rootInstall = "install" // --install, COURSE_NIAGARA_INSTALL
	rootLabsrc  = "labsrc"  // --labsrc, COURSE_LAB_SOURCES
	// rootSolutions: --solutions, COURSE_SOLUTIONS: the participant's own module sources as the
	// niagara-research corpus keeps them (sources/n4-dev-course/solutions: review/, cert-exam/),
	// read by the annotation rows of the review assignments.
	rootSolutions = "solutions"
)

// checkEvidence is the evidence kind of a verified marker one check gives: the kind's (evidenceOf),
// except that a files or file-text row over another root than the station is offline.
func checkEvidence(ck *Check) string {
	if ck.Root != rootStation && (ck.Kind == kindFiles || ck.Kind == kindFileText) {
		return key.EvidenceOffline
	}
	return evidenceOf[ck.Kind]
}

// rootDir is the directory the row reads, or the SKIP reason when the run was not given it or it
// is not a readable directory (a configuration error, as for --source).
func (s *Station) rootDir(ck *Check) (dir, skip string) {
	switch ck.Root {
	case rootStation:
		return s.dir, ""
	case rootInstall:
		dir = s.install
	case rootLabsrc:
		dir = s.labsrc
	case rootSolutions:
		dir = s.solutions
	default:
		return "", "unknown root " + ck.Root
	}
	if dir == "" {
		return "", fmt.Sprintf("no --%s DIR: %s not checked", ck.Root, ck.Glob)
	}
	if fi, err := os.Stat(dir); err != nil {
		return "", fmt.Sprintf("cannot read %s: %v", dir, err)
	} else if !fi.IsDir() {
		return "", fmt.Sprintf("cannot read %s: not a directory", dir)
	}
	return dir, ""
}

// expandBraces expands every {a,b} group of a glob into its alternatives, in order ("{x,y}/{b,c}"
// is x/b, x/c, y/b, y/c); an unclosed brace is literal. filepath.Glob has no braces.
func expandBraces(glob string) []string {
	i := strings.IndexByte(glob, '{')
	if i < 0 {
		return []string{glob}
	}
	end := strings.IndexByte(glob[i:], '}')
	if end < 0 {
		return []string{glob}
	}
	end += i
	var out []string
	for _, alt := range strings.Split(glob[i+1:end], ",") {
		for _, rest := range expandBraces(glob[end+1:]) {
			out = append(out, glob[:i]+alt+rest)
		}
	}
	return out
}

// rootFiles lists what a Glob matches under the row's root, sorted, as "/"-separated paths relative
// to it ("../certificates/x" beside the lab sources): regular files, or directories for an
// alternative ending in "/" (printed with the "/"). pattern is the Glob as printed ({station}
// replaced for the station root); skip is set when the root cannot be read.
func (s *Station) rootFiles(ck *Check) (pattern string, files []string, skip string) {
	dir, skip := s.rootDir(ck)
	if skip != "" {
		return ck.Glob, nil, skip
	}
	pattern = ck.Glob
	if ck.Root == rootStation {
		pattern = strings.ReplaceAll(pattern, "{station}", globEscape(s.Name))
	}
	seen := map[string]bool{}
	for _, alt := range expandBraces(pattern) {
		wantDir := strings.HasSuffix(alt, "/")
		matches, err := filepath.Glob(filepath.Join(globEscape(dir), filepath.FromSlash(strings.TrimSuffix(alt, "/"))))
		if err != nil {
			return pattern, nil, fmt.Sprintf("bad pattern %s: %v", pattern, err)
		}
		for _, m := range matches {
			st, err := os.Stat(m)
			if err != nil || (wantDir && !st.IsDir()) || (!wantDir && !st.Mode().IsRegular()) {
				continue
			}
			rel, err := filepath.Rel(dir, m)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if wantDir {
				rel += "/"
			}
			if !seen[rel] {
				seen[rel] = true
				files = append(files, rel)
			}
		}
	}
	sort.Strings(files)
	return pattern, files, ""
}

// rootPath is the path of a rootFiles entry on disk.
func (s *Station) rootPath(ck *Check, rel string) string {
	dir, _ := s.rootDir(ck)
	return filepath.Join(dir, filepath.FromSlash(rel))
}

// NeedsSolutions reports whether a check reads the corpus solutions, so a summary names it.
func NeedsSolutions(checks []*Check) bool {
	return slices.ContainsFunc(checks, func(ck *Check) bool { return ck.Root == rootSolutions })
}
