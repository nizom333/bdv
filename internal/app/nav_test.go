package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestMoveDownUp(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("ep.1")
	h.keys("down", "k")
	h.wantSelected("ep.1")
	h.keys("up")
	h.wantSelected("ep")
	h.keys("G")
	h.wantSelected("old.1")
	h.keys("j")
	h.wantSelected("old.1")
	h.keys("g")
	h.wantSelected("ep")
	h.keys("k")
	h.wantSelected("ep")
}

func TestCollapseExpand(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("h")
	h.wantIDs("ep", "bl", "lone", "old", "old.1")
	h.wantSelected("ep")
	h.keys("l")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.keys("enter")
	h.wantIDs("ep", "bl", "lone", "old", "old.1")
	h.keys("space")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.keys("left")
	h.wantIDs("ep", "bl", "lone", "old", "old.1")
	h.keys("right")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
}

func TestExpandLeafIsNoop(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "l", "enter")
	h.wantSelected("bl")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
}

func TestLeftOnLeafSelectsParent(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("ep.1")
	h.keys("h")
	h.wantSelected("ep")
	// Свёрнутый узел: h ведёт к родителю, а корень остаётся на месте.
	h.keys("G", "h")
	h.wantSelected("old")
	h.keys("h", "h")
	h.wantSelected("old")
}

func TestTreeRowTruncatesCyrillic(t *testing.T) {
	h := newHarness(t, fixtureMain)
	found := false
	for _, l := range strings.Split(h.view(), "\n") {
		if w := lipgloss.Width(l); w > 120 {
			t.Fatalf("ширина строки %d > 120: %q", w, l)
		}
		if strings.Contains(l, "lone") {
			found = true
			if !strings.Contains(l, "…") {
				t.Fatalf("строка lone не обрезана: %q", l)
			}
		}
	}
	if !found {
		t.Fatalf("нет строки lone:\n%s", h.view())
	}
}

func TestTreeScrollsWithCursor(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 120, Height: 6})
	h.keys("G")
	h.wantSelected("old.1")
	h.wantView("old.1")
	h.wantNoView("Команды и таблица")
	if n := len(strings.Split(h.view(), "\n")); n != 6 {
		t.Fatalf("строк в view = %d, want 6:\n%s", n, h.view())
	}
	h.keys("g")
	h.wantView("Команды и таблица")
}

func TestTreeRowStripsEscapes(t *testing.T) {
	const fx = `[{"id":"x\u001b[2J","title":"Злой \u001b]0;x\u0007 заголовок\nвторая\tстрока","status":"open","priority":2,"issue_type":"task"}]`
	h := newHarness(t, fx)
	v := h.view()
	if strings.ContainsAny(v, "\x1b\x07") {
		t.Fatalf("view содержит управляющие символы: %q", v)
	}
	h.wantView("Злой ]0;x заголовок вторая строка")
	// Статус «не найдена» тоже чистится.
	h = newHarness(t, fixtureMain, func(o *Options) { o.InitialID = "z\x1b]0;t\x07" })
	if v := h.view(); strings.ContainsAny(v, "\x1b\x07") {
		t.Fatalf("статус содержит управляющие символы: %q", v)
	}
	h.wantView("task z]0;t not found")
}

func TestErrorScreenStripsEscapes(t *testing.T) {
	h := prepareHarness(t, fixtureMain)
	h.fake.Fail("list", "Error: \x1b]0;pwn\x07 boom", 1)
	h.start()
	if v := h.view(); strings.ContainsAny(v, "\x1b\x07") {
		t.Fatalf("экран ошибки содержит управляющие символы: %q", v)
	}
	h.wantView("Error: ]0;pwn boom")
}
