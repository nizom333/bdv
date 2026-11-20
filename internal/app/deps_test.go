package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/nizom333/bdv/internal/i18n"
)

// fixtureMainBlClosed — fixtureMain, в котором блокер bl закрыт: ep.1 больше не ждёт.
var fixtureMainBlClosed = strings.Replace(fixtureMain,
	`{"id":"bl","title":"Блокер","status":"open","priority":0,"issue_type":"bug",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"}`,
	`{"id":"bl","title":"Блокер","status":"closed","priority":0,"issue_type":"bug",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-19T10:00:00Z","closed_at":"2026-11-19T10:00:00Z"}`, 1)

// fixtureTwoBlockers — задача ждёт два открытых блокера: один в выборке, другого нет.
const fixtureTwoBlockers = `[
{"id":"w","title":"Ждёт двоих","status":"open","priority":1,"issue_type":"task",
 "dependencies":[{"issue_id":"w","depends_on_id":"b1","type":"blocks"},{"issue_id":"w","depends_on_id":"gone","type":"blocks"}],
 "created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-01T10:00:00Z"},
{"id":"b1","title":"Первый блокер","status":"in_progress","priority":0,"issue_type":"task",
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-02T10:00:00Z"}
]`

// treeLineOf — строка дерева с коротким ID id.
func (h *harness) treeLineOf(id string) string {
	h.t.Helper()
	for _, l := range h.treeLines() {
		if strings.Contains(l, " "+id+" ") {
			return l
		}
	}
	h.t.Fatalf("строки %s нет в дереве:\n%s", id, strings.Join(h.treeLines(), "\n"))
	return ""
}

// У заблокированной задачи перед заголовком ⊘, ID на месте; у прочих знака нет.
// Блокер закрылся — знак пропал.
func TestBlockedMarkerInTree(t *testing.T) {
	h := newHarness(t, fixtureMain)
	if l := h.treeLineOf("ep.1"); !strings.Contains(l, "○ P2 ep.1 ⊘ Карточка Записи") {
		t.Fatalf("строка ep.1 = %q", l)
	}
	for _, id := range []string{"ep", "bl", "lone"} {
		if l := h.treeLineOf(id); strings.Contains(l, "⊘") {
			t.Fatalf("строка %s со знаком ⊘: %q", id, l)
		}
	}
	h.fake.SetList(fixtureMainBlClosed)
	h.keys("r")
	if l := h.treeLineOf("ep.1"); strings.Contains(l, "⊘") {
		t.Fatalf("блокер закрыт, а знак остался: %q", l)
	}
}

// Верхняя строка: «⊘ N waiting» после сводки статусов, только при N > 0; N — задачи,
// видимые при переключателях c/e (запрос и R не влияют).
func TestTopBarWaiting(t *testing.T) {
	h := newHarness(t, fixtureMain)
	if got := h.topBar(); !strings.Contains(got, "● 0 blocked · ⊘ 1 waiting") {
		t.Fatalf("верхняя строка = %q", got)
	}
	h.keys("R", "/", "з", "з", "з")
	if got := h.topBar(); !strings.Contains(got, "⊘ 1 waiting") {
		t.Fatalf("с R и поиском = %q", got)
	}
	h.keys("esc", "R")
	h.fake.SetList(fixtureMainBlClosed)
	h.keys("r")
	if got := h.topBar(); strings.Contains(got, "⊘") || strings.Contains(got, "waiting") {
		t.Fatalf("никто не ждёт, а счётчик есть: %q", got)
	}

	ru := newHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	if got := ru.topBar(); !strings.Contains(got, "● 0 блок · ⊘ 1 ждут") {
		t.Fatalf("RU = %q", got)
	}
}

// Закрытая задача с открытым блокером не ждёт (fixtureMainV2: ep.1 закрыта, bl открыт):
// с c у неё нет ⊘, её нет в счётчике, в карточке нет строки «waiting on», а при R+c
// она видна.
func TestClosedWithOpenBlockerNotWaiting(t *testing.T) {
	h := newFlatHarness(t, fixtureMainV2)
	// У ep все дети закрыты — ветка свёрнута по умолчанию: l раскрывает.
	h.keys("c", "l")
	if l := h.treeLineOf("ep.1"); strings.Contains(l, "⊘") || !strings.Contains(l, "✓") {
		t.Fatalf("строка закрытой ep.1 = %q", l)
	}
	if got := h.topBar(); strings.Contains(got, "⊘") || strings.Contains(got, "waiting") {
		t.Fatalf("закрытая в счётчике: %q", got)
	}
	h.keys("j")
	h.wantSelected("ep.1")
	h.wantCard("Blocked by")
	h.wantNoCard("waiting on")
	h.keys("R")
	h.wantSelected("ep.1")
	if !slices.Contains(h.m.VisibleIDs(), "ep.1") {
		t.Fatalf("R+c скрыл закрытую ep.1: %v", h.m.VisibleIDs())
	}
}

// R: только готовые задачи, пометка «ready»; повторное R возвращает всё.
// Выбранная заблокированная задача скрывается — выбор уходит на ближайшую.
func TestReadyToggle(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.keys("j")
	h.wantSelected("ep.1")
	h.keys("R")
	h.wantIDs("ep", "bl", "lone", "old", "old.1")
	h.wantStatus("ready")
	if h.m.SelectedID() == "ep.1" {
		t.Fatal("скрытая задача осталась выбранной")
	}
	h.keys("R")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.wantNoStatus("ready")
}

// R совместим с группами, поиском и c/e.
func TestReadyWithGroupsSearchClosed(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("R")
	h.wantIDs("#P0", "bl", "#P1", "ep", "#P2", "old", "old.1", "#P3", "lone")
	h.keys("c")
	h.wantIDs("#P0", "bl", "#P1", "ep", "ep.2", "#P2", "old", "old.1", "#P3", "lone")
	h.keys("/", "к", "а", "р", "enter")
	h.wantIDs()
	h.wantView("Nothing found: кар")
	h.keys("esc", "/", "п", "о", "и", "enter")
	h.wantIDs("#P1", "ep", "ep.2")
	h.wantStatus("closed", "ready", "filter: пои")
	h.keys("esc", "c", "e")
	h.wantStatus("events", "ready")
	h.wantNoStatus("closed")
}

// Пустое дерево из-за R — своя подсказка, а не «нет открытых задач».
func TestReadyEmptyTree(t *testing.T) {
	h := newFlatHarness(t, `[{"id":"w","title":"Ждёт","status":"open","priority":1,"issue_type":"task",
 "dependencies":[{"issue_id":"w","depends_on_id":"gone","type":"blocks"}]}]`)
	h.keys("R")
	h.wantIDs()
	h.wantView("No ready tasks — press R to show all")
	h.wantNoView("No open tasks")
}

// Переход по связи на заблокированную задачу выключает R, как c/e для закрытых и событий.
func TestJumpToBlockedTurnsReadyOff(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("R")
	h.wantSelected("ep")
	h.wantCard("Children", "Карточка Записи")
	h.keys("1")
	h.wantSelected("ep.1")
	h.wantNoStatus("ready")
}

// Карточка: под чипами строка «⊘ waiting on N open blocker(s)», только у заблокированной.
func TestCardWaitingLine(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("ep.1")
	if ls := h.cardLines(); len(ls) < 3 || strings.TrimSpace(ls[2]) != "⊘ waiting on 1 open blocker" {
		t.Fatalf("третья строка карточки не «⊘ waiting on 1 open blocker»:\n%s", h.view())
	}
	h.wantCard("Blocked by")
	h.keys("j")
	h.wantSelected("bl")
	h.wantNoCard("waiting on")

	two := newFlatHarness(t, fixtureTwoBlockers)
	two.wantSelected("b1")
	two.keys("j")
	two.wantSelected("w")
	two.wantCard("⊘ waiting on 2 open blockers")

	for _, c := range []struct{ fx, want string }{
		{fixtureMain, "⊘ ждёт 1 открытого блокера"}, {fixtureTwoBlockers, "⊘ ждёт 2 открытых блокеров"},
	} {
		ru := newFlatHarness(t, c.fx, func(o *Options) { o.Lang = i18n.RU })
		ru.keys("j")
		ru.wantCard(c.want)
	}
}

// Ready и ⊘ в RU: пометка «готовые».
func TestReadyRU(t *testing.T) {
	h := newHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	h.keys("R")
	h.wantStatus("готовые")
}
