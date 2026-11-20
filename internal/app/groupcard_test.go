package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/i18n"
)

// v1.4: карточка заголовка группы — сводка и секция Tasks: задачи группы в порядке
// дерева с учётом фильтров, «[n] иконка id  заголовок», ⊘ у ждущих; цифра — переход.
func TestGroupCardListsTasks(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.wantSelected("bl")
	h.keys("j")
	h.wantSelected("#P1")
	h.wantCardLine0("2 tasks")
	h.wantCard("Tasks", "[1] ◐ ep  Команды и таблица", "[2] ○ ep.1 "+blockedMark+"  Карточка Записи")
	// Закрытая ep.2 скрыта фильтром — и в списке группы её нет.
	h.wantNoCard("ep.2", "[3]")
	h.keys("c")
	h.wantSelected("#P1")
	h.wantCard("[3] ✓ ep.2  Поиск в рамке")
	h.keys("c")
	h.wantNoCard("ep.2")
	// Переход цифрой выбирает задачу; фокус и вкладка не меняются.
	h.keys("2")
	h.wantSelected("ep.1")
	h.wantCardOf("ep.1", "Карточка Записи")
}

// Переход цифрой из карточки свёрнутой группы раскрывает группу и выбирает задачу.
func TestGroupCardJumpRevealsCollapsed(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("j", "h")
	h.wantSelected("#P1")
	h.wantIDs("#P0", "bl", "#P1", "#P2", "old", "old.1", "#P3", "lone")
	h.wantCard("[1] ◐ ep  Команды и таблица")
	h.keys("1")
	h.wantSelected("ep")
	h.wantIDs("#P0", "bl", "#P1", "ep", "ep.1", "#P2", "old", "old.1", "#P3", "lone")
}

// Поиск сужает список группы, как и счётчик заголовка.
func TestGroupCardFollowsQuery(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("/", "К", "а", "р", "т", "enter", "k", "k")
	h.wantSelected("#P1")
	h.wantCardLine0("2 tasks")
	h.wantCard("[1] ◐ ep  Команды и таблица", "[2] ○ ep.1")
}

// Больше девяти задач: нумеруются первые девять, цифры за пределами — ничего.
func TestGroupCardNumbersFirstNine(t *testing.T) {
	var b strings.Builder
	b.WriteString("[")
	for i := 1; i <= 11; i++ {
		if i > 1 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":"g%02d","title":"Задача %02d","status":"open","priority":2,"issue_type":"task","created_at":"2026-11-%02dT10:00:00Z","updated_at":"2026-11-%02dT10:00:00Z"}`, i, i, i, i)
	}
	b.WriteString("]")
	h := newHarness(t, b.String())
	h.wantSelected("g01")
	h.keys("k")
	h.wantSelected("#P2")
	h.wantCard("[9] ○ g09  Задача 09", "g10  Задача 10", "g11  Задача 11")
	h.wantNoCard("[10]")
	h.keys("9")
	h.wantSelected("g09")
}

// Карточка группы в 80x24 и в узком окне на обоих языках влезает целиком.
func TestGroupCardFits(t *testing.T) {
	for _, l := range []i18n.Lang{i18n.EN, i18n.RU} {
		h := newHarness(t, fixtureMany(40), func(o *Options) { o.Lang = l })
		for _, sz := range [][2]int{{80, 24}, {70, 24}} {
			h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			h.keys("g")
			h.wantSelected("#P0")
			if sz[0] < 80 {
				h.keys("enter")
			}
			h.wantFits(sz[0], sz[1], fmt.Sprintf("группа %s %dx%d", l, sz[0], sz[1]))
			h.wantCard("Tasks", "[1]")
			if sz[0] < 80 {
				h.keys("esc")
			}
		}
	}
}
