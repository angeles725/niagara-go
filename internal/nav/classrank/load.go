package classrank

import "github.com/angeles725/niagara-go/internal/nav/registry"

// Load is load_ranker: the Ranker of the base directory's indexes, or nil
// after an exit-2 error.
func Load(env *registry.Env) *Ranker {
	classes, importedBy, meta, ok := readInputs(env)
	if !ok {
		return nil
	}
	r, err := newRanker(build(classes, importedBy), meta)
	if err != nil { // a freshly built state is always well formed
		panic(err)
	}
	return r
}
