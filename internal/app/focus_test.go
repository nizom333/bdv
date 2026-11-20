package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// fixtureLong — ep с двумя детьми, у каждого описание из 60 абзацев.
func fixtureLong() string {
	paras := make([]string, 60)
	for i := range paras {
		paras[i] = fmt.Sprintf("Абзац %02d", i+1)
	}
	desc, _ := json.Marshal(strings.Join(paras, "\n\n"))
	return fmt.Sprintf(`[
{"id":"ep","title":"Команды и таблица","status":"in_progress","priority":1,"issue_type":"epic",
 "created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-10T10:00:00Z"},
{"id":"ep.1","title":"Карточка Записи","status":"open","priority":2,"issue_type":"task","parent":"ep","description":%[1]s,
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-11T10:00:00Z"},
{"id":"ep.2","title":"Вторая длинная","status":"open","priority":2,"issue_type":"task","parent":"ep","description":%[1]s,
 "created_at":"2026-11-03T10:00:00Z","updated_at":"2026-11-12T10:00:00Z"}
]`, desc)
}

func wheel(x int, b tea.MouseButton) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: 3, Button: b, Action: tea.MouseActionPress}
}

func (h *harness) wantFocus(want string) {
	h.t.Helper()
	if got := h.m.Focus(); got != want {
		h.t.Fatalf("Focus = %q, want %q", got, want)
	}
}

func TestTabFocus(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.wantFocus("tree")
	h.keys("tab")
	h.wantFocus("detail")
	h.keys("tab")
	h.wantFocus("tree")
}

func TestDetailFocusKeysDoNotTouchTree(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("tab", "j", "h", "l", "down")
	h.wantSelected("ep")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	// Цифры работают и в фокусе карточки.
	h.keys("1")
	h.wantSelected("ep.1")
	h.wantFocus("detail")
}

func TestDetailScroll(t *testing.T) {
	h := newHarness(t, fixtureLong())
	h.send(tea.WindowSizeMsg{Width: 120, Height: 20})
	h.keys("j")
	h.wantCardOf("ep.1", "Карточка Записи")
	h.wantCard("Абзац 01")
	h.keys("tab", "pgdown")
	h.wantSelected("ep.1")
	h.wantNoCard("Карточка Записи", "Абзац 01")
	h.keys("pgup")
	h.wantCardOf("ep.1", "Карточка Записи")
	// j прокручивает на строку: шапка уходит, курсор дерева на месте.
	h.keys("j")
	h.wantSelected("ep.1")
	h.wantNoCard("Карточка Записи")
	h.keys("k")
	h.wantCardOf("ep.1", "Карточка Записи")
}

func TestDetailScrollResetsOnSelect(t *testing.T) {
	h := newHarness(t, fixtureLong())
	h.send(tea.WindowSizeMsg{Width: 120, Height: 20})
	h.keys("j", "tab", "pgdown")
	h.wantNoCard("Абзац 01")
	h.keys("tab", "j")
	h.wantSelected("ep.2")
	// Верх карточки новой задачи снова виден.
	h.wantCardOf("ep.2", "Вторая длинная")
	h.wantCard("Абзац 01")
}

func TestWheelOverTree(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.send(wheel(5, tea.MouseButtonWheelDown))
	h.wantSelected("ep.1")
	h.wantCardOf("ep.1", "Карточка Записи")
	h.send(wheel(5, tea.MouseButtonWheelUp))
	h.wantSelected("ep")
	h.wantFocus("tree")
}

func TestWheelOverDetailDoesNotMoveCursor(t *testing.T) {
	h := newHarness(t, fixtureLong())
	h.send(tea.WindowSizeMsg{Width: 120, Height: 20})
	h.keys("j")
	h.send(wheel(100, tea.MouseButtonWheelDown))
	h.wantSelected("ep.1")
	h.wantFocus("tree")
	h.wantNoCard("Карточка Записи")
	h.send(wheel(100, tea.MouseButtonWheelUp))
	h.wantCardOf("ep.1", "Карточка Записи")
}

func TestMouseClickIgnored(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.send(tea.MouseMsg{X: 5, Y: 3, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	h.send(tea.MouseMsg{X: 5, Y: 3, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionRelease})
	h.wantSelected("ep")
}

func TestNarrowMode(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 70, Height: 30})
	// Одна панель: дерево, карточки нет.
	if n := strings.Count(h.viewLines()[h.frameTop()], "╭"); n != 1 || !strings.Contains(h.treeTitle(), "Tasks (6)") {
		t.Fatalf("в узком режиме не одно дерево:\n%s", h.view())
	}
	h.keys("enter")
	h.wantFocus("detail")
	h.wantCardOf("ep", "Команды и таблица")
	h.wantNoView("Tasks (6)")
	h.keys("esc")
	h.wantFocus("tree")
	h.wantView("Tasks (6)")
	// Дерево занимает всю ширину, а не 30 колонок.
	h.wantView("Контекст компании для длинного")
}

func TestNarrowExpandWithoutEnter(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 70, Height: 30})
	h.keys("space")
	h.wantIDs("ep", "bl", "lone", "old", "old.1")
	h.keys("l")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.keys("h")
	h.wantIDs("ep", "bl", "lone", "old", "old.1")
	h.wantFocus("tree")
}

func TestNarrowWheelActsOnShownPane(t *testing.T) {
	h := newHarness(t, fixtureLong())
	h.send(tea.WindowSizeMsg{Width: 70, Height: 20})
	// Дерево на всю ширину: колесо двигает курсор при любом X.
	h.send(wheel(50, tea.MouseButtonWheelDown))
	h.wantSelected("ep.1")
	h.keys("enter")
	h.wantCardOf("ep.1", "Карточка Записи")
	// Карточка на всю ширину: колесо прокручивает её даже при малом X.
	h.send(wheel(5, tea.MouseButtonWheelDown))
	h.wantSelected("ep.1")
	h.wantNoCard("Карточка Записи")
}

func TestNarrowEscOrder(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 70, Height: 30})
	// Ввод важнее: Esc сбрасывает запрос и оставляет карточку.
	h.keys("enter", "/", "з", "а", "п")
	h.wantSelected("ep.1")
	h.keys("esc")
	h.wantFocus("detail")
	h.wantNoView("/зап")
	h.keys("esc")
	h.wantFocus("tree")
	// Закреплённый фильтр: первый Esc уводит из карточки, второй сбрасывает фильтр.
	h.keys("/", "з", "а", "п", "enter")
	h.wantFocus("tree")
	h.keys("enter")
	h.wantFocus("detail")
	h.keys("esc")
	h.wantFocus("tree")
	h.wantView("filter: зап")
	h.keys("esc")
	h.wantNoView("filter:")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
}

func TestHelpScreen(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("?")
	h.wantView("Tab", "copy ID")
	h.keys("x")
	h.wantNoView("copy ID")
	h.wantView("Tasks (6)")
}

func TestHelpSwallowsKey(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("?", "q")
	if h.quit {
		t.Fatal("q, закрывший справку, завершил программу")
	}
	h.keys("?", "j")
	h.wantSelected("ep")
	h.wantNoView("copy ID")
}

func TestHelpFits80x24(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.keys("?")
	h.wantView("copy ID", "back to tree", "quit", "Tab", "follow link", "clear")
	for _, l := range strings.Split(h.view(), "\n") {
		if w := ansi.StringWidth(l); w > 80 {
			t.Fatalf("ширина строки справки %d > 80: %q", w, l)
		}
	}
}

func TestHelpNarrow(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 70, Height: 30})
	h.keys("?")
	for i := 0; i < 40 && !strings.Contains(h.view(), "back to tree"); i++ {
		h.keys("j")
	}
	h.wantView("back to tree")
}

func TestHelpScrollsWhenShort(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 60, Height: 12})
	h.keys("?")
	h.wantView("move")
	h.wantNoView("back to tree")
	// Клавиши прокрутки не закрывают справку.
	h.keys("pgdown", "pgdown", "pgdown", "pgdown")
	h.wantView("back to tree")
	h.wantNoView("move")
	h.keys("pgup", "pgup", "pgup", "pgup")
	h.wantView("move")
	// Любая другая клавиша закрывает её и проглатывается.
	h.keys("q")
	if h.quit {
		t.Fatal("q, закрывший справку, завершил программу")
	}
	h.wantNoView("move")
	h.wantView("Tasks (6)")
}
