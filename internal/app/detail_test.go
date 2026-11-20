package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/nizom333/bdv/internal/bd"
)

func TestDetailFollowsCursor(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantCardOf("ep.1", "Карточка Записи")
	h.wantCount("show", 2)
}

func TestCommentsFetchedOnlyWhenPresent(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.wantCount("comments", 0)
	h.keys("j")
	h.wantCount("comments", 1)
	h.wantView("Comments", "Первый комментарий")
}

func TestDetailCached(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "k", "j")
	h.wantCount("show", 2)
	h.wantCount("comments", 1)
}

func TestStaleShowCachedNotShown(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	const dep = `[{"id":"ep.1","dependencies":[],
 "dependents":[{"id":"d-1","title":"Зависимая","status":"open","priority":2,"dependency_type":"blocks"}]}]`
	h.fake.SetShow("ep.1", dep)
	h.wantSelected("ep")
	// Ответ пришёл для задачи, которая сейчас не выбрана.
	h.send(showMsg{id: "ep.1", d: bd.Detail{Dependents: []bd.Related{
		{ID: "d-1", Title: "Зависимая", Status: "open", Priority: 2, DependencyType: "blocks"},
	}}})
	h.wantNoView("Зависимая")
	before := h.fake.Count("show")
	h.keys("j")
	h.wantView("Зависимая")
	h.wantCount("show", before)
}

func TestShowErrorNotice(t *testing.T) {
	h := prepareFlatHarness(t, fixtureMain)
	h.fake.Fail("show", "boom", 1)
	h.start()
	h.wantView("failed to load links: boom")
	// Ошибка тоже кешируется: повторный выбор не дёргает bd.
	h.keys("j", "k")
	h.wantCount("show", 2)
}

func TestBlockedByVisible(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("ep.1")
	lines := h.cardLines()
	i := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, "Blocked by") })
	if i < 0 {
		t.Fatalf("в карточке нет Blocked by:\n%s", strings.Join(lines, "\n"))
	}
	if !slices.ContainsFunc(lines[i+1:], func(l string) bool {
		return strings.Contains(l, "[") && strings.Contains(l, "bl ") && strings.Contains(l, "Блокер")
	}) {
		t.Fatalf("после Blocked by нет ссылки на bl «Блокер»:\n%s", strings.Join(lines, "\n"))
	}
}

func TestFocusDefaultsToTree(t *testing.T) {
	h := newHarness(t, fixtureMain)
	if f := h.m.Focus(); f != "tree" {
		t.Fatalf("Focus = %q, want tree", f)
	}
}

// Длинный заголовок переносится в карточке: последнее слово видно (дерево его обрезает).
func TestLongTitleWrapsInCard(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "j")
	h.wantSelected("lone")
	h.wantCard("колонку")
}
