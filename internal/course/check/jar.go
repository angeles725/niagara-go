package check

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The jar kinds (jar, jar-entry, jar-text, jar-signed): they read <install>/modules/<part>.jar.

// globRe turns a jar-entry glob into a regexp: ** crosses '/', * and ? do not.
func globRe(glob string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch {
		case strings.HasPrefix(glob[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(glob[i:], "**"):
			b.WriteString(".*")
			i++
		case glob[i] == '*':
			b.WriteString("[^/]*")
		case glob[i] == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(glob[i : i+1]))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

func (s *Station) jar(name string) (*zip.ReadCloser, error) {
	if j, ok := s.jars[name]; ok {
		return j.zr, j.err
	}
	zr, err := zip.OpenReader(filepath.Join(s.install, "modules", name+".jar"))
	s.jars[name] = &jarFile{zr, err}
	return zr, err
}

func (s *Station) checkJar(ck *Check) (string, string) {
	rel := "modules/" + ck.Jar + ".jar"
	if s.install == "" {
		return Skip, "no --install DIR: " + rel + " not checked"
	}
	zr, err := s.jar(ck.Jar)
	if os.IsNotExist(err) {
		return Fail, "missing " + rel + " under " + s.install
	}
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s: %v", rel, err)
	}
	switch ck.Kind {
	case kindJar:
		return Pass, fmt.Sprintf("%s (%s)", rel, entryCount(len(zr.File)))
	case kindJarSigned:
		sf, block := entries(zr, globRe("META-INF/*.SF")), entries(zr, regexp.MustCompile(`^META-INF/[^/]+\.(?:RSA|DSA|EC)$`))
		if len(sf) == 0 || len(block) == 0 {
			return Fail, fmt.Sprintf("%s: %d signature files (.SF), %d signature blocks (.RSA/.DSA/.EC)", rel, len(sf), len(block))
		}
		return Pass, fmt.Sprintf("%s: %s, %s", rel, sf[0].Name, block[0].Name)
	case kindJarEntry:
		found := entries(zr, globRe(ck.Glob))
		ok := countOK(ck.Op, len(found), ck.N)
		var ev string
		if len(found) == 1 {
			ev = fmt.Sprintf("%s: 1 entry matches %s", rel, ck.Glob)
		} else {
			ev = fmt.Sprintf("%s: %d entries match %s", rel, len(found), ck.Glob)
		}
		if !ok {
			ev += fmt.Sprintf(", want %s (searched %s)", want(ck.Op, ck.N), entryCount(len(zr.File)))
		}
		return verdict(ok), ev
	default: // kindJarText
		return jarText(zr, rel, ck)
	}
}

func entries(zr *zip.ReadCloser, re *regexp.Regexp) []*zip.File {
	var out []*zip.File
	for _, f := range zr.File {
		if re.MatchString(f.Name) {
			out = append(out, f)
		}
	}
	return out
}

func jarText(zr *zip.ReadCloser, rel string, ck *Check) (string, string) {
	found := entries(zr, globRe(ck.Glob))
	if len(found) == 0 {
		return Fail, fmt.Sprintf("%s: no entry %s (searched %s)", rel, ck.Glob, entryCount(len(zr.File)))
	}
	rc, err := found[0].Open()
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s!%s: %v", rel, found[0].Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 16<<20))
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s!%s: %v", rel, found[0].Name, err)
	}
	var missing []string
	for _, want := range ck.Contains {
		if !strings.Contains(string(data), want) {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		return Fail, fmt.Sprintf("%s!%s lacks %s (%d of %d found)", rel, found[0].Name,
			strings.Join(missing, ", "), len(ck.Contains)-len(missing), len(ck.Contains))
	}
	return Pass, fmt.Sprintf("%s!%s has %s", rel, found[0].Name, strings.Join(ck.Contains, ", "))
}

func entryCount(n int) string {
	if n == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", n)
}
