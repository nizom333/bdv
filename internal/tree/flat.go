package tree

import "github.com/nizom333/bdv/internal/bd"

// Flat — плоский список задач леса для вкладок Specs и Decisions: задачи, для которых
// keep истинно, проходящие фильтры c/e (Passes) и запрос o.Query, в порядке соседей
// дерева (статус, приоритет, created_at, ID). Группы, ReadyOnly и раскрытие здесь
// не применяются: строки без глубины, детей и контекста.
func Flat(f *Forest, o Options, keep func(*bd.Issue) bool) []Row {
	q := normQuery(o)
	var list []*bd.Issue
	for _, is := range f.byID {
		if keep(is) && Passes(is, o) && matchesQuery(is, q) {
			list = append(list, is)
		}
	}
	sortIssues(list)
	rows := make([]Row, len(list))
	for i, is := range list {
		rows[i] = Row{Issue: is, Group: is.Priority}
	}
	return rows
}

// Tickets — тикеты спеки id: прямые дети без служебных событий (event), отсортированы.
// Закрытые входят: по ним считается прогресс.
func (f *Forest) Tickets(id string) []*bd.Issue {
	var out []*bd.Issue
	for _, c := range f.children[id] {
		if c.IssueType != "event" {
			out = append(out, c)
		}
	}
	return out
}

// Progress — прогресс спеки id по её тикетам (Tickets): сколько закрыто и сколько всего.
func (f *Forest) Progress(id string) (done, total int) {
	for _, c := range f.Tickets(id) {
		total++
		if c.Status == "closed" {
			done++
		}
	}
	return done, total
}
