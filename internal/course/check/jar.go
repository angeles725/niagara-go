package check

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/angeles725/niagara-go/internal/modjar"
)

// The jar kinds (jar, jar-entry, jar-text, jar-signed): they read <install>/modules/<part>.jar.

// globRe turns a jar-entry glob into a regexp: ** crosses '/', * and ? do not, {a,b} is one of
// the literal names a or b (so a check can list exact entries).
func globRe(glob string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(glob); i++ {
		switch {
		case glob[i] == '{' && strings.IndexByte(glob[i:], '}') > 0:
			end := i + strings.IndexByte(glob[i:], '}')
			alts := strings.Split(glob[i+1:end], ",")
			for j, a := range alts {
				alts[j] = regexp.QuoteMeta(a)
			}
			b.WriteString("(?:" + strings.Join(alts, "|") + ")")
			i = end
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

func (s *Station) jar(name string) (*modjar.Jar, error) {
	if j, ok := s.jars[name]; ok {
		return j.jar, j.err
	}
	j, err := modjar.Open(filepath.Join(s.install, "modules", name+".jar"))
	s.jars[name] = &jarFile{j, err}
	return j, err
}

func (s *Station) checkJar(ck *Check) (string, string) {
	rel := "modules/" + ck.Jar + ".jar"
	if s.install == "" {
		return Skip, "no --install DIR: " + rel + " not checked"
	}
	j, err := s.jar(ck.Jar)
	if os.IsNotExist(err) {
		return Fail, "missing " + rel + " under " + s.install
	}
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s: %v", rel, err)
	}
	switch ck.Kind {
	case kindJar:
		return Pass, fmt.Sprintf("%s (%s)", rel, entryCount(len(j.Names())))
	case kindJarSigned:
		sf, block := entries(j, globRe("META-INF/*.SF")), entries(j, regexp.MustCompile(`^META-INF/[^/]+\.(?:RSA|DSA|EC)$`))
		if len(sf) == 0 || len(block) == 0 {
			return Fail, fmt.Sprintf("%s: %d signature files (.SF), %d signature blocks (.RSA/.DSA/.EC)", rel, len(sf), len(block))
		}
		return Pass, fmt.Sprintf("%s: %s, %s", rel, j.Names()[sf[0]], j.Names()[block[0]])
	case kindJarEntry:
		found := entries(j, globRe(ck.Glob))
		ok := countOK(ck.Op, len(found), ck.N)
		var ev string
		if len(found) == 1 {
			ev = fmt.Sprintf("%s: 1 entry matches %s", rel, ck.Glob)
		} else {
			ev = fmt.Sprintf("%s: %d entries match %s", rel, len(found), ck.Glob)
		}
		if !ok {
			ev += fmt.Sprintf(", want %s (searched %s)", want(ck.Op, ck.N), entryCount(len(j.Names())))
		}
		return verdict(ok), ev
	default: // kindJarText
		return jarText(j, rel, ck)
	}
}

// entries is the index of every entry of j whose name re matches, in archive order.
func entries(j *modjar.Jar, re *regexp.Regexp) []int {
	var out []int
	for i, n := range j.Names() {
		if re.MatchString(n) {
			out = append(out, i)
		}
	}
	return out
}

func jarText(j *modjar.Jar, rel string, ck *Check) (string, string) {
	found := entries(j, globRe(ck.Glob))
	if len(found) == 0 {
		return Fail, fmt.Sprintf("%s: no entry %s (searched %s)", rel, ck.Glob, entryCount(len(j.Names())))
	}
	name := j.Names()[found[0]]
	data, err := j.Read(found[0], 16<<20)
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s!%s: %v", rel, name, err)
	}
	var missing []string
	for _, want := range ck.Contains {
		if !strings.Contains(string(data), want) {
			missing = append(missing, want)
		}
	}
	if len(missing) > 0 {
		return Fail, fmt.Sprintf("%s!%s lacks %s (%d of %d found)", rel, name,
			strings.Join(missing, ", "), len(ck.Contains)-len(missing), len(ck.Contains))
	}
	return Pass, fmt.Sprintf("%s!%s has %s", rel, name, strings.Join(ck.Contains, ", "))
}

func entryCount(n int) string {
	if n == 1 {
		return "1 entry"
	}
	return fmt.Sprintf("%d entries", n)
}
