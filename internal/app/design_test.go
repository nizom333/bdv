package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/i18n"
)

// wantFrame проверяет рамку панели [l, r): уголки, боковые границы на каждой строке тела.
func (h *harness) wantFrame(c [2]int) {
	h.t.Helper()
	ls := h.viewLines()
	top := h.frameTop() // под верхней строкой и строкой вкладок
	cell := func(l string, x int) string { return ansi.Cut(l, x, x+1) }
	if top != 2 || cell(ls[top], c[0]) != "╭" || cell(ls[top], c[1]-1) != "╮" {
		h.t.Fatalf("верхняя граница [%d,%d) в строке %d: %q", c[0], c[1], top, ls[top])
	}
	last := ls[len(ls)-2]
	if cell(last, c[0]) != "╰" || cell(last, c[1]-1) != "╯" {
		h.t.Fatalf("нижняя граница [%d,%d): %q", c[0], c[1], last)
	}
	for i, l := range ls[top+1 : len(ls)-2] {
		if cell(l, c[0]) != "│" || cell(l, c[1]-1) != "│" {
			h.t.Fatalf("строка тела %d без боковых границ [%d,%d): %q", i, c[0], c[1], l)
		}
	}
}

// Обе панели в скруглённых рамках, заголовки вписаны в верхнюю границу:
// дерево — «Tasks (N)», карточка — полный ID; высота тела height-5 (v1.4: строка вкладок).
func TestFramesAndTitles(t *testing.T) {
	h := newHarness(t, fixturePrefixed)
	h.wantSelected("acme-shop-fjpw")
	tr, cd := h.panes()
	if tr[1] != 48 || cd != [2]int{48, 120} {
		t.Fatalf("panes = %v %v", tr, cd)
	}
	h.wantFrame(tr)
	h.wantFrame(cd)
	if got := h.treeTitle(); got != activeMark+" Tasks (2)" {
		t.Errorf("заголовок дерева = %q", got)
	}
	if got := h.cardTitle(); got != "acme-shop-fjpw" {
		t.Errorf("заголовок карточки = %q", got)
	}
	ls := h.viewLines()
	if len(ls) != 40 || len(h.treeLines()) != 40-chromeRows {
		t.Fatalf("строк %d, тела %d:\n%s", len(ls), len(h.treeLines()), h.view())
	}
	for i, l := range ls {
		if w := ansi.StringWidth(l); w != 120 {
			t.Errorf("строка %d шириной %d: %q", i, w, l)
		}
	}
	// Строка дерева во всю внутреннюю ширину, заголовок группы и задача на месте.
	tl := h.treeLines()
	if !strings.HasPrefix(tl[0], " ▾ P1 · 2") || !strings.Contains(tl[1], "○ fjpw Первая") {
		t.Errorf("строки дерева:\n%s", strings.Join(tl, "\n"))
	}
	// Карточка: заголовок задачи без ID, затем чипы.
	cl := h.cardLines()
	if strings.TrimSpace(cl[0]) != "Первая" || !strings.Contains(cl[1], "OPEN") || strings.Contains(cl[0]+cl[1], "acme") {
		t.Errorf("карточка:\n%s", strings.Join(cl, "\n"))
	}
	// Заголовок группы в рамке карточки — P1, сводка в той же рамке.
	h.keys("k")
	h.wantSelected("#P1")
	h.wantFrame(cd)
	if got := h.cardTitle(); got != "P1" {
		t.Errorf("заголовок карточки группы = %q", got)
	}
	if got := h.cardLines()[0]; strings.TrimSpace(got) != "2 tasks" {
		t.Errorf("первая строка карточки группы = %q", got)
	}
	h.wantCard("○ 2 open")
	// Без выбора заголовок карточки пуст, рамка остаётся.
	e := newHarness(t, `[{"id":"x","title":"Закрытая","status":"closed","priority":2,"issue_type":"task"}]`)
	e.wantSelected("")
	_, ec := e.panes()
	e.wantFrame(ec)
	if got := e.cardTitle(); got != "" {
		t.Errorf("заголовок пустой карточки = %q", got)
	}
}

// Активная панель помечена знаком activeMark перед заголовком рамки (цвет в тестах не виден).
func TestActiveFrameFollowsFocus(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	if !strings.HasPrefix(h.treeTitle(), activeMark+" ") || strings.Contains(h.cardTitle(), activeMark) {
		t.Fatalf("фокус дерева: %q | %q", h.treeTitle(), h.cardTitle())
	}
	h.keys("tab")
	if strings.Contains(h.treeTitle(), activeMark) || h.cardTitle() != activeMark+" ep" {
		t.Fatalf("фокус карточки: %q | %q", h.treeTitle(), h.cardTitle())
	}
	h.keys("tab")
	if !strings.HasPrefix(h.treeTitle(), activeMark+" ") || strings.Contains(h.cardTitle(), activeMark) {
		t.Fatalf("снова дерево: %q | %q", h.treeTitle(), h.cardTitle())
	}
	// В узком режиме единственная панель всегда активна.
	h.send(tea.WindowSizeMsg{Width: 70, Height: 30})
	if got := h.treeTitle(); got != activeMark+" Tasks (6)" {
		t.Fatalf("узкое дерево: %q", got)
	}
	h.keys("enter")
	if got := h.cardTitle(); got != activeMark+" ep" {
		t.Fatalf("узкая карточка: %q", got)
	}
}

// Верхняя строка: имя проекта и сводка по статусам видимых при текущих c/e задач;
// нулевые счётчики скрыты, кроме blocked. Запрос сводку не меняет.
func TestTopBarSummary(t *testing.T) {
	h := newHarness(t, fixtureMain, func(o *Options) { o.Project = "demo-app" })
	const open = "◐ 1 in progress · ○ 4 open · ● 0 blocked"
	if got := h.topBar(); !strings.HasPrefix(got, "demo-app") || !strings.Contains(got, open) || strings.Contains(got, "closed") {
		t.Fatalf("верхняя строка = %q", got)
	}
	h.keys("c")
	if got := h.topBar(); !strings.Contains(got, open+" · ✓ 2 closed") {
		t.Fatalf("с закрытыми = %q", got)
	}
	h.keys("e")
	if got := h.topBar(); !strings.Contains(got, open+" · ✓ 3 closed") {
		t.Fatalf("с событиями = %q", got)
	}
	h.keys("/", "з", "а", "п")
	if got := h.topBar(); !strings.Contains(got, open+" · ✓ 3 closed") {
		t.Fatalf("при поиске = %q", got)
	}
	// Ширина: строка обрезается по окну.
	h.send(tea.WindowSizeMsg{Width: 40, Height: 20})
	if w := ansi.StringWidth(h.topBar()); w > 40 {
		t.Fatalf("ширина верхней строки %d > 40: %q", w, h.topBar())
	}
	// Без имени проекта — только сводка; имя чистится от управляющих символов.
	n := newHarness(t, fixtureMain)
	if got := n.topBar(); !strings.HasPrefix(got, "◐ 1 in progress") {
		t.Fatalf("без проекта = %q", got)
	}
	e := newHarness(t, fixtureMain, func(o *Options) { o.Project = "x\x1b]0;t\x07y" })
	if got := e.topBar(); strings.ContainsAny(got, "\x1b\x07") || !strings.HasPrefix(got, "x]0;ty") {
		t.Fatalf("имя не очищено: %q", got)
	}
	ru := newHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	if got := ru.topBar(); !strings.Contains(got, "◐ 1 в работе · ○ 4 открыто · ● 0 блок") {
		t.Fatalf("RU = %q", got)
	}
}

// Статус-строка: слева время или ⚠ с ошибкой, затем режимы словами, справа подсказки.
func TestStatusLineModes(t *testing.T) {
	h := newHarness(t, fixtureMain)
	if got := h.statusLine(); !strings.HasPrefix(got, "updated 15:04:05") || !strings.HasSuffix(got, "/ search · m comment · ? help") {
		t.Fatalf("по умолчанию = %q", got)
	}
	h.wantNoStatus("closed", "events", "no groups", "filter")
	// При вводе за запросом стоит одна клетка курсора textinput, затем разделитель.
	h.keys("/", "з", "а", "п")
	if got := h.statusLine(); !strings.HasPrefix(got, "/зап  · updated 15:04:05") {
		t.Fatalf("при вводе = %q", got)
	}
	h.keys("esc")
	h.keys("c", "e", "p", "/", "з", "а", "п", "enter")
	got := h.statusLine()
	last := -1
	for _, s := range []string{"updated 15:04:05", "closed", "events", "no groups", "filter: зап", "? help"} {
		i := strings.Index(got, s)
		if i <= last {
			t.Fatalf("статус-строка %q: нет %q по порядку", got, s)
		}
		last = i
	}
	// Узкое окно: подсказки сокращаются, справка остаётся.
	h.send(tea.WindowSizeMsg{Width: 60, Height: 20})
	got = h.statusLine()
	if !strings.HasSuffix(got, "? help") || strings.Contains(got, "/ search") || ansi.StringWidth(got) > 60 {
		t.Fatalf("узкая статус-строка = %q", got)
	}
	// Ошибка обновления вместо времени, со знаком ⚠.
	h.send(tea.WindowSizeMsg{Width: 120, Height: 20})
	h.fake.Fail("list", "Error: dolt down", 1)
	h.keys("r")
	if got := h.statusLine(); !strings.HasPrefix(got, "⚠ update failed: Error: dolt down") || strings.Contains(got, "updated 15:04:05") {
		t.Fatalf("с ошибкой = %q", got)
	}
	ru := newHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	ru.keys("c", "e")
	ru.wantStatus("обновлено 15:04:05", "закрытые", "события", "/ поиск · m комментарий · ? справка")
}

// fixtureMany — n корневых задач с длинными заголовками, по приоритетам 0–4.
func fixtureMany(n int) string {
	var b strings.Builder
	b.WriteString("[")
	for i := 1; i <= n; i++ {
		if i > 1 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"id":"proj-t%02d","title":"Задача номер %02d с очень длинным заголовком для узкого окна","status":"open","priority":%d,"issue_type":"task","labels":["batch:%02d"],"description":"%s"}`,
			i, i, i%5, i, strings.Repeat("Длинное описание с переносом. ", 8))
	}
	b.WriteString("]")
	return b.String()
}

// wantFits — все строки не шире w и ровно hgt строк.
func (h *harness) wantFits(w, hgt int, what string) {
	h.t.Helper()
	ls := h.viewLines()
	if len(ls) != hgt {
		h.t.Fatalf("%s: строк %d, want %d:\n%s", what, len(ls), hgt, h.view())
	}
	for i, l := range ls {
		if lw := ansi.StringWidth(l); lw > w {
			h.t.Fatalf("%s: строка %d шириной %d > %d: %q", what, i, lw, w, l)
		}
	}
}

// wantCursorVisible — строка выбранной задачи (короткий ID) видна в дереве.
func (h *harness) wantCursorVisible(short string) {
	h.t.Helper()
	for _, l := range h.treeLines() {
		if strings.Contains(l, " "+short+" ") {
			return
		}
	}
	h.t.Fatalf("строки %s нет в дереве:\n%s", short, strings.Join(h.treeLines(), "\n"))
}

// 80x24: ни одна строка не шире окна, высота ровно 24, курсор дерева виден.
func TestViewFits80x24(t *testing.T) {
	h := newHarness(t, fixtureMany(40), func(o *Options) { o.Project = "very-long-project-name-for-the-top-bar" })
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.wantFits(80, 24, "старт")
	h.wantCursorVisible("t05")
	h.keys("G")
	h.wantSelected("proj-t39")
	h.wantFits(80, 24, "конец")
	h.wantCursorVisible("t39")
	h.keys("k", "k", "k")
	h.wantCursorVisible("t24")
	h.keys("g")
	h.wantSelected("#P0")
	h.wantFits(80, 24, "начало")
	h.keys("tab", "pgdown")
	h.wantFits(80, 24, "карточка")
	h.keys("tab", "c", "e", "p", "/", "з", "а", "д")
	h.wantFits(80, 24, "режимы и ввод")
	h.keys("enter", "?")
	h.wantFits(80, 24, "справка")
	h.wantView("copy ID", "back to tree")
	h.keys("x")
	// Узкий режим — то же.
	h.send(tea.WindowSizeMsg{Width: 79, Height: 24})
	h.wantFits(79, 24, "узкий")
	h.keys("enter")
	h.wantFits(79, 24, "узкая карточка")
}

// Узкий режим: одна панель в рамке на всю ширину; справка тоже в рамке.
func TestNarrowFramed(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 70, Height: 30})
	h.wantFrame([2]int{0, 70})
	h.wantFits(70, 30, "дерево")
	if got := h.treeTitle(); got != activeMark+" Tasks (6)" {
		t.Fatalf("заголовок = %q", got)
	}
	h.keys("enter")
	h.wantFrame([2]int{0, 70})
	h.wantCardOf("ep", "Команды и таблица")
	h.keys("?")
	ls := h.viewLines()
	if !strings.HasPrefix(ls[0], "╭") || !strings.Contains(ls[0], "Help") || !strings.HasPrefix(ls[len(ls)-1], "╰") {
		t.Fatalf("справка без рамки:\n%s", h.view())
	}
	h.wantFits(70, 30, "справка")
}

// Слишком маленькое окно: одна строка «window too small», обрезанная по ширине;
// ни паники, ни строк шире или выше окна — на любом экране.
func TestTooSmallWindow(t *testing.T) {
	for _, sz := range [][2]int{{1, 1}, {10, 3}, {19, 10}, {80, 5}} {
		w, hg := sz[0], sz[1]
		for _, lang := range []i18n.Lang{i18n.EN, i18n.RU} {
			h := newHarness(t, fixtureMain, func(o *Options) { o.Lang = lang })
			h.send(tea.WindowSizeMsg{Width: w, Height: hg})
			want := ansi.Truncate(i18n.For(lang).TooSmall, w, "")
			for _, k := range []string{"", "j", "tab", "?", "x", "/", "з", "esc", "enter", "G"} {
				if k != "" {
					h.keys(k)
				}
				if got := h.view(); got != want {
					t.Fatalf("%dx%d %v после %q: view = %q, want %q", w, hg, lang, k, got, want)
				}
			}
			// Окно снова нормальное — обычный экран.
			h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
			h.wantFits(80, 24, "после маленького окна")
		}
	}
	// Загрузка в маленьком окне — тоже одна строка.
	h := prepareHarness(t, fixtureMain)
	h.holdList = true
	h.send(tea.WindowSizeMsg{Width: 10, Height: 3})
	h.run(h.m.Init())
	if got := h.view(); got != "window too" {
		t.Fatalf("загрузка 10x3 = %q", got)
	}
	// Граница: 20x6 — уже обычный экран.
	h = newHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 20, Height: 6})
	h.wantFits(20, 6, "20x6")
	h.wantNoView("too small")
}
