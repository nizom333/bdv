package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/i18n"
)

// helpEntry — строка справки «клавиша  действие» с любым числом пробелов между ними.
func (h *harness) wantHelpEntry(key, action string) {
	h.t.Helper()
	for _, l := range h.viewLines() {
		i := strings.Index(l, " "+key+" ")
		if i < 0 {
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(l[i+len(key)+1:], " "), action) {
			return
		}
	}
	h.t.Fatalf("в справке нет строки %q → %q:\n%s", key, action, h.view())
}

// Справка v1.2 в 80x24 на обоих языках: целиком, без прокрутки, все клавиши
// копирования, мыши, R и c в подтверждении bd init на месте, подсказка про выделение
// Shift/Fn+мышь видна.
func TestHelpV12Fits80x24(t *testing.T) {
	for _, c := range []struct {
		lang                 i18n.Lang
		title                string
		entries              [][2]string
		note, shift, fn, end string
	}{
		{i18n.EN, "Help · any key closes", [][2]string{
			{"y", "copy ID"}, {"Y", "copy title"}, {"D", "copy description"}, {"M", "copy card"},
			{"S", "capture on / off"}, {"R", "ready: not blocked"}, {"wheel", "cursor / scroll"}, {"i", "bd init, then y"}, {"c", "IDs: hash or 1, 2, 3"},
		}, "Select text:", "Shift+drag (Ghostty, iTerm2)", "Fn+drag (Terminal.app)", "back to tree"},
		{i18n.RU, "Справка · любая клавиша закрывает", [][2]string{
			{"y", "копировать ID"}, {"Y", "заголовок"}, {"D", "описание"}, {"M", "карточку"},
			{"S", "перехват вкл / выкл"}, {"R", "готовые (без блока)"}, {"колесо", "курсор / прокрутка"}, {"i", "bd init, затем y"}, {"c", "ID: хеш или 1, 2, 3"},
		}, "Выделить текст:", "Shift+мышь (Ghostty, iTerm2)", "Fn+мышь (Terminal.app)", "назад к дереву"},
	} {
		h := newHarness(t, fixtureMain, func(o *Options) { o.Lang = c.lang })
		h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
		h.keys("?")
		h.wantFits(80, 24, "справка "+c.lang.String())
		h.wantView(c.title, c.note, c.shift, c.fn, c.end, "Tab", "Ctrl+S")
		for _, e := range c.entries {
			h.wantHelpEntry(e[0], e[1])
		}
	}
}
