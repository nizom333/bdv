package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/nizom333/bdv/internal/card"
	"github.com/nizom333/bdv/internal/testcolor"
)

// rowColors — цвета, которые встречаются на строке дерева: иконки статусов и P0/P1.
func rowColors() map[string]lipgloss.AdaptiveColor {
	cs := map[string]lipgloss.AdaptiveColor{}
	for _, st := range []string{"in_progress", "blocked", "deferred", "closed"} {
		cs[st] = card.StatusColor(st)
	}
	for _, p := range []int{0, 1} {
		cs["P"+string(rune('0'+p))] = card.PrioStyle(p).GetForeground().(lipgloss.AdaptiveColor)
	}
	return cs
}

// Выбранная строка читается: в активной панели все цвета строки — не ниже 3:1 к фону
// выделения в обеих темах (красный 196 на прежнем фоне 60 давал 1.5:1), текст — не ниже
// 4.5:1. В неактивной панели — красные P0 и blocked не ниже 3:1.
func TestSelectionContrast(t *testing.T) {
	on := selOn.GetBackground().(lipgloss.AdaptiveColor)
	off := selOff.GetBackground().(lipgloss.AdaptiveColor)
	fg := selOn.GetForeground().(lipgloss.AdaptiveColor)
	check := func(name, a, b string, min float64) {
		t.Helper()
		if r := testcolor.Contrast(a, b); r < min {
			t.Errorf("%s: %s на %s — %.2f:1, want ≥ %.1f", name, a, b, r, min)
		}
	}
	check("текст/light", fg.Light, on.Light, 4.5)
	check("текст/dark", fg.Dark, on.Dark, 4.5)
	for name, c := range rowColors() {
		check(name+"/active/light", c.Light, on.Light, 3)
		check(name+"/active/dark", c.Dark, on.Dark, 3)
		if name == "blocked" || name == "P0" {
			check(name+"/inactive/light", c.Light, off.Light, 3)
			check(name+"/inactive/dark", c.Dark, off.Dark, 3)
		}
	}
	// Активное и неактивное выделение различаются.
	if on == off {
		t.Errorf("selOn и selOff одного цвета: %v", on)
	}
}

// sgrBefore — параметры последней SGR-последовательности перед подстрокой sub в s.
func sgrBefore(t *testing.T, s, sub string) []string {
	t.Helper()
	i := strings.Index(s, sub)
	if i < 0 {
		t.Fatalf("нет %q в %q", sub, s)
	}
	j := strings.LastIndex(s[:i], "\x1b[")
	if j < 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s[j+2:i], "m"), ";")
}

// faintSGR — есть ли среди параметров SGR приглушение (2), не считая цветов 38/48.
func faintSGR(ps []string) bool {
	for i := 0; i < len(ps); i++ {
		switch ps[i] {
		case "38", "48":
			if i+1 < len(ps) && ps[i+1] == "5" {
				i += 2
			} else {
				i += 4
			}
		case "2":
			return true
		}
	}
	return false
}

// На выбранной строке ID не приглушён (faint на фоне выделения плохо читается),
// на остальных строках — приглушён.
func TestSelectedIDNotFaint(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j")
	h.wantSelected("bl")
	for i, r := range h.m.rows {
		if r.IsGroup || r.Context {
			continue
		}
		line := h.m.treeLine(r, i == h.m.cursor, true, 60)
		if got := faintSGR(sgrBefore(t, line, r.Issue.ID)); got != (i != h.m.cursor) {
			t.Errorf("%s: faint = %v, selected = %v: %q", r.Issue.ID, got, i == h.m.cursor, line)
		}
	}
}
