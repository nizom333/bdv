package tree

import (
	"reflect"
	"testing"

	"github.com/nizom333/bdv/internal/bd"
)

// blocks — зависимость blocks: from ждёт on.
func blocks(is bd.Issue, on ...string) bd.Issue {
	for _, o := range on {
		is.Dependencies = append(is.Dependencies, bd.Dep{IssueID: is.ID, DependsOnID: o, Type: "blocks"})
	}
	return is
}

// fixtureBlocked:
//
//	ep (P1, ждёт open)        — заблокирован
//	  ep.1 (ждёт done)        — блокер закрыт: готов
//	  ep.2 (ждёт gone)        — блокера нет в выборке: заблокирован
//	open (P0)                 — блокер ep, готов
//	done (closed)             — закрытый блокер
//	solo (P2, ждёт open)      — заблокирован, без потомков
//	rel (P2, related на open) — не blocks: готов
func fixtureBlocked() []bd.Issue {
	rel := iss("rel", "", "open", "task", 2, "06")
	rel.Dependencies = []bd.Dep{
		{IssueID: "rel", DependsOnID: "open", Type: "related"},
		{IssueID: "rel", DependsOnID: "ep", Type: "parent-child"},
		// Чужая запись (issue_id не rel) не делает rel заблокированной.
		{IssueID: "other", DependsOnID: "open", Type: "blocks"},
	}
	return []bd.Issue{
		blocks(iss("ep", "", "in_progress", "epic", 1, "01"), "open"),
		blocks(iss("ep.1", "ep", "open", "task", 1, "02"), "done"),
		blocks(iss("ep.2", "ep", "open", "task", 1, "03"), "gone"),
		iss("open", "", "open", "bug", 0, "04"),
		iss("done", "", "closed", "task", 2, "05"),
		blocks(iss("solo", "", "open", "task", 2, "07"), "open", "done", "open"),
		rel,
	}
}

func TestBlockedBy(t *testing.T) {
	f := Build(fixtureBlocked())
	for _, c := range []struct {
		id   string
		want []string
	}{
		{"ep", []string{"open"}},   // открытый блокер
		{"ep.1", nil},              // закрытый блокер не держит
		{"ep.2", []string{"gone"}}, // блокер вне выборки считается открытым
		{"solo", []string{"open"}}, // повтор блокера — один раз
		{"rel", nil},               // related, parent-child и чужие записи — не блокеры
		{"open", nil},              // без зависимостей
		{"nope", nil},              // задачи нет
	} {
		if got := f.BlockedBy(c.id); !reflect.DeepEqual(got, c.want) {
			t.Errorf("BlockedBy(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

// ReadyOnly: остаются незаблокированные задачи; заблокированный предок готовой задачи —
// строка-контекст; заблокированная задача без готовых потомков скрыта.
func TestReadyOnlyFlat(t *testing.T) {
	f := Build(fixtureBlocked())
	o := Options{ReadyOnly: true}
	rows := Rows(f, o, Expansion{})
	if got, want := ids(rows), []string{"ep", "ep.1", "open", "rel"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	if !rows[0].Context || rows[1].Context || rows[2].Context {
		t.Fatalf("контекст: ep=%v ep.1=%v open=%v", rows[0].Context, rows[1].Context, rows[2].Context)
	}
	if got := ids(Rows(f, Options{}, Expansion{})); !reflect.DeepEqual(got, []string{"ep", "ep.1", "ep.2", "open", "rel", "solo"}) {
		t.Fatalf("без фильтра = %v", got)
	}
}

// С группами: пустые после фильтра группы не показываются, счётчик группы — видимые задачи.
func TestReadyOnlyGroups(t *testing.T) {
	f := Build(fixtureBlocked())
	o := Options{ReadyOnly: true, GroupByPriority: true}
	rows := Rows(f, o, Expansion{})
	if got, want := ids(rows), []string{"#P0", "open", "#P1", "ep", "ep.1", "#P2", "rel"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	if rows[2].GroupCount != 2 || rows[5].GroupCount != 1 {
		t.Fatalf("счётчики групп: P1=%d P2=%d", rows[2].GroupCount, rows[5].GroupCount)
	}
	if got := GroupMembers(f, o, 2); len(got) != 1 || got[0].ID != "rel" {
		t.Fatalf("GroupMembers P2 = %v", got)
	}
}

// С поиском: задача проходит, если готова и совпадает с запросом.
func TestReadyOnlyWithQuery(t *testing.T) {
	f := Build(fixtureBlocked())
	if got := ids(Rows(f, Options{ReadyOnly: true, Query: "ep."}, Expansion{})); !reflect.DeepEqual(got, []string{"ep", "ep.1"}) {
		t.Fatalf("ep. = %v", got)
	}
	if got := ids(Rows(f, Options{ReadyOnly: true, Query: "solo"}, Expansion{})); len(got) != 0 {
		t.Fatalf("solo = %v", got)
	}
}

// С закрытыми: закрытая незаблокированная задача видна, заблокированные — нет.
func TestReadyOnlyWithClosed(t *testing.T) {
	f := Build(fixtureBlocked())
	got := ids(Rows(f, Options{ReadyOnly: true, ShowClosed: true}, Expansion{}))
	if want := []string{"ep", "ep.1", "open", "rel", "done"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
}

// WaitingCount — число ждущих задач (Forest.Waiting) среди видимых при переключателях c/e;
// запрос и ReadyOnly не учитываются. Закрытая задача не ждёт, даже с открытым блокером.
func TestWaitingCount(t *testing.T) {
	f := Build(fixtureBlocked())
	if got := WaitingCount(f, Options{}); got != 3 {
		t.Fatalf("WaitingCount = %d, want 3 (ep, ep.2, solo)", got)
	}
	if got := WaitingCount(f, Options{ReadyOnly: true, Query: "zzz"}); got != 3 {
		t.Fatalf("WaitingCount с фильтрами = %d, want 3", got)
	}
	closed := blocks(iss("c", "", "closed", "task", 2, "08"), "open")
	f = Build(append(fixtureBlocked(), closed))
	if WaitingCount(f, Options{}) != 3 || WaitingCount(f, Options{ShowClosed: true}) != 3 {
		t.Fatalf("закрытая с открытым блокером: %d / %d", WaitingCount(f, Options{}), WaitingCount(f, Options{ShowClosed: true}))
	}
}

// Waiting: задача не закрыта и у неё есть открытый блокер. Закрытая задача с открытым
// блокером не ждёт; BlockedBy по-прежнему перечисляет её блокеры (для карточки).
func TestWaitingPredicate(t *testing.T) {
	closed := blocks(iss("c", "", "closed", "task", 2, "08"), "open")
	f := Build(append(fixtureBlocked(), closed))
	for id, want := range map[string]bool{
		"ep": true, "ep.1": false, "ep.2": true, "solo": true, "rel": false, "open": false,
		"c": false, "nope": false,
	} {
		if got := f.Waiting(id); got != want {
			t.Errorf("Waiting(%q) = %v, want %v", id, got, want)
		}
	}
	if got := f.BlockedBy("c"); !reflect.DeepEqual(got, []string{"open"}) {
		t.Errorf("BlockedBy(c) = %v", got)
	}
}

// ReadyOnly с закрытыми: закрытая задача с открытым блокером не ждёт и остаётся видна.
func TestReadyOnlyKeepsClosedWithOpenBlocker(t *testing.T) {
	closed := blocks(iss("c", "", "closed", "task", 2, "08"), "open")
	f := Build(append(fixtureBlocked(), closed))
	got := ids(Rows(f, Options{ReadyOnly: true, ShowClosed: true}, Expansion{}))
	if want := []string{"ep", "ep.1", "open", "rel", "done", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
}

// Waiting не выделяет память: вызывается для каждой строки дерева при каждом рендере.
func TestWaitingNoAlloc(t *testing.T) {
	f := Build(fixtureBlocked())
	if n := testing.AllocsPerRun(100, func() { f.Waiting("solo"); f.Waiting("ep.1") }); n != 0 {
		t.Fatalf("Waiting: %v аллокаций", n)
	}
}
