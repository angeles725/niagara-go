package key

import "strings"

// GroupKeys resolves a --lab / text spelling to group keys: 3, 03 and L3, B and LB, A and LA,
// RA (RA1, RA2 and RAX together) or a key in any case.
func (c *Corpus) GroupKeys(name string) []string {
	n := strings.ToUpper(strings.TrimSpace(name))
	switch {
	case n == "B" || n == "A":
		n = "L" + n
	case n != "" && strings.Trim(n, "0123456789") == "":
		n = "L" + strings.TrimLeft(n, "0")
	case len(n) > 1 && n[0] == 'L' && strings.Trim(n[1:], "0123456789") == "":
		n = "L" + strings.TrimLeft(n[1:], "0") // L03 is L3
	case n == "RA":
		var keys []string
		for _, g := range c.Groups {
			if strings.HasPrefix(g.Key, "RA") {
				keys = append(keys, g.Key)
			}
		}
		return keys
	}
	if c.Group(n) == nil {
		return nil
	}
	return []string{n}
}

// GroupList is every group key, comma-separated, in source order.
func (c *Corpus) GroupList() string {
	keys := make([]string, len(c.Groups))
	for i, g := range c.Groups {
		keys[i] = g.Key
	}
	return strings.Join(keys, ", ")
}
