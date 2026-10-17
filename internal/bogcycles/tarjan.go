package bogcycles

import "sort"

// sccs is _sccs(): Tarjan's strongly connected components, iterative, visiting nodes and
// successors in sorted order so the result order matches the Python original.
func sccs(nodes map[string]bool, edges map[string]map[string]bool) [][]string {
	succ := func(v string) []string {
		out := make([]string, 0, len(edges[v]))
		for w := range edges[v] {
			out = append(out, w)
		}
		sort.Strings(out)
		return out
	}
	type frame struct {
		v    string
		next []string
	}
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var out [][]string
	counter := 0
	visit := func(v string) frame {
		index[v], low[v] = counter, counter
		counter++
		stack = append(stack, v)
		onStack[v] = true
		return frame{v, succ(v)}
	}
	order := make([]string, 0, len(nodes))
	for v := range nodes {
		order = append(order, v)
	}
	sort.Strings(order)
	for _, v0 := range order {
		if _, seen := index[v0]; seen {
			continue
		}
		work := []frame{visit(v0)}
		for len(work) > 0 {
			top := &work[len(work)-1]
			v, advanced := top.v, false
			for len(top.next) > 0 {
				w := top.next[0]
				top.next = top.next[1:]
				if _, seen := index[w]; !seen {
					work = append(work, visit(w))
					advanced = true
					break
				}
				if onStack[w] {
					low[v] = min(low[v], index[w])
				}
			}
			if advanced {
				continue
			}
			work = work[:len(work)-1]
			if len(work) > 0 {
				p := work[len(work)-1].v
				low[p] = min(low[p], low[v])
			}
			if low[v] == index[v] {
				var comp []string
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					comp = append(comp, w)
					if w == v {
						break
					}
				}
				out = append(out, comp)
			}
		}
	}
	return out
}
