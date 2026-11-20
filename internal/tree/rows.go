package tree

import (
	"sort"
	"strconv"
	"strings"

	"github.com/nizom333/bdv/internal/bd"
)

// Options — фильтры отображения. Query — подстрока поиска по ID и заголовку (без учёта регистра).
// GroupByPriority собирает корни под заголовки групп по приоритету. ReadyOnly оставляет
// только задачи, которые не ждут (Forest.Waiting); предки таких задач видны как
// контекст, как при поиске.
type Options struct {
	ShowClosed, ShowEvents bool
	Query                  string
	GroupByPriority        bool
	ReadyOnly              bool
}

// Expansion — ручные действия пользователя: id → раскрыт. Группы хранятся по GroupKey.
type Expansion map[string]bool

// Expanded: ручное значение, иначе значение по умолчанию.
func (e Expansion) Expanded(f *Forest, id string) bool {
	if v, ok := e[id]; ok {
		return v
	}
	return f.DefaultExpanded(id)
}

// GroupExpanded: ручное значение группы p, по умолчанию раскрыта.
func (e Expansion) GroupExpanded(p int) bool {
	if v, ok := e[GroupKey(p)]; ok {
		return v
	}
	return true
}

// GroupKey — ключ группы приоритета p в Expansion и ID строки-заголовка: "#P<p>".
func GroupKey(p int) string { return "#P" + strconv.Itoa(p) }

// ParseGroupKey разбирает ключ группы; ok=false, если key не ключ группы.
func ParseGroupKey(key string) (p int, ok bool) {
	rest, found := strings.CutPrefix(key, "#P")
	if !found {
		return 0, false
	}
	p, err := strconv.Atoi(rest)
	if err != nil || GroupKey(p) != key {
		return 0, false
	}
	return p, true
}

// GroupOf — номер группы задачи id: приоритет корня её ветки.
func GroupOf(f *Forest, id string) int {
	root := id
	if a := Ancestors(f, id); len(a) > 0 {
		root = a[0]
	}
	if iss, ok := f.Issue(root); ok {
		return iss.Priority
	}
	return 0
}

// Row — строка дерева для отрисовки. Строка-заголовок группы: IsGroup, Issue == nil,
// Depth 0, Group — приоритет, GroupCount — число видимых задач группы без учёта
// сворачивания. У задач в режиме групп Group — номер их группы.
type Row struct {
	Issue                          *bd.Issue
	Depth                          int
	HasChildren, Expanded, Context bool
	Group                          int
	IsGroup                        bool
	GroupCount                     int
}

// Key — ID задачи строки или GroupKey для заголовка группы.
func (r Row) Key() string {
	if r.IsGroup {
		return GroupKey(r.Group)
	}
	return r.Issue.ID
}

// Passes — фильтры статуса и типа (без Query).
func Passes(iss *bd.Issue, o Options) bool {
	if iss.Status == "closed" && !o.ShowClosed {
		return false
	}
	if iss.IssueType == "event" && !o.ShowEvents {
		return false
	}
	return true
}

// matches — собственное условие узла: фильтры статуса/типа, «готовые» и, если задан, поиск.
func matches(f *Forest, iss *bd.Issue, o Options, q string) bool {
	if !Passes(iss, o) {
		return false
	}
	if o.ReadyOnly && f.Waiting(iss.ID) {
		return false
	}
	return matchesQuery(iss, q)
}

// matchesQuery — нормализованный запрос q (normQuery) встречается в ID или заголовке.
func matchesQuery(iss *bd.Issue, q string) bool {
	return q == "" || strings.Contains(strings.ToLower(iss.ID+" "+iss.Title), q)
}

func normQuery(o Options) string { return strings.ToLower(strings.TrimSpace(o.Query)) }

// visibleSet — видимость узлов: узел виден, если проходит сам или виден потомок.
func visibleSet(f *Forest, o Options, q string) map[string]bool {
	visible := map[string]bool{}
	var calc func(iss *bd.Issue) bool
	calc = func(iss *bd.Issue) bool {
		v := matches(f, iss, o, q)
		for _, c := range f.children[iss.ID] {
			if calc(c) {
				v = true
			}
		}
		visible[iss.ID] = v
		return v
	}
	for _, r := range f.roots {
		calc(r)
	}
	return visible
}

// groupRoots — видимые корни по группам и номера групп по возрастанию.
func groupRoots(f *Forest, visible map[string]bool) (map[int][]*bd.Issue, []int) {
	by := map[int][]*bd.Issue{}
	for _, r := range f.roots {
		if visible[r.ID] {
			by[r.Priority] = append(by[r.Priority], r)
		}
	}
	keys := make([]int, 0, len(by))
	for p := range by {
		keys = append(keys, p)
	}
	sort.Ints(keys)
	return by, keys
}

// collect добавляет в out видимые узлы ветки iss в порядке обхода, без учёта сворачивания.
func collect(f *Forest, visible map[string]bool, iss *bd.Issue, out []*bd.Issue) []*bd.Issue {
	if !visible[iss.ID] {
		return out
	}
	out = append(out, iss)
	for _, c := range f.children[iss.ID] {
		out = collect(f, visible, c, out)
	}
	return out
}

// GroupMembers — видимые задачи группы p (корни приоритета p и их потомки) без учёта
// сворачивания, в порядке дерева. Группировка в o не важна.
func GroupMembers(f *Forest, o Options, p int) []*bd.Issue {
	visible := visibleSet(f, o, normQuery(o))
	var out []*bd.Issue
	for _, r := range f.roots {
		if r.Priority == p {
			out = collect(f, visible, r, out)
		}
	}
	return out
}

// Rows строит плоский список видимых строк.
func Rows(f *Forest, o Options, e Expansion) []Row {
	q := normQuery(o)
	visible := visibleSet(f, o, q)

	var rows []Row
	// Сверху вниз: выдача строк, дети свёрнутого узла пропускаются.
	var walk func(iss *bd.Issue, depth, group int)
	walk = func(iss *bd.Issue, depth, group int) {
		if !visible[iss.ID] {
			return
		}
		has := false
		for _, c := range f.children[iss.ID] {
			if visible[c.ID] {
				has = true
				break
			}
		}
		exp := e.Expanded(f, iss.ID)
		if q != "" {
			// При поиске предки совпадений раскрыты принудительно.
			exp = has
		}
		rows = append(rows, Row{
			Issue: iss, Depth: depth, HasChildren: has, Expanded: exp,
			Context: !matches(f, iss, o, q), Group: group,
		})
		if !exp {
			return
		}
		for _, c := range f.children[iss.ID] {
			walk(c, depth+1, group)
		}
	}
	if !o.GroupByPriority {
		for _, r := range f.roots {
			walk(r, 0, 0)
		}
		return rows
	}
	// Группы: корни по приоритету, внутри прежняя сортировка соседей; пустых групп нет.
	by, keys := groupRoots(f, visible)
	for _, p := range keys {
		n := 0
		for _, r := range by[p] {
			n += len(collect(f, visible, r, nil))
		}
		// При поиске группа раскрыта принудительно, как предки совпадений.
		exp := q != "" || e.GroupExpanded(p)
		rows = append(rows, Row{Group: p, IsGroup: true, GroupCount: n, HasChildren: true, Expanded: exp})
		if !exp {
			continue
		}
		for _, r := range by[p] {
			walk(r, 1, p)
		}
	}
	return rows
}

// ShortPrefix — общий префикс ID вида "<x>-": самый длинный общий префикс всех ID,
// обрезанный по последний дефис. "" — если такого нет или у какого-то ID после него
// не остаётся хвоста.
func ShortPrefix(issues []bd.Issue) string {
	if len(issues) == 0 {
		return ""
	}
	pre := issues[0].ID
	for _, is := range issues[1:] {
		n := 0
		for n < len(pre) && n < len(is.ID) && pre[n] == is.ID[n] {
			n++
		}
		pre = pre[:n]
	}
	i := strings.LastIndexByte(pre, '-')
	if i < 0 {
		return ""
	}
	pre = pre[:i+1]
	for _, is := range issues {
		if len(is.ID) == len(pre) {
			return ""
		}
	}
	return pre
}

// StatusCounts — число задач леса по статусам среди проходящих фильтры c/e (Passes);
// запрос и сворачивание не учитываются. Для сводки в верхней строке.
func StatusCounts(f *Forest, o Options) map[string]int {
	counts := map[string]int{}
	for _, iss := range f.byID {
		if Passes(iss, o) {
			counts[iss.Status]++
		}
	}
	return counts
}

// WaitingCount — число ждущих задач (Forest.Waiting) среди проходящих фильтры c/e
// (Passes); запрос, ReadyOnly и сворачивание не учитываются. Для счётчика в верхней строке.
func WaitingCount(f *Forest, o Options) int {
	n := 0
	for id, iss := range f.byID {
		if Passes(iss, o) && f.Waiting(id) {
			n++
		}
	}
	return n
}
