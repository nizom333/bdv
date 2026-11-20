package app

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// runes — один KeyMsg с несколькими рунами: так Bubble Tea v1 отдаёт руны,
// пришедшие одним чтением (быстрый набор, автоповтор, tmux send-keys j j j).
func runes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// separately нажимает каждую руну s отдельным KeyMsg.
func (h *harness) separately(s string) {
	for _, r := range s {
		h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestBatchedRunesMoveSelection(t *testing.T) {
	want := newHarness(t, fixtureMain)
	want.separately("jjj")
	if want.m.SelectedID() == "ep" {
		t.Fatal("три отдельных j не сдвинули выбор")
	}

	h := newHarness(t, fixtureMain)
	h.send(runes("jjj"))
	h.wantSelected(want.m.SelectedID())
}

// Склеенные руны ведут себя так же, как те же клавиши по одной:
// цифры, смесь клавиш, qq, переход в поиск и справку посреди пачки.
func TestBatchedRunesMatchSeparatePresses(t *testing.T) {
	for _, s := range []string{"jjj", "jk", "jjjk", "12", "qq", "jq", "/кон", "jj/зап", "?jj", "Gk", "jl"} {
		t.Run(s, func(t *testing.T) {
			want := newHarness(t, fixtureMain)
			want.separately(s)
			h := newHarness(t, fixtureMain)
			h.send(runes(s))
			if got, w := h.m.SelectedID(), want.m.SelectedID(); got != w {
				t.Fatalf("SelectedID = %q, отдельные нажатия дают %q", got, w)
			}
			if got, w := h.m.VisibleIDs(), want.m.VisibleIDs(); !slices.Equal(got, w) {
				t.Fatalf("VisibleIDs = %v, отдельные нажатия дают %v", got, w)
			}
			if got, w := h.m.Focus(), want.m.Focus(); got != w {
				t.Fatalf("Focus = %q, отдельные нажатия дают %q", got, w)
			}
			if h.quit != want.quit {
				t.Fatalf("quit = %v, отдельные нажатия дают %v", h.quit, want.quit)
			}
			// После q экран не показывается; к тому же команды пачки исполняются уже
			// после отмены контекста, и карточка ловит «context canceled».
			if got, w := h.view(), want.view(); !h.quit && got != w {
				t.Fatalf("view отличается.\nпачкой:\n%s\nпо одной:\n%s", got, w)
			}
		})
	}
}

func TestBatchedRunesInSearchTyped(t *testing.T) {
	want := newHarness(t, fixtureMain)
	want.keys("/", "к", "о", "н")

	h := newHarness(t, fixtureMain)
	h.keys("/")
	h.send(runes("кон"))
	h.wantIDs(want.m.VisibleIDs()...)
	h.wantSelected(want.m.SelectedID())
	h.wantView("/кон")
}

func TestPasteOutsideSearchIgnored(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	before := h.view()
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jjcq"), Paste: true})
	if h.quit {
		t.Fatal("вставка с q завершила программу")
	}
	h.wantSelected("ep")
	if got := h.view(); got != before {
		t.Fatalf("вставка вне поиска изменила экран:\n%s", got)
	}

	// В режиме ввода вставка попадает в запрос.
	h.keys("/")
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("кон"), Paste: true})
	h.wantView("/кон")
	h.wantIDs("lone")
}
