package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Регрессия: переход цифрой из Specs на задачу, которую Tasks и так помнила выбранной,
// перерисовывает карточку — рамка, тело и ссылки той задачи, а не спеки.
func TestJumpFromSpecsToRememberedTaskRefreshesCard(t *testing.T) {
	h := specsHarness(t)
	h.keys("j")
	h.wantSelected("sp.2")
	h.keys("]")
	h.wantCardOf("sp", "Спека вкладок")
	h.wantCard("[1] ◐ P2 sp.2  Тикет два")
	h.keys("1")
	h.wantTab("tasks")
	h.wantSelected("sp.2")
	h.wantCardOf("sp.2", "Тикет два")
	h.wantNoCard("Tickets", "описание спеки")
	// Ссылки — карточки sp.2: [1] — родитель sp.
	h.wantCard("Parent", "[1] ◐ P1 sp  Спека вкладок")
	h.keys("1")
	h.wantSelected("sp")
}

// То же из Decisions: Tasks помнит t1, решение dd — ребёнок t1, цифра ведёт на t1.
func TestJumpFromDecisionsToRememberedTaskRefreshesCard(t *testing.T) {
	fx := fixtureSpecs[:len(fixtureSpecs)-1] + `,
{"id":"dd","title":"Решение под задачей","status":"open","priority":2,"issue_type":"decision","parent":"t1",
 "created_at":"2026-11-08T10:00:00Z","updated_at":"2026-11-08T10:00:00Z"}]`
	h := newHarness(t, fx, func(o *Options) { o.InitialID = "t1" })
	h.wantSelected("t1")
	h.keys("]", "]", "j")
	h.wantTab("decisions")
	h.wantCardOf("dd", "Решение под задачей")
	h.wantCard("Parent", "[1] ○ P3 t1  Просто задача")
	h.keys("1")
	h.wantTab("tasks")
	h.wantSelected("t1")
	h.wantCardOf("t1", "Просто задача")
	// Ссылки — карточки t1: [1] — ребёнок dd.
	h.wantCard("Children", "[1] ○ P2 dd  Решение под задачей")
	h.keys("1")
	h.wantSelected("dd")
}

// Узкий режим, карточка в фокусе: переход на пустую вкладку возвращает фокус к списку,
// а не оставляет пустую рамку карточки.
func TestEmptyTabNarrowFocusesTree(t *testing.T) {
	h := specsHarness(t)
	h.send(tea.WindowSizeMsg{Width: 70, Height: 24})
	h.keys("enter")
	h.wantFocus("detail")
	h.keys("]")
	h.wantTab("specs")
	h.wantFocus("detail") // выбор есть — фокус остаётся
	h.keys("]", "]")
	h.wantTab("memory")
	h.wantSelected("")
	h.wantFocus("tree")
	h.wantTree("No memories")
	if strings.Contains(h.view(), "╭─ "+activeMark+" ─") {
		t.Fatalf("пустая рамка карточки:\n%s", h.view())
	}
}

// r в ленивой вкладке перечитывает только её данные, не список задач.
func TestLazyTabReloadSkipsList(t *testing.T) {
	h := specsHarness(t)
	h.keys("[", "[")
	h.wantTab("memory")
	lists, mems := h.fake.Count("list"), h.fake.Count("memories")
	h.keys("r")
	h.wantCount("memories", mems+1)
	h.wantCount("list", lists)
	h.keys("]")
	forms := h.fake.Count("formula-list")
	h.keys("r")
	h.wantCount("formula-list", forms+1)
	h.wantCount("list", lists)
}

// Выбор Tasks исчез, пока была открыта другая вкладка: замена ищется по лесу, каким
// он был при уходе из Tasks (соседи исчезнувшей задачи), а не первая задача.
func TestTasksSelectionVanishedWhileAway(t *testing.T) {
	h := specsHarness(t)
	h.keys("j", "j")
	h.wantSelected("sp.1")
	h.keys("]")
	h.wantTab("specs")
	// sp.1 удалена; перечитываем список из Specs.
	h.fake.SetList(strings.Replace(fixtureSpecs, `{"id":"sp.1","title":"Тикет один","status":"open","priority":2,"issue_type":"task","parent":"sp",
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-02T10:00:00Z"},
`, "", 1))
	h.keys("r")
	h.wantSelected("sp")
	h.keys("[")
	h.wantTab("tasks")
	h.wantSelected("sp.2")
}
