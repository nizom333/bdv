package app

import "testing"

func TestToggleClosed(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("c")
	h.wantIDs("ep", "ep.1", "ep.2", "bl", "lone", "old", "old.1")
	h.wantStatus("closed")
	h.keys("c")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.wantNoStatus("closed")
}

func TestToggleEvents(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("c", "e")
	h.wantIDs("ep", "ep.1", "ep.2", "ep.3", "bl", "lone", "old", "old.1")
	h.wantStatus("closed", "events")
}

func TestToggleHidesSelected(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("c", "j", "j")
	h.wantSelected("ep.2")
	h.keys("c")
	// ep.3 — скрытый event, поэтому выбирается предыдущий сосед.
	h.wantSelected("ep.1")
	h.wantCardOf("ep.1", "Карточка Записи")
	h.keys("j")
	h.wantSelected("bl")
}

// Регрессия Task 5: скрытая последняя строка оставляла курсор за концом списка.
func TestToggleHiddenLastRowKeepsCursorInRange(t *testing.T) {
	const fx = `[
{"id":"a","title":"Открытая","status":"open","priority":2,"issue_type":"task"},
{"id":"b","title":"Закрытая","status":"closed","priority":2,"issue_type":"task"}
]`
	h := newFlatHarness(t, fx)
	h.keys("c", "j")
	h.wantSelected("b")
	h.keys("c")
	h.wantIDs("a")
	h.wantSelected("a")
	h.wantCardOf("a", "Открытая")
	h.keys("k")
	h.wantSelected("a")
}

// Ни соседей, ни видимых предков: выбор падает на первую строку.
func TestToggleHiddenSelectionFallsBackToFirstRow(t *testing.T) {
	const fx = `[
{"id":"p","title":"Закрытый родитель","status":"closed","priority":2,"issue_type":"epic"},
{"id":"p.1","title":"Закрытый ребёнок","status":"closed","priority":2,"issue_type":"task","parent":"p"},
{"id":"q","title":"Открытая","status":"open","priority":2,"issue_type":"task"}
]`
	h := newFlatHarness(t, fx)
	h.keys("c", "j", "l", "j")
	h.wantSelected("p.1")
	h.keys("c")
	h.wantIDs("q")
	h.wantSelected("q")
	h.wantCardOf("q", "Открытая")
}

func TestToggleHidesEverything(t *testing.T) {
	h := newHarness(t, `[{"id":"x","title":"Закрытая","status":"closed","priority":2,"issue_type":"task"}]`)
	h.keys("c")
	h.wantSelected("x")
	h.keys("c")
	h.wantIDs()
	h.wantSelected("")
	h.wantNoCard("Закрытая")
}

func TestSearchNoMatches(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("/", "п", "о", "и")
	// ep.2 «Поиск в рамке» закрыта.
	h.wantIDs()
	h.wantView("Nothing found: пои")
	h.wantNoView("No open tasks")
}

func TestSearchCyrillicTitle(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("/", "к", "о", "н", "т")
	h.wantIDs("lone")
	h.wantSelected("lone")
}

func TestSearchContextAndSelection(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("/", "з", "а", "п")
	h.wantIDs("ep", "ep.1")
	h.wantSelected("ep.1")
	h.wantView("/зап")
}

func TestSearchEnterKeepsFilter(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("/", "з", "а", "п", "enter")
	h.wantView("filter: зап")
	h.keys("k")
	h.wantSelected("ep")
	h.keys("j")
	h.wantSelected("ep.1")
	h.wantIDs("ep", "ep.1")
	h.wantView("filter: зап")
}

func TestSearchEscClears(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("/", "з", "а", "п", "esc")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.wantSelected("ep.1")
	h.wantNoView("/зап", "filter:")
}

func TestSearchEscClearsPinnedFilter(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("/", "з", "а", "п", "enter", "esc")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.wantSelected("ep.1")
	h.wantNoView("filter:")
}

func TestSearchRunesAreNotCommands(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("/", "q", "c", "?", "1")
	if h.quit {
		t.Fatal("q в режиме ввода завершил программу")
	}
	h.wantNoStatus("closed")
	h.wantNoView("copy ID")
	h.wantView("/qc?1")
}

func TestSearchEmptyEnterNoFilter(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("/", "enter")
	h.wantNoView("filter:")
	h.keys("q")
	if !h.quit {
		t.Fatal("после выхода из ввода q не работает")
	}
}

func TestDigitJump(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("ep.1")
	// [1] — Parent ep.
	h.keys("1")
	h.wantSelected("ep")
	h.wantCardOf("ep", "Команды и таблица")
}

func TestDigitJumpToClosedEnablesClosed(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	// Children карточки ep: [1] ep.1, [2] ep.2, [3] ep.3.
	h.keys("2")
	h.wantSelected("ep.2")
	h.wantStatus("closed")
	h.wantCardOf("ep.2", "Поиск в рамке")
	h.keys("g", "3")
	h.wantSelected("ep.3")
	h.wantStatus("events")
}

func TestDigitJumpExpandsAncestors(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("h")
	h.wantIDs("ep", "bl", "lone", "old", "old.1")
	h.keys("1")
	h.wantSelected("ep.1")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
}

func TestDigitJumpClearsQuery(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("/", "з", "а", "п", "enter")
	h.wantSelected("ep.1")
	// [2] — Blocked by bl, который не подходит под «зап».
	h.keys("2")
	h.wantSelected("bl")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.wantNoView("filter:")
}

func TestDigitOutOfRange(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "j")
	h.wantSelected("lone")
	h.keys("9")
	h.wantSelected("lone")
}

func TestDigitJumpNotFound(t *testing.T) {
	h := newHarness(t, `[
{"id":"a","title":"Сирота","status":"open","priority":2,"issue_type":"task","parent":"zz"},
{"id":"b","title":"Соседка","status":"open","priority":2,"issue_type":"task"}
]`)
	h.wantSelected("a")
	h.keys("1")
	h.wantSelected("a")
	h.wantView("task zz not found")
	// Статус живёт до следующей смены выбора.
	h.keys("j")
	h.wantSelected("b")
	h.wantNoView("not found")
}

func TestInitialIDNotFoundClearedOnMove(t *testing.T) {
	h := newHarness(t, fixtureMain, func(o *Options) { o.InitialID = "zz" })
	h.wantView("task zz not found")
	h.keys("j")
	h.wantNoView("not found")
}

// Esc после поиска без совпадений возвращает выбор, бывший до поиска.
func TestSearchEscRestoresSelectionAfterNoMatches(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j")
	h.wantSelected("bl")
	h.keys("/", "z", "z", "z")
	h.wantSelected("")
	h.keys("esc")
	h.wantSelected("bl")
}

// То же для закреплённого фильтра без совпадений.
func TestPinnedEscRestoresSelectionAfterNoMatches(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "/", "z", "z", "z", "enter")
	h.wantSelected("")
	h.keys("esc")
	h.wantSelected("bl")
}
