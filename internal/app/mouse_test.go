package app

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/i18n"
)

// pressS отправляет S напрямую в Update, выполняет его команду и возвращает её
// сообщения (пачка разворачивается): команды Bubble Tea, которые переключают перехват
// мыши, а побочные команды пишут сырые последовательности в Options.Term (h.term).
func (h *harness) pressS() []tea.Msg {
	h.t.Helper()
	m, cmd := h.m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")})
	h.m = m.(Model)
	if cmd == nil {
		h.t.Fatal("S не вернул команду")
	}
	var out []tea.Msg
	var walk func(c tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			for _, c := range msg {
				walk(c)
			}
		case nil:
		default:
			out = append(out, msg)
		}
	}
	walk(cmd)
	return out
}

// hasMsgType — среди msgs есть сообщение того же типа, что want.
func hasMsgType(msgs []tea.Msg, want tea.Msg) bool {
	for _, m := range msgs {
		if reflect.TypeOf(m) == reflect.TypeOf(want) {
			return true
		}
	}
	return false
}

func wheelDown(x int) tea.MouseMsg {
	return tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: x, Y: 5}
}

// S выключает перехват (tea.DisableMouse) и включает обратно (tea.EnableMouseCellMotion);
// по умолчанию перехват включён, пометка «mouse off» только при выключенном.
func TestMouseToggleCmd(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.wantNoStatus("mouse off")
	if msgs := h.pressS(); !hasMsgType(msgs, tea.DisableMouse()) {
		t.Fatalf("S при включённом перехвате: %#v, want tea.DisableMouse", msgs)
	}
	h.wantStatus("mouse off")
	if msgs := h.pressS(); !hasMsgType(msgs, tea.EnableMouseCellMotion()) || hasMsgType(msgs, tea.DisableMouse()) {
		t.Fatalf("S при выключенном перехвате: %#v, want tea.EnableMouseCellMotion", msgs)
	}
	h.wantNoStatus("mouse off")
}

// Колесо в alt screen при выключенном перехвате терминал превращает в ↑/↓ (режим
// alternate scroll, DECSET 1007), и курсор дерева ехал бы. S выключает 1007 вместе
// с перехватом и включает обратно, только если выключал сам; AltScrollOff говорит main,
// что 1007 надо вернуть при выходе.
func TestMouseToggleAlternateScroll(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	if h.m.AltScrollOff() || h.term.Len() != 0 {
		t.Fatalf("до S: AltScrollOff=%v, term=%q", h.m.AltScrollOff(), h.term.String())
	}
	h.pressS()
	if got := h.term.String(); got != "\x1b[?1007l" || !h.m.AltScrollOff() {
		t.Fatalf("S выкл: term=%q AltScrollOff=%v", got, h.m.AltScrollOff())
	}
	h.term.Reset()
	h.pressS()
	if got := h.term.String(); got != "\x1b[?1007h" || h.m.AltScrollOff() {
		t.Fatalf("S вкл: term=%q AltScrollOff=%v", got, h.m.AltScrollOff())
	}
}

// Пока перехват выключен, события мыши ничего не делают; после включения — как раньше.
func TestMouseIgnoredWhileOff(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.wantSelected("ep")
	h.pressS()
	h.send(wheelDown(5))
	h.wantSelected("ep")
	h.pressS()
	h.send(wheelDown(5))
	h.wantSelected("ep.1")
}

// S в поиске, справке и поле комментария — не переключатель.
func TestMouseToggleIgnoredInTypingHelpComment(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("/", "S")
	h.wantStatus("/S")
	h.keys("esc", "?", "S", "m", "S")
	h.wantBox("S")
	h.keys("esc")
	h.wantNoStatus("mouse off")
	h.send(wheelDown(5))
	h.wantSelected("ep.1")
}

func TestMouseOffRU(t *testing.T) {
	h := newFlatHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	h.pressS()
	h.wantStatus("мышь выкл")
}
