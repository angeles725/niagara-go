package check

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/angeles725/niagara-go/internal/bog"
)

// The listing kind: a file the station wrote (Glob under the station directory) that lists its
// components of Type, one line each (RA2.R5b: Review.txt, "name: value {status} @ priority"). Match is
// a Go regexp with a (?P<name>...) group; the lines it matches must name exactly the components of
// Type (Within and Under filter them as for type), none missing, none extra, none twice. With Slot
// set, Match also has (?P<value>...) and (?P<priority>...) groups: every line must be at priority
// Want and its value must equal the component's effective Slot (read as for slot, Default when not
// stored), numbers compared as numbers (a decimal comma is a point). The file is compared with the
// bog as both were last saved. Two selected components with the same short name make the check
// SKIP (ambiguous), since a line names a component by its short name only.

func (s *Station) checkListing(ck *Check) (string, string) {
	if ck.dotted() && !s.nested {
		return Skip, ck.Slot + " is a nested slot: the station was read without nested values"
	}
	re, err := regexp.Compile(ck.Match)
	if err != nil {
		return Skip, fmt.Sprintf("bad pattern %s: %v", ck.Match, err)
	}
	ni, vi, pi := re.SubexpIndex("name"), re.SubexpIndex("value"), re.SubexpIndex("priority")
	if ni < 0 || (ck.Slot != "" && (vi < 0 || pi < 0)) {
		return Skip, "bad pattern " + ck.Match + ": no name group, or a Slot without value and priority groups"
	}
	_, files, skip := s.rootFiles(ck)
	if skip != "" {
		return Skip, skip
	}
	if len(files) != 1 {
		return Fail, fmt.Sprintf("found %d files %s, want one", len(files), ck.Glob)
	}
	lines, err := fileLines(s.rootPath(ck, files[0]))
	if err != nil {
		return Skip, fmt.Sprintf("cannot read %s: %v", files[0], err)
	}
	cs, _, where := s.selected(ck)
	// The file names components by their short name: two selected components sharing one cannot be
	// told apart, so the check cannot decide (never a PASS one line would earn for both).
	byName := map[string]*bog.Comp{}
	for _, c := range cs {
		if prev := byName[c.Name]; prev != nil {
			var paths []string
			for _, o := range cs {
				if o.Name == c.Name {
					paths = append(paths, o.Path)
				}
			}
			return Skip, fmt.Sprintf("ambiguous: %s share the name %s", strings.Join(paths, ", "), c.Name)
		}
		byName[c.Name] = c
	}
	listed := map[string]bool{}
	var extra, twice, bad []string
	for i, l := range lines {
		m := re.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		name := m[ni]
		c := byName[name]
		switch {
		case c == nil:
			extra = append(extra, name)
		case listed[name]:
			twice = append(twice, name)
		case ck.Slot != "":
			if why := listedValue(ck, c, m[vi], m[pi]); why != "" && len(bad) == 0 {
				bad = append(bad, fmt.Sprintf("%s:%d %s", files[0], i+1, why))
			}
		}
		listed[name] = true
	}
	var missing []string
	for _, c := range cs {
		if !listed[c.Name] {
			missing = append(missing, c.Name)
		}
	}
	sort.Strings(missing)
	ev := fmt.Sprintf("%s lists %d of %d %s%s", files[0], len(cs)-len(missing), len(cs), ck.Type, where)
	if ck.Slot != "" {
		ev += fmt.Sprintf(", each at %s with its %s", ck.Want, ck.Slot)
	}
	var problems []string
	for _, p := range []struct {
		label string
		names []string
	}{{"missing", missing}, {"not a " + ck.Type, extra}, {"listed twice", twice}} {
		if len(p.names) > 0 {
			problems = append(problems, p.label+": "+strings.Join(p.names, ", "))
		}
	}
	problems = append(problems, bad...)
	if len(cs) == 0 {
		problems = append(problems, "no "+ck.Type+" component")
	}
	if len(problems) > 0 {
		return Fail, ev + "; " + strings.Join(problems, "; ")
	}
	return Pass, ev
}

// listedValue checks one line's value and priority against the component; "" when they agree.
func listedValue(ck *Check, c *bog.Comp, value, priority string) string {
	if priority != ck.Want {
		return fmt.Sprintf("%s is at %s, want %s", c.Name, priority, ck.Want)
	}
	want, stored := effective(c, ck.Slot, ck.Default)
	got, err1 := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	exp, err2 := strconv.ParseFloat(want, 64)
	if err1 != nil || err2 != nil || got != exp {
		return fmt.Sprintf("%s = %s, want %s", c.Name, value, describe(c, ck.Slot, want, stored))
	}
	return ""
}
