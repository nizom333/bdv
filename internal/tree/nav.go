package tree

// Nearest ищет ближайший видимый узел вместо исчезнувшего id: соседи после,
// соседи до (в обратном порядке), затем предки от ближайшего.
func Nearest(f *Forest, id string, visible func(id string) bool) string {
	sibs := f.roots
	if p := f.parent[id]; p != "" {
		sibs = f.children[p]
	}
	idx := -1
	for i, s := range sibs {
		if s.ID == id {
			idx = i
			break
		}
	}
	if idx >= 0 {
		for i := idx + 1; i < len(sibs); i++ {
			if visible(sibs[i].ID) {
				return sibs[i].ID
			}
		}
		for i := idx - 1; i >= 0; i-- {
			if visible(sibs[i].ID) {
				return sibs[i].ID
			}
		}
	}
	for p := f.parent[id]; p != ""; p = f.parent[p] {
		if visible(p) {
			return p
		}
	}
	return ""
}
