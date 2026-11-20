package app

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/i18n"
	"github.com/nizom333/bdv/internal/testbd"
	"github.com/nizom333/bdv/internal/tree"
)

// fixtureCommentsBl — комментарии bl после отправки (в списке у bl comment_count 0).
const fixtureCommentsBl = `[{"id":7,"issue_id":"bl","author":"me","text":"Новый комментарий","created_at":"2026-11-19T15:00:00Z"}]`

// commentBox — строки поля комментария: от строки с заголовком до статус-строки;
// nil, если поля нет. Заголовок — «Comment on» или «Комментарий к».
func (h *harness) commentBox() []string {
	ls := h.viewLines()
	for i, l := range ls[:len(ls)-1] {
		if strings.Contains(l, "Comment on ") || strings.Contains(l, "Комментарий к ") {
			return ls[i : len(ls)-1]
		}
	}
	return nil
}

func (h *harness) wantBox(subs ...string) {
	h.t.Helper()
	box := h.commentBox()
	if box == nil {
		h.t.Fatalf("поля комментария нет:\n%s", h.view())
	}
	b := strings.Join(box, "\n")
	for _, s := range subs {
		if !strings.Contains(b, s) {
			h.t.Fatalf("поле комментария не содержит %q:\n%s", s, b)
		}
	}
}

func (h *harness) wantBoxWithout(subs ...string) {
	h.t.Helper()
	b := strings.Join(h.commentBox(), "\n")
	for _, s := range subs {
		if strings.Contains(b, s) {
			h.t.Fatalf("поле комментария содержит %q:\n%s", s, b)
		}
	}
}

func (h *harness) wantNoBox() {
	h.t.Helper()
	if box := h.commentBox(); box != nil {
		h.t.Fatalf("поле комментария открыто:\n%s", h.view())
	}
}

// typeText набирает текст по одной клавише: пробел — space, \n — enter.
func (h *harness) typeText(s string) {
	for _, r := range s {
		switch r {
		case ' ':
			h.keys("space")
		case '\n':
			h.keys("enter")
		default:
			h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
}

func (h *harness) wantAdds(want ...testbd.CommentAdd) {
	h.t.Helper()
	if got := h.fake.CommentAdds(); !slices.Equal(got, want) {
		h.t.Fatalf("CommentAdds = %+v, want %+v", got, want)
	}
}

// m открывает поле, Ctrl+S отправляет: поле закрывается, тост, комментарий приходит
// обычным путём карточки (запрошен, хотя в списке comment_count 0), список перечитан.
func TestCommentOpenAndSend(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j")
	h.wantSelected("bl")
	h.wantStatus("m comment")
	h.wantNoCard("Comments")
	h.fake.SetComments("bl", fixtureCommentsBl)
	h.keys("m")
	h.wantBox("Comment on bl:", "Ctrl+S send · Esc cancel")
	lists, comments := h.fake.Count("list"), h.fake.Count("comments")
	h.typeText("Новый комментарий")
	h.wantBox("Новый комментарий")
	h.keys("ctrl+s")
	h.wantAdds(testbd.CommentAdd{ID: "bl", Text: "Новый комментарий"})
	h.wantNoBox()
	h.wantStatus("comment added")
	h.wantCount("list", lists+1)
	h.wantCount("comments", comments+1)
	h.wantCard("Comments", "me", "Новый комментарий")
	h.wantSelected("bl")
	// Тост снимается своим таймером.
	h.tick()
	h.wantNoStatus("comment added")
	// Черновик удалён: поле снова пустое.
	h.keys("m")
	h.wantBox("Comment on bl:")
	h.wantBoxWithout("Новый")
}

// Пустой текст и текст из пробелов и переводов строк не отправляются; поле остаётся.
func TestCommentEmptyNotSent(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "m", "ctrl+s", "alt+enter")
	h.wantAdds()
	h.wantBox("Comment on bl:")
	h.typeText("  \n  \n")
	h.keys("ctrl+s", "alt+enter")
	h.wantAdds()
	h.wantBox("Comment on bl:", "Ctrl+S send")
	h.wantNoStatus("comment added")
}

// v1.4: Alt+Enter больше не отправляет — быстрые Esc+Enter терминал склеивает
// в Alt+Enter, и комментарий уходил при отмене. Поле остаётся открытым с текстом,
// отправляет только Ctrl+S; текст уходит без пробелов по краям.
func TestCommentAltEnterDoesNotSend(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "m")
	h.typeText("  строка 1\nстрока 2\n\n")
	h.keys("alt+enter")
	h.wantAdds()
	h.wantBox("Comment on bl:", "строка 1", "Ctrl+S send")
	h.keys("ctrl+s")
	h.wantAdds(testbd.CommentAdd{ID: "bl", Text: "строка 1\nстрока 2"})
	h.wantNoBox()
}

// Пока идёт отправка, второй Ctrl+S не отправляет повторно, ввод, Esc и m не работают,
// показано «sending…».
func TestCommentDoubleSendOnce(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.holdAdd = true
	h.keys("j", "j", "m")
	h.typeText("раз")
	h.keys("ctrl+s", "ctrl+s")
	h.wantAdds(testbd.CommentAdd{ID: "bl", Text: "раз"})
	h.wantBox("sending…", "раз")
	h.wantBoxWithout("Ctrl+S send")
	h.typeText("xm")
	h.keys("backspace", "esc")
	h.wantBox("sending…", "раз")
	h.wantBoxWithout("раx", "разx", "ра ")
	h.release()
	h.wantNoBox()
	h.wantStatus("comment added")
	h.wantAdds(testbd.CommentAdd{ID: "bl", Text: "раз"})
}

// Ctrl+C во время отправки завершает программу и отменяет корневой контекст.
func TestCommentCtrlCWhileSending(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.holdAdd = true
	h.keys("j", "j", "m")
	h.typeText("раз")
	h.keys("ctrl+s", "ctrl+c")
	if !h.quit || h.m.ctx.Err() == nil {
		t.Fatalf("quit = %v, ctx.Err = %v", h.quit, h.m.ctx.Err())
	}
}

// Ошибка: поле остаётся с текстом, под ним первая строка ошибки очищенной; правка
// снова работает, повторная отправка уходит.
func TestCommentErrorKeepsText(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.fake.Fail("comments-add", "Error: database \x1b[31mis locked\x1b[0m\nsecond line", 1)
	h.keys("j", "j", "m")
	h.typeText("текст")
	h.keys("ctrl+s")
	h.wantBox("Comment on bl:", "текст", "database", "is locked", "Ctrl+S send")
	h.wantBoxWithout("second line", "sending…")
	h.wantNoView("\x1b")
	h.wantNoStatus("comment added")
	h.typeText("!")
	h.wantBox("текст!")
	h.fake.Fail("comments-add", "", 0)
	h.keys("ctrl+s")
	h.wantAdds(testbd.CommentAdd{ID: "bl", Text: "текст"}, testbd.CommentAdd{ID: "bl", Text: "текст!"})
	h.wantNoBox()
	h.wantStatus("comment added")
	// Ошибка не остаётся до следующего открытия.
	h.keys("m")
	h.wantBoxWithout("locked")
}

// Таймаут: поле остаётся с текстом, под ним текст таймаута из каталога.
func TestCommentTimeoutKeepsText(t *testing.T) {
	// Запас на старт sh: иначе фейк убивается раньше, чем запишет вызов.
	h := newFlatHarness(t, fixtureMain, func(o *Options) { o.DetailTimeout = 700 * time.Millisecond })
	h.fake.Sleep("comments-add", 5)
	h.keys("j", "j", "m")
	h.typeText("долго")
	start := time.Now()
	h.keys("ctrl+s")
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("отправка ждала %v", d)
	}
	h.wantBox("долго", "bd did not respond in 0.7 s")
	h.wantNoStatus("comment added")
	h.typeText("!")
	h.wantBox("долго!")
}

// Esc закрывает поле без отправки; черновик возвращается на той же задаче,
// на другой задаче поле пустое; после Esc команды снова работают.
func TestCommentEscKeepsDraft(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "m")
	h.typeText("черновик")
	h.keys("esc")
	h.wantNoBox()
	h.wantAdds()
	h.keys("k")
	h.wantSelected("ep.1")
	h.keys("m")
	h.wantBox("Comment on ep.1:")
	h.wantBoxWithout("черновик")
	h.keys("esc", "j")
	h.wantSelected("bl")
	h.keys("m")
	h.wantBox("Comment on bl:", "черновик")
}

// В поле q, j, /, ?, c, y, r и прочие — текст, а не команды; склеенные руны тоже.
// Колесо мыши не двигает выбор.
func TestCommentKeysNotCommands(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "m")
	lists, shows := h.fake.Count("list"), h.fake.Count("show")
	h.keys("q", "j", "/", "?", "c", "y", "r", "p", "1", "tab", "G")
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("kq")})
	h.send(tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown, X: 5, Y: 5})
	if h.quit {
		t.Fatal("q в поле завершил программу")
	}
	h.wantSelected("bl")
	h.wantBox("qj/?cyrp1Gkq")
	h.wantNoView("Help ·")
	// Флат-харнесс: «no groups» остаётся, p группы не включил.
	h.wantStatus("no groups")
	h.wantNoStatus("closed", "/qj")
	h.wantCount("list", lists)
	h.wantCount("show", shows)
	if len(h.copied) != 0 {
		t.Fatalf("y скопировал: %v", h.copied)
	}
	// Склеенные руны вне поля: m открывает его, остальное — текст.
	h.keys("esc")
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("mhi")})
	h.wantBox("Comment on bl:", "qj/?cyrp1Gkqhi")
}

// На заголовке группы, в режиме поиска и в справке m поле не открывает.
func TestCommentNotOnGroupHeader(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("k")
	if _, ok := tree.ParseGroupKey(h.m.SelectedID()); !ok {
		t.Fatalf("выбран %q, want заголовок группы", h.m.SelectedID())
	}
	h.keys("m")
	h.wantNoBox()
	h.keys("j", "/", "m")
	h.wantNoBox()
	h.wantStatus("/m")
	h.keys("esc", "?", "m")
	h.wantNoBox()
	h.wantNoView("Help ·")
	// Пустое дерево — тоже ничего.
	h = newHarness(t, `[]`)
	h.keys("m")
	h.wantNoBox()
}

// 80x24: поле внизу, панели укоротились, курсор дерева виден, ничего не вылезает.
// 80x12: поле занимает всё тело и остаётся рабочим. Узкий режим и 120x40 — тоже.
func TestCommentFits80x24(t *testing.T) {
	h := newHarness(t, fixtureMany(40))
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.keys("G")
	h.wantSelected("proj-t39")
	h.keys("m")
	h.wantFits(80, 24, "поле 80x24")
	h.wantCursorVisible("t39")
	h.wantBox("Comment on proj-t39:", "Ctrl+S send · Esc cancel")
	if n := len(h.treeLines()); n != 24-chromeRows-8 {
		t.Fatalf("тело панелей %d строк, want %d:\n%s", n, 24-chromeRows-8, h.view())
	}
	if n := len(h.commentBox()); n != 8 {
		t.Fatalf("поле %d строк, want 8:\n%s", n, h.view())
	}
	h.typeText("1\n2\n3\n4\n5\n6\n7 " + strings.Repeat("длинная строка ", 10))
	h.wantFits(80, 24, "длинный текст")
	h.wantBox("7 длинная")
	// Ошибка в строке подсказки тоже не вылезает.
	h.fake.Fail("comments-add", strings.Repeat("очень длинная ошибка ", 10), 1)
	h.keys("ctrl+s")
	h.wantFits(80, 24, "ошибка")
	h.wantBox("очень длинная", "Ctrl+S send")
	h.keys("esc")
	h.wantNoBox()
	h.wantFits(80, 24, "после Esc")
	h.wantCursorVisible("t39")
	if n := len(h.treeLines()); n != 24-chromeRows {
		t.Fatalf("после Esc тело панелей %d строк, want %d", n, 24-chromeRows)
	}

	// Низкое окно: поле на всё тело, панели скрыты, ввод и отправка работают.
	h.send(tea.WindowSizeMsg{Width: 80, Height: 12})
	h.keys("m")
	h.wantFits(80, 12, "поле 80x12")
	h.wantBox("Comment on proj-t39:", "Ctrl+S send · Esc cancel", "7 длинная")
	h.wantNoView("Tasks (")
	h.fake.Fail("comments-add", "", 0)
	h.keys("ctrl+s")
	if adds := h.fake.CommentAdds(); len(adds) != 2 || !strings.HasPrefix(adds[1].Text, "1\n2\n3") {
		t.Fatalf("CommentAdds = %+v", adds)
	}
	h.wantNoBox()
	h.wantFits(80, 12, "после отправки 80x12")
	h.wantView("Tasks (")

	// Наименьшее окно 20x6 и узкий режим.
	for _, sz := range [][2]int{{20, 6}, {79, 24}, {120, 40}} {
		h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		h.keys("m")
		h.typeText("x")
		h.wantFits(sz[0], sz[1], "поле")
		h.keys("esc")
		h.wantFits(sz[0], sz[1], "после поля")
	}
	h.send(tea.WindowSizeMsg{Width: 120, Height: 40})
	h.keys("m")
	if n := len(h.treeLines()); n != 40-chromeRows-8 {
		t.Fatalf("120x40: тело панелей %d строк, want %d", n, 40-chromeRows-8)
	}
	h.wantCursorVisible("t39")
}

// Все вызовы bd, кроме записей (wantWriteDiscipline), по-прежнему с --readonly первым
// аргументом; comments add — без него.
func TestOtherCallsStillReadonly(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j")
	h.keys("j", "m")
	h.typeText("запись")
	h.keys("ctrl+s", "r")
	h.tick()
	h.keys("k", "m")
	h.typeText("ещё")
	h.keys("ctrl+s", "c", "e", "j", "j", "j", "r")
	h.tick()
	seen := wantWriteDiscipline(t, h.fake.Calls())
	if seen["comments-add"] != 2 || seen["list"] < 3 || seen["show"] == 0 || seen["comments"] == 0 {
		t.Fatalf("вызовы: %v", seen)
	}
}

// Подсказка m в статус-строке и раздел комментария в справке; справка влезает в 80x24
// на обоих языках без прокрутки.
func TestCommentHintsAndHelp(t *testing.T) {
	for _, c := range []struct {
		lang              i18n.Lang
		hint, title, help string
	}{
		{i18n.EN, "m comment", "Help · any key closes", "Comment"},
		{i18n.RU, "m комментарий", "Справка · любая клавиша закрывает", "Комментарий"},
	} {
		h := newHarness(t, fixtureMain, func(o *Options) { o.Lang = c.lang })
		h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
		h.wantStatus(c.hint)
		h.keys("?")
		h.wantFits(80, 24, "справка "+c.lang.String())
		h.wantView(c.title, c.help, "Ctrl+S", "Esc")
		// v1.4: Alt+Enter убран из отправки и из справки.
		h.wantNoView("Alt+Enter")
	}
}

// Русский интерфейс поля: заголовок, подсказка, «отправка…», «комментарий добавлен».
func TestCommentRussian(t *testing.T) {
	h := newFlatHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	h.holdAdd = true
	h.keys("j", "j", "m")
	h.wantBox("Комментарий к bl:", "Ctrl+S отправить · Esc отмена")
	h.typeText("привет")
	h.keys("ctrl+s")
	h.wantBox("отправка…")
	h.release()
	h.wantStatus("комментарий добавлен")
}

// Длинный комментарий не обрезается: больше 99 строк (предел textarea по умолчанию).
func TestCommentLongTextNotCut(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "m")
	h.typeText(strings.Repeat("строка\n", 120) + "конец")
	h.keys("ctrl+s")
	adds := h.fake.CommentAdds()
	if len(adds) != 1 || strings.Count(adds[0].Text, "\n") != 120 || !strings.HasSuffix(adds[0].Text, "конец") {
		t.Fatalf("строк %d", strings.Count(adds[0].Text, "\n"))
	}
}

// show, ещё идущий в момент успешной отправки, выдан в старом поколении и будет
// отброшен; флаг ожидания снимается, и show запрашивается заново.
func TestCommentReissuesShowInFlight(t *testing.T) {
	h := prepareFlatHarness(t, fixtureMain)
	h.holdShow = true
	h.start()
	h.keys("j")
	h.wantSelected("ep.1")
	h.wantShow("ep.1", 1)
	h.keys("m")
	h.typeText("пока show идёт")
	h.keys("ctrl+s")
	h.wantNoBox()
	h.wantShow("ep.1", 2)
	h.holdShow = false
	h.release()
	h.wantCard("Blocked by", "Блокер")
}

// Подсказки справа в порядке спеки; «m comment» только при выбранной задаче.
func TestCommentHintOrderAndVisibility(t *testing.T) {
	h := newHarness(t, fixtureMain)
	if got := h.statusLine(); !strings.HasSuffix(got, "/ search · m comment · ? help") {
		t.Fatalf("на задаче = %q", got)
	}
	h.keys("k")
	if _, ok := tree.ParseGroupKey(h.m.SelectedID()); !ok {
		t.Fatalf("выбран %q, want заголовок группы", h.m.SelectedID())
	}
	if got := h.statusLine(); !strings.HasSuffix(got, "/ search · ? help") || strings.Contains(got, "m comment") {
		t.Fatalf("на заголовке группы = %q", got)
	}
	h = newHarness(t, `[]`)
	h.wantNoStatus("m comment")
	h.wantStatus("/ search · ? help")
}

// Успешная отправка, пока идёт фоновый list: после него запускается ещё один,
// чтобы список увидел новый комментарий.
func TestCommentReloadsAfterInFlightList(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j")
	h.holdList = true
	h.keys("r") // list в полёте, ответ задержан
	lists := h.fake.Count("list")
	h.keys("m")
	h.typeText("во время list")
	h.keys("ctrl+s")
	h.wantNoBox()
	h.wantCount("list", lists) // reload пропущен: list уже идёт
	h.holdList = false
	h.release()
	h.wantCount("list", lists+1)
	// Повтор один: следующий ответ новых list не порождает.
	h.wantCount("list", lists+1)
	h.keys("r")
	h.wantCount("list", lists+2)
}

// Вставка с переводами строк Windows (CRLF) и одиночным \r не даёт пустых строк:
// \r\n и \r становятся \n.
func TestCommentPasteCRLF(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "m")
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\r\nb"), Paste: true})
	h.keys("ctrl+s")
	h.keys("m")
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c\rd"), Paste: true})
	h.keys("ctrl+s")
	h.wantAdds(testbd.CommentAdd{ID: "bl", Text: "a\nb"}, testbd.CommentAdd{ID: "bl", Text: "c\nd"})
}

// Таймаут отправки: bd мог успеть записать комментарий. Под полем — подсказка проверить
// комментарии, а комментарии задачи запрашиваются заново (хотя в списке comment_count 0),
// чтобы карточка показала, сохранился ли он.
func TestCommentTimeoutRefetchesComments(t *testing.T) {
	h := newFlatHarness(t, fixtureMain, func(o *Options) { o.DetailTimeout = 700 * time.Millisecond })
	h.fake.Sleep("comments-add", 5)
	h.keys("j", "j")
	h.wantNoCard("Comments")
	h.fake.SetComments("bl", fixtureCommentsBl)
	comments := h.fake.Count("comments")
	h.keys("m")
	h.typeText("долго")
	h.keys("ctrl+s")
	h.wantBox("долго", "bd did not respond in 0.7 s", "may have been saved — check comments before resending")
	h.wantCount("comments", comments+1)
	h.wantCard("Comments", "Новый комментарий")
}

// Ошибка bd при отправке: комментарии тоже запрашиваются заново, но подсказки
// про возможное сохранение нет — это только для таймаута.
func TestCommentErrorRefetchesComments(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.fake.Fail("comments-add", "Error: database is locked", 1)
	h.keys("j", "j")
	h.fake.SetComments("bl", fixtureCommentsBl)
	comments := h.fake.Count("comments")
	h.keys("m")
	h.typeText("текст")
	h.keys("ctrl+s")
	h.wantBox("текст", "is locked")
	h.wantBoxWithout("may have been saved")
	h.wantCount("comments", comments+1)
	h.wantCard("Comments", "Новый комментарий")
}

// Русская подсказка таймаута.
func TestCommentTimeoutRussian(t *testing.T) {
	h := newFlatHarness(t, fixtureMain, func(o *Options) {
		o.DetailTimeout = 700 * time.Millisecond
		o.Lang = i18n.RU
	})
	h.fake.Sleep("comments-add", 5)
	h.keys("j", "j", "m")
	h.typeText("долго")
	h.keys("ctrl+s")
	h.wantBox("возможно, сохранён — проверьте комментарии перед повторной отправкой")
}
