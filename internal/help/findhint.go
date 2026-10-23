package help

import (
	"fmt"
	"regexp"
	"strings"
)

// module.xml is not in the full-text index (module-index.json is structured,
// read by module and type), so a find miss on a name that looks like a module
// or a type says where to look instead.
var (
	moduleNameLike = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-(rt|ux|wb|se|doc)$`)
	typeSpecLike   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*:[A-Za-z][A-Za-z0-9_]*$`)
	typeNameLike   = regexp.MustCompile(`^[A-Z][a-z0-9]+(?:[A-Z][A-Za-z0-9]*)+$`)
)

// moduleXMLHint is the extra miss line for query, "" when it looks like
// neither a module name nor a type.
func moduleXMLHint(query string) string {
	q := strings.TrimSpace(query)
	var cmds []string
	switch {
	case moduleNameLike.MatchString(q):
		cmds = []string{"module"}
	case typeSpecLike.MatchString(q):
		cmds = []string{"type"}
	case typeNameLike.MatchString(q):
		cmds = []string{"type", "module"}
	default:
		return ""
	}
	for i, c := range cmds {
		cmds[i] = fmt.Sprintf("nh %s '%s'", c, q)
	}
	return "  module.xml lookups: " + strings.Join(cmds, ", ")
}
