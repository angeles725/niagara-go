package helpbuild

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	CoursePseudoJar = "(course)"
	CourseTextDir   = "course-text"
)

// CourseCodeDir is devguide-examples/course.
var CourseCodeDir = filepath.Join("devguide-examples", "course")

// CourseDirMissingError is CourseDirMissing: the registry binds a
// course_dir that does not exist (a checkout without ../sources). The
// course is optional, so course_extractor skips it.
type CourseDirMissingError struct{ Dir string }

func (e *CourseDirMissingError) Error() string { return "course_dir not found: " + e.Dir }

// CourseDir is course.course_dir: the absolute course tree bound to the
// version whose data root is baseDir, "" when none is.
func CourseDir(baseDir string) (string, error) { return courseDir(baseDir) }

// courseFile is one file of the course tree: its path relative to the
// tree ("/"-separated) and on disk.
type courseFile struct{ rel, path string }

// courseFiles is course._files: os.walk with sorted folders and files, a
// folder's files before its subfolders.
func courseFiles(src string) []courseFile {
	var out []courseFile
	prefix := map[string]string{src: ""}
	pyWalk(src, func(dir string, dirs, files []string) []string {
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
	if hasSuffixFold(rel, ".txt") {
		return rel
	}
	return rel + ".txt"
}

// CourseRow is one row of course.plan: Kind is "text", "code", "pdf" or ""
// (None); Dest is relative to the data root, "" when the file is not
// copied.
type CourseRow struct{ Rel, Kind, Dest string }

// CoursePlan is course.plan(src, rules): every file of the course tree with
// its kind and destination. rules must be non-nil (Default() is Python's
// rules=None).
func CoursePlan(src string, rules *Rules) []CourseRow {
	var out []CourseRow
	for _, f := range courseFiles(src) {
		row := CourseRow{Rel: f.rel, Kind: rules.CourseKind(f.rel)}
		switch row.Kind {
		case "text":
			row.Dest = filepath.Join(CourseTextDir, courseTextDest(f.rel))
		case "code":
			row.Dest = filepath.Join(CourseCodeDir, f.rel)
		}
		out = append(out, row)
	}
	return out
}

// RemoveCourseCopies is course.remove_copies: delete course-text/ and
// devguide-examples/course/ of baseDir (only those two trees) and return
// the ones that existed. Like shutil.rmtree, a symlink in their place is
// an error.
func RemoveCourseCopies(baseDir string) ([]string, error) {
	var removed []string
	for _, d := range []string{CourseTextDir, CourseCodeDir} {
		p := filepath.Join(baseDir, d)
		if !isDir(p) {
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

// CourseCounts is build()'s {"text": n, "code": n, "course_dir": path}.
type CourseCounts struct {
	Text, Code int
	CourseDir  string
}

// BuildCourse is course.build: copy the bound course into baseDir,
// replacing earlier copies. Both trees are built in staging directories
// and swapped in at the end (RebuiltDir, devguide-examples/course/ first
// as Python's nested with-statement exits it first): a failed copy keeps
// the previous copies. It returns nil when no course is bound (copies
// left from an earlier binding are removed), a *CourseDirMissingError when
// the bound tree does not exist (nothing is touched), another error when a
// copy fails.
func BuildCourse(baseDir string, rules *Rules) (*CourseCounts, error) {
	src, err := courseDir(baseDir)
	if err != nil {
		return nil, err
	}
	if src == "" {
		_, err := RemoveCourseCopies(baseDir)
		return nil, err
	}
	if !isDir(src) {
		return nil, &CourseDirMissingError{src}
	}
	counts := &CourseCounts{CourseDir: src}
	err = RebuiltDir(filepath.Join(baseDir, CourseTextDir), func(textTmp string) error {
		return RebuiltDir(filepath.Join(baseDir, CourseCodeDir), func(codeTmp string) error {
			for _, row := range CoursePlan(src, rules) {
				var top, tmp string
				switch row.Kind {
				case "text":
					top, tmp = CourseTextDir, textTmp
				case "code":
					top, tmp = CourseCodeDir, codeTmp
				default:
					continue
				}
				out := filepath.Join(tmp, strings.TrimPrefix(row.Dest, top+string(filepath.Separator)))
				if err := os.MkdirAll(filepath.Dir(out), 0o777); err != nil {
					return err
				}
				if err := pyCopyfile(pycompat.Join(src, row.Rel), out); err != nil {
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

// ToolOutput is what a builder tool prints: stdout and stderr lines and the
// exit status.
type ToolOutput struct {
	Stdout, Stderr []string
	Status         int
}

// CourseExtractor is tools/course_extractor.py main() for data root
// baseDir. err is what Python does not catch (a malformed registry: a
// traceback, exit 1); out holds no line then.
func CourseExtractor(baseDir string, rules *Rules) (*ToolOutput, error) {
	out := &ToolOutput{}
	base, err := filepath.Abs(baseDir)
	if err != nil {
		return out, err
	}
	counts, err := BuildCourse(base, rules)
	var missing *CourseDirMissingError
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
	var le *JSONLoadError
	var shape *registryShapeError
	var rec *pyjson.RecursionError
	return errors.As(err, &le) && !le.OS || errors.As(err, &shape) || errors.As(err, &rec)
}

// osErrorText is str(OSError) of a failure of build().
func osErrorText(err error) string {
	var le *JSONLoadError
	if errors.As(err, &le) {
		return le.Msg
	}
	return pycompat.OSErrorText(err)
}
