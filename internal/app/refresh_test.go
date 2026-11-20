package app

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/bd"
)

func (h *harness) wantVisible(id string, want bool) {
	h.t.Helper()
	if got := slices.Contains(h.m.VisibleIDs(), id); got != want {
		h.t.Fatalf("%q видна = %v, want %v; VisibleIDs = %v", id, got, want, h.m.VisibleIDs())
	}
}

func TestManualRefresh(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.fake.SetList(fixtureMainV2)
	h.keys("r")
	h.wantVisible("new", true)
	h.wantCount("list", 2)
}

func TestRefreshKeyIsRuneWhileTyping(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("/", "r")
	h.wantCount("list", 1)
	h.wantView("/r")
}

func TestPollRefresh(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.fake.SetList(fixtureMainV2)
	h.tick()
	h.wantVisible("new", true)
	h.wantCount("list", 2)
	if len(h.timers) != 1 {
		t.Fatalf("timers = %v, want один следующий pollMsg", h.timers)
	}
	if _, ok := h.timers[0].(pollMsg); !ok {
		t.Fatalf("timers[0] = %T, want pollMsg", h.timers[0])
	}
}

func TestPollSkipsWhileInFlight(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.holdList = true
	h.keys("r")
	h.tick()
	h.wantCount("list", 2) // первая загрузка + r; опрос не стартовал
	h.keys("r")
	h.wantCount("list", 2) // и повторный r тоже
	h.holdList = false
	h.release()
	h.tick()
	h.wantCount("list", 3) // после ответа опрос снова работает
}

func TestRefreshKeepsSelectionAndExpansion(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	// Свернуть ep и old, затем выбрать bl.
	h.keys("g", "h", "G", "h", "h", "g", "j")
	h.wantIDs("ep", "bl", "lone", "old")
	h.wantSelected("bl")
	h.fake.SetList(fixtureMainV2)
	h.keys("r")
	h.wantSelected("bl")
	h.wantVisible("ep.1", false)
	// У old открытый ребёнок, но ручное сворачивание важнее значения по умолчанию.
	h.wantVisible("old.1", false)
	h.wantVisible("new", true)
}

func TestRefreshSelectionGone(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "j")
	h.wantSelected("lone")
	h.fake.SetList(fixtureMainV2)
	h.keys("r")
	// lone удалена: следующий сосед по старому лесу — old.
	h.wantSelected("old")
}

func TestRefreshClosedSelectedMovesToSibling(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("ep.1")
	h.fake.SetList(fixtureMainV2)
	h.keys("r")
	// ep.1 закрылась; ep.2 и ep.3 скрыты, поэтому выбор уходит к родителю.
	h.wantSelected("ep")
}

func TestRefreshInvalidatesChangedDetail(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("ep.1")
	h.wantShow("ep.1", 1)
	h.wantCount("comments", 1)
	h.fake.SetList(fixtureMainV2)
	h.keys("c", "r")
	h.wantSelected("ep.1")
	// updated_at у ep.1 изменился: подробности и комментарии запрошены заново.
	h.wantShow("ep.1", 2)
	h.wantCount("comments", 2)
}

func TestRefreshKeepsUnchangedDetail(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j")
	h.wantSelected("bl")
	h.wantShow("bl", 1)
	h.fake.SetList(fixtureMainV2)
	h.keys("r")
	h.wantSelected("bl")
	h.wantShow("bl", 1)
}

func TestRefreshKeepsShowNotice(t *testing.T) {
	h := prepareFlatHarness(t, fixtureMain)
	h.fake.Fail("show", "boom", 1)
	h.start()
	h.wantView("failed to load links: boom")
	h.tick()
	// Опрос, задача не изменилась: ошибка остаётся в кеше и не повторяется.
	h.wantCount("list", 2)
	h.wantShow("ep", 1)
	h.wantView("failed to load links: boom")
}

// Ручной r повторяет неудавшуюся загрузку связей выбранной задачи.
func TestManualRefreshRetriesShowNotice(t *testing.T) {
	h := prepareFlatHarness(t, fixtureMain)
	h.fake.Fail("show", "boom", 1)
	h.fake.SetShow("ep", `[{"id":"ep","dependents":[
 {"id":"bl","title":"Блокер","status":"open","priority":0,"dependency_type":"blocks"}]}]`)
	h.start()
	h.wantView("failed to load links: boom")
	h.wantNoView("Blocks")
	h.fake.Fail("show", "", 0)
	h.keys("r")
	h.wantShow("ep", 2)
	h.wantNoView("failed to load links")
	h.wantView("Blocks")
}

// Ручной r повторяет и неудавшуюся загрузку комментариев.
func TestManualRefreshRetriesCommentsNotice(t *testing.T) {
	h := prepareFlatHarness(t, fixtureMain)
	h.fake.Fail("comments", "boom", 1)
	h.start()
	h.keys("j")
	h.wantSelected("ep.1")
	h.wantView("failed to load comments: boom")
	h.fake.Fail("comments", "", 0)
	h.keys("r")
	h.wantCount("comments", 2)
	h.wantNoView("failed to load comments")
	h.wantView("Первый комментарий")
}

func TestRefreshFailureKeepsData(t *testing.T) {
	h := newHarness(t, fixtureMain)
	before := h.m.VisibleIDs()
	h.fake.Fail("list", "Error: dolt down\nдетали", 1)
	h.keys("r")
	h.wantIDs(before...)
	h.wantView("update failed: Error: dolt down")
	h.wantNoView("детали")
	// Пометка не снимается сменой выбора.
	h.keys("j")
	h.wantView("update failed: Error: dolt down")
}

func TestRefreshRecovers(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.fake.Fail("list", "Error: dolt down", 1)
	h.keys("r")
	h.wantView("update failed")
	h.fake.Fail("list", "", 0)
	h.keys("r")
	h.wantNoView("update failed")
	h.wantView("updated 15:04:05")
}

func TestRefreshErrorCleaned(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.fake.Fail("list", "Error: \x07dolt\tdown", 1)
	h.keys("r")
	h.wantView("update failed: Error: dolt down")
	h.wantNoView("\x07")
}

func TestKeepsFilterOnRefresh(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("/", "з", "а", "п", "enter")
	ids := h.m.VisibleIDs()
	h.keys("r")
	h.wantCount("list", 2)
	h.wantView("filter: зап")
	h.wantIDs(ids...)
}

func TestRefreshKeepsCardScroll(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 120, Height: 8})
	h.keys("j", "tab")
	top := h.view()
	h.keys("j", "j")
	scrolled := h.view()
	if scrolled == top {
		t.Fatalf("карточка не прокрутилась:\n%s", top)
	}
	h.keys("r")
	h.wantCount("list", 2)
	if got := h.view(); got != scrolled {
		t.Fatalf("после обновления прокрутка сбилась:\n%s\nwant:\n%s", got, scrolled)
	}
}

func TestRefreshKeepsDefaultExpandedBranchOpen(t *testing.T) {
	h := newHarness(t, fixtureMain)
	// ep раскрыта по умолчанию; в V2 её последний открытый ребёнок закрывается.
	h.wantVisible("ep.1", true)
	h.fake.SetList(fixtureMainV2)
	h.keys("c", "r")
	// Ветка, которую пользователь видел раскрытой, не сворачивается из-за обновления.
	h.wantVisible("ep.1", true)
}

func TestRefreshNewBranchUsesDefault(t *testing.T) {
	h := newHarness(t, fixtureMain)
	// new.1 — новый открытый ребёнок у bl, которая была листом: ветка раскрывается по общим правилам.
	h.fake.SetList(fixtureMain[:len(fixtureMain)-1] + `,
{"id":"bl.1","title":"Новый ребёнок","status":"open","priority":2,"issue_type":"task","parent":"bl",
 "created_at":"2026-11-19T14:00:00Z","updated_at":"2026-11-19T14:00:00Z"}
]`)
	h.keys("r")
	h.wantVisible("bl.1", true)
}

func TestStaleShowAfterInvalidationDropped(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("ep.1")
	h.fake.SetList(fixtureMainV2)
	h.keys("c", "r")
	h.wantSelected("ep.1")
	// Ответ, выданный до инвалидации (gen 0), не попадает ни в кеш, ни на экран.
	h.send(showMsg{id: "ep.1", gen: 0, d: bd.Detail{Dependents: []bd.Related{
		{ID: "d-1", Title: "Устаревшая", Status: "open", Priority: 2, DependencyType: "blocks"},
	}}})
	h.wantNoView("Устаревшая")
	// Рендер карточки кешируется до следующего списка: обновляем, чтобы карточка
	// собралась заново из кеша подробностей.
	h.keys("r")
	h.wantSelected("ep.1")
	h.wantNoView("Устаревшая")
	h.wantShow("ep.1", 2)
}

func TestStaleCommentsAfterInvalidationDropped(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.fake.SetList(fixtureMainV2)
	h.keys("c", "r")
	h.send(commentsMsg{id: "ep.1", gen: 0, c: []bd.Comment{{ID: 9, IssueID: "ep.1", Author: "x", Text: "Устаревший комментарий"}}})
	h.wantNoView("Устаревший комментарий")
	h.keys("r")
	h.wantNoView("Устаревший комментарий")
	h.wantView("Первый комментарий")
	h.wantCount("comments", 2)
}

func TestRefreshCollapsedDefaultBranchFollowsDefault(t *testing.T) {
	const v1 = `[
{"id":"a","title":"Открытая","status":"open","priority":2,"issue_type":"task"},
{"id":"x","title":"Закрытый эпик","status":"closed","priority":2,"issue_type":"epic"},
{"id":"x.1","title":"Закрытый ребёнок","status":"closed","priority":2,"issue_type":"task","parent":"x"}
]`
	h := newFlatHarness(t, v1)
	h.keys("c", "r")
	h.wantIDs("a", "x") // x свёрнута по умолчанию: открытых потомков нет
	h.fake.SetList(v1[:len(v1)-1] + `,
{"id":"x.2","title":"Новый открытый","status":"open","priority":2,"issue_type":"task","parent":"x"}
]`)
	h.keys("r")
	// Свёрнутая по умолчанию ветка не замораживается и раскрывается по общим правилам.
	h.wantVisible("x.2", true)
}

func TestRefreshKeepsTreeOffset(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 120, Height: 7}) // три строки дерева на шесть задач
	firstRow := func() string { return h.treeLines()[0] }
	top := firstRow()
	h.keys("j", "j", "j", "j", "j")
	scrolled := firstRow()
	if scrolled == top {
		t.Fatalf("дерево не прокрутилось:\n%s", h.view())
	}
	h.keys("k") // курсор внутри окна, а не у края
	h.keys("r")
	h.wantCount("list", 2)
	if got := firstRow(); got != scrolled {
		t.Fatalf("первая строка дерева = %q, want %q", got, scrolled)
	}
}
