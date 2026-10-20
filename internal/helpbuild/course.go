package helpbuild

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/angeles725/niagara-go/internal/atomicfile"
	"github.com/angeles725/niagara-go/internal/helpbuild/pdftext"
	"github.com/angeles725/niagara-go/internal/helpbuild/pyfs"
	"github.com/angeles725/niagara-go/internal/helpbuild/registry"
	"github.com/angeles725/niagara-go/internal/pycompat"
	"github.com/angeles725/niagara-go/internal/pyjson"
)

// The developer course as a source kind (niagara_help_lib/course.py and
// tools/course_extractor.py): the registry's "course_dir" binds a course
// tree to one version; the builder copies its page text to course-text/
// and its example code to devguide-examples/course/, following the
// "course" globs of install_docs.json. gate() and print_gate() read the
// full-text index and arrive with S10.

// Course destinations and the pseudo-jar of the course exclude rules.
const (
	coursePseudoJar = "(course)"
	courseTextDir   = "course-text"
)

// courseCodeDir is devguide-examples/course.
var courseCodeDir = filepath.Join("devguide-examples", "course")

// courseDirMissingError is CourseDirMissing: the registry binds a
// course_dir that does not exist (a checkout without ../sources). The
// course is optional, so course_extractor skips it.
type courseDirMissingError struct{ Dir string }

func (e *courseDirMissingError) Error() string { return "course_dir not found: " + e.Dir }

// courseFile is one file of the course tree: its path relative to the
// tree ("/"-separated) and on disk.
type courseFile struct{ rel, path string }

// courseFiles is course._files: os.walk with sorted folders and files, a
// folder's files before its subfolders.
func courseFiles(src string) []courseFile {
	var out []courseFile
	prefix := map[string]string{src: ""}
	pyfs.Walk(src, func(dir string, dirs, files []string) []string {
		sort.Strings(dirs)
		sort.Strings(files)
		for _, f := range files {
			out = append(out, courseFile{prefix[dir] + f, pycompat.Join(dir, f)})
		}
		for _, d := range dirs {
			prefix[pycompat.Join(dir, d)] = prefix[dir] + d + "/"
		}
		return dirs
	})
	return out
}

// courseTextDest is course._text_dest: text/<n>.txt -> <n>.txt, other
// readmes keep their path (+ .txt when they are not .txt).
func courseTextDest(rel string) string {
	rel = strings.TrimPrefix(rel, "text/")
	if pdftext.HasSuffixFold(rel, ".txt") {
		return rel
	}
	return rel + ".txt"
}

// courseRow is one row of course.plan: Kind is "text", "code", "pdf" or ""
// (None); Dest is relative to the data root, "" when the file is not
// copied.
type courseRow struct{ Rel, Kind, Dest string }

// coursePlan is course.plan(src, rules): every file of the course tree with
// its kind and destination. rules must be non-nil (Default() is Python's
// rules=None).
func coursePlan(src string, rules *Rules) []courseRow {
	var out []courseRow
	for _, f := range courseFiles(src) {
		row := courseRow{Rel: f.rel, Kind: rules.CourseKind(f.rel)}
		switch row.Kind {
		case "text":
			row.Dest = filepath.Join(courseTextDir, courseTextDest(f.rel))
		case "code":
			row.Dest = filepath.Join(courseCodeDir, f.rel)
		}
		out = append(out, row)
	}
	return out
}

// removeCourseCopies is course.remove_copies: delete course-text/ and
// devguide-examples/course/ of baseDir (only those two trees) and return
// the ones that existed. Like shutil.rmtree, a symlink in their place is
// an error.
func removeCourseCopies(baseDir string) ([]string, error) {
	var removed []string
	for _, d := range []string{courseTextDir, courseCodeDir} {
		p := filepath.Join(baseDir, d)
		if !pycompat.IsDir(p) {
			continue
		}
		if st, err := os.Lstat(p); err == nil && st.Mode()&os.ModeSymlink != 0 {
			return removed, errRmtreeSymlink{p}
		}
		if err := os.RemoveAll(p); err != nil {
			return removed, err
		}
		removed = append(removed, d)
	}
	return removed, nil
}

// errRmtreeSymlink is the OSError shutil.rmtree raises on a symlink;
// Python 3.15 raises OSError(None, None, path), whose text this is.
type errRmtreeSymlink struct{ path string }

func (e errRmtreeSymlink) Error() string { return "[Errno None] None: " + pycompat.FSRepr(e.path) }

// courseCounts is build()'s {"text": n, "code": n, "course_dir": path}.
type courseCounts struct {
	Text, Code int
	CourseDir  string
}

// buildCourse is course.build: copy the bound course into baseDir,
// replacing earlier copies. Both trees are built in staging directories
// and swapped in at the end (atomicfile.RebuiltDir, devguide-examples/course/ first
// as Python's nested with-statement exits it first): a failed copy keeps
// the previous copies. It returns nil when no course is bound (copies
// left from an earlier binding are removed), a *CourseDirMissingError when
// the bound tree does not exist (nothing is touched), another error when a
// copy fails.
func buildCourse(baseDir string, rules *Rules) (*courseCounts, error) {
	src, err := registry.CourseDir(baseDir)
	if err != nil {
		return nil, err
	}
	if src == "" {
		_, err := removeCourseCopies(baseDir)
		return nil, err
	}
	if !pycompat.IsDir(src) {
		return nil, &courseDirMissingError{src}
	}
	counts := &courseCounts{CourseDir: src}
	err = atomicfile.RebuiltDir(filepath.Join(baseDir, courseTextDir), func(textTmp string) error {
		return atomicfile.RebuiltDir(filepath.Join(baseDir, courseCodeDir), func(codeTmp string) error {
			for _, row := range coursePlan(src, rules) {
				var top, tmp string
				switch row.Kind {
				case "text":
					top, tmp = courseTextDir, textTmp
				case "code":
					top, tmp = courseCodeDir, codeTmp
				default:
					continue
				}
				out := filepath.Join(tmp, strings.TrimPrefix(row.Dest, top+string(filepath.Separator)))
				if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
					return err
				}
				if err := pyfs.Copyfile(pycompat.Join(src, row.Rel), out); err != nil {
					return err
				}
				if row.Kind == "text" {
					counts.Text++
				} else {
					counts.Code++
				}
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

// toolOutput is what a builder tool prints: stdout and stderr lines and the
// exit status.
type toolOutput struct {
	Stdout, Stderr []string
	Status         int
}

// courseExtractor is tools/course_extractor.py main() for data root
// baseDir. err is what Python does not catch (a malformed registry: a
// traceback, exit 1); out holds no line then.
func courseExtractor(baseDir string, rules *Rules) (*toolOutput, error) {
	out := &toolOutput{}
	base, err := filepath.Abs(baseDir)
	if err != nil {
		return out, err
	}
	counts, err := buildCourse(base, rules)
	var missing *courseDirMissingError
	switch {
	case errors.As(err, &missing):
		out.Stdout = append(out.Stdout, fmt.Sprintf("course: %s; skipped (the course is optional: course-text/ "+
			"and devguide-examples/course/ keep their last copy)", missing))
	case err != nil && !pythonCrash(err):
		out.Stderr = append(out.Stderr, fmt.Sprintf("ERROR: course: cannot copy the course into %s: %s "+
			"(course-text/ and devguide-examples/course/ keep their last copy)", base, osErrorText(err)))
		out.Status = 2
	case err != nil:
		return out, err
	case counts == nil:
		out.Stdout = append(out.Stdout, "course: no course bound to this version; skipped")
	default:
		out.Stdout = append(out.Stdout, fmt.Sprintf("course: %d page text files -> course-text/, %d example code "+
			"files -> devguide-examples/course/ (from %s)", counts.Text, counts.Code, counts.CourseDir))
	}
	return out, nil
}

// pythonCrash reports an error course_extractor does not catch (it is not
// an OSError): a registry that is not JSON or not of the expected shape.
func pythonCrash(err error) bool {
	var le *pyfs.JSONLoadError
	var shape *registry.ShapeError
	var rec *pyjson.RecursionError
	return errors.As(err, &le) && !le.OS || errors.As(err, &shape) || errors.As(err, &rec)
}

// osErrorText is str(OSError) of a failure of build().
func osErrorText(err error) string {
	var le *pyfs.JSONLoadError
	if errors.As(err, &le) {
		return le.Msg
	}
	return pycompat.OSErrorText(err)
}
