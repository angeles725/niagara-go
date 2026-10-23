package helpbuild

import "github.com/angeles725/niagara-go/internal/helpbuild/zipfile"

// jarMember is one member of a jar and its bytes, or why they cannot be read.
type jarMember struct {
	name string
	data []byte
	err  error
}

// readMembers reads the members of jar whose name match accepts, in the order
// they first appear; a repeated name reads its last copy, as ZipFile.open
// does. A member that cannot be inflated carries its error; err is for a jar
// that cannot be opened.
func readMembers(jar string, match func(name string) bool) ([]jarMember, error) {
	z, err := zipfile.Open(jar)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	last := map[string]int{}
	var order []string
	for i := range z.Infos {
		name := z.Infos[i].Name
		if !match(name) {
			continue
		}
		if _, seen := last[name]; !seen {
			order = append(order, name)
		}
		last[name] = i
	}
	out := make([]jarMember, len(order))
	for i, name := range order {
		data, err := z.ReadEntry(&z.Infos[last[name]], true)
		out[i] = jarMember{name, data, err}
	}
	return out, nil
}
