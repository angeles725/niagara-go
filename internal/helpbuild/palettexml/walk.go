package palettexml

import "slices"

// Counts are the folders (the root excluded), entries and properties of a palette.
type Counts struct{ Folders, Entries, Properties int }

// Counts counts p.
func (p *Palette) Counts() Counts {
	var c Counts
	p.Walk(func(_ []string, n *Node) {
		if n.Folder {
			c.Folders++
		} else {
			c.Entries++
			c.Properties += len(n.Properties)
		}
	})
	return c
}

// Walk calls fn for every folder and entry below the root, depth first, with
// the names from the root's first child down to the node.
func (p *Palette) Walk(fn func(path []string, n *Node)) {
	var walk func(path []string, n *Node)
	walk = func(path []string, n *Node) {
		for _, c := range n.Children {
			sub := append(slices.Clip(path), c.Name)
			fn(sub, c)
			walk(sub, c)
		}
	}
	walk(nil, p.Root)
}
