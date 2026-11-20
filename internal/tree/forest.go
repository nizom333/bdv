// Пакет tree — чистая модель дерева задач: лес, сортировка, видимость, раскрытие.
package tree

import (
	"slices"
	"sort"

	"github.com/nizom333/bdv/internal/bd"
)

// Forest — лес задач: родитель вне выборки делает задачу корнем.
type Forest struct {
	byID     map[string]*bd.Issue
	children map[string][]*bd.Issue
	parent   map[string]string
	roots    []*bd.Issue
}

// statusRank — ранг статуса для сортировки.
func statusRank(s string) int {
	switch s {
	case "in_progress":
		return 0
	case "open":
		return 1
	case "blocked":
		return 2
	case "deferred":
		return 3
	case "closed":
		return 5
	}
	return 4
}

// less задаёт порядок соседей: статус, приоритет, created_at, ID.
func less(a, b *bd.Issue) bool {
	if ra, rb := statusRank(a.Status), statusRank(b.Status); ra != rb {
		return ra < rb
	}
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	if !a.CreatedAt.Equal(b.CreatedAt) {
		return a.CreatedAt.Before(b.CreatedAt)
	}
	return a.ID < b.ID
}

func sortIssues(s []*bd.Issue) {
	sort.SliceStable(s, func(i, j int) bool { return less(s[i], s[j]) })
}

// Build строит лес; дети и корни отсортированы.
func Build(issues []bd.Issue) *Forest {
	f := &Forest{
		byID:     make(map[string]*bd.Issue, len(issues)),
		children: map[string][]*bd.Issue{},
		parent:   map[string]string{},
	}
	for i := range issues {
		f.byID[issues[i].ID] = &issues[i]
	}
	// Родители, реально присутствующие в выборке (самоссылка — не родитель).
	par := map[string]string{}
	for i := range issues {
		iss := &issues[i]
		if p := iss.Parent; p != "" && p != iss.ID {
			if _, ok := f.byID[p]; ok {
				par[iss.ID] = p
			}
		}
	}
	breakCycles(par)
	for i := range issues {
		iss := &issues[i]
		if p, ok := par[iss.ID]; ok {
			f.children[p] = append(f.children[p], iss)
			f.parent[iss.ID] = p
		} else {
			f.roots = append(f.roots, iss)
		}
	}
	sortIssues(f.roots)
	for _, c := range f.children {
		sortIssues(c)
	}
	return f
}

// Issue возвращает задачу по ID.
func (f *Forest) Issue(id string) (*bd.Issue, bool) {
	i, ok := f.byID[id]
	return i, ok
}

// Count — сколько задач леса (без фильтров) удовлетворяют pred.
func (f *Forest) Count(pred func(*bd.Issue) bool) int {
	n := 0
	for _, iss := range f.byID {
		if pred(iss) {
			n++
		}
	}
	return n
}

// Len — число задач в лесу, без фильтров.
func (f *Forest) Len() int { return len(f.byID) }

// Children — все дети узла, отсортированы, без фильтров.
func (f *Forest) Children(id string) []*bd.Issue { return f.children[id] }

// ParentID — ID родителя или "", если его нет в лесу.
func (f *Forest) ParentID(id string) string { return f.parent[id] }

// DefaultExpanded: раскрыт, если есть потомок не closed и не event.
func (f *Forest) DefaultExpanded(id string) bool {
	for _, c := range f.children[id] {
		if (c.Status != "closed" && c.IssueType != "event") || f.DefaultExpanded(c.ID) {
			return true
		}
	}
	return false
}

// BlockedBy — ID открытых блокеров задачи id в порядке её зависимостей, без повторов:
// цели зависимостей blocks с IssueID == id, которые не закрыты. Блокер, которого нет
// в лесу (не попал в выборку bd list), считается открытым. nil — задача не заблокирована
// или её нет в лесу.
func (f *Forest) BlockedBy(id string) []string {
	is, ok := f.byID[id]
	if !ok {
		return nil
	}
	var out []string
	for _, d := range is.Dependencies {
		if d.Type != "blocks" || d.IssueID != id || slices.Contains(out, d.DependsOnID) {
			continue
		}
		if b, ok := f.byID[d.DependsOnID]; ok && b.Status == "closed" {
			continue
		}
		out = append(out, d.DependsOnID)
	}
	return out
}

// Waiting — задача id ждёт: она не закрыта и у неё есть хотя бы один открытый блокер
// (как в BlockedBy). Закрытой задаче ждать нечего. Единый признак для ⊘ в дереве,
// счётчика наверху, строки карточки, фильтра ReadyOnly и reveal. Без выделения памяти:
// останавливается на первом открытом блокере.
func (f *Forest) Waiting(id string) bool {
	is, ok := f.byID[id]
	if !ok || is.Status == "closed" {
		return false
	}
	for _, d := range is.Dependencies {
		if d.Type != "blocks" || d.IssueID != id {
			continue
		}
		if b, ok := f.byID[d.DependsOnID]; !ok || b.Status != "closed" {
			return true
		}
	}
	return false
}

// Ancestors — предки от корня к родителю, без самого id.
func Ancestors(f *Forest, id string) []string {
	var rev []string
	for p := f.parent[id]; p != ""; p = f.parent[p] {
		rev = append(rev, p)
	}
	out := make([]string, len(rev))
	for i, p := range rev {
		out[len(rev)-1-i] = p
	}
	return out
}

// breakCycles разрывает циклы parent: у члена цикла с наименьшим ID
// связь с родителем отбрасывается, и он становится корнем.
func breakCycles(par map[string]string) {
	done := map[string]bool{}
	for start := range par {
		if done[start] {
			continue
		}
		pos := map[string]int{}
		var path []string
		cur := start
		for {
			if done[cur] {
				break
			}
			if i, ok := pos[cur]; ok {
				min := path[i]
				for _, id := range path[i:] {
					if id < min {
						min = id
					}
				}
				delete(par, min)
				break
			}
			pos[cur] = len(path)
			path = append(path, cur)
			next, ok := par[cur]
			if !ok {
				break
			}
			cur = next
		}
		for _, id := range path {
			done[id] = true
		}
	}
}
