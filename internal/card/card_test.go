package card

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/i18n"
	"github.com/nizom333/bdv/internal/testcolor"
)

func TestMain(m *testing.M) {
	// Детерминированные даты и цвета.
	time.Local = time.UTC
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func base() *bd.Issue {
	return &bd.Issue{
		ID: "p-1", Title: "Эпик", Status: "in_progress", Priority: 1,
		IssueType: "epic", Assignee: "alice", Labels: []string{"ui", "bug"},
		CreatedAt: ts("2026-09-23T10:00:00Z"), UpdatedAt: ts("2026-09-23T11:00:00Z"),
	}
}

func render(in Input) (string, []string) {
	in.Style = "notty"
	in.Width = 80
	return Render(in)
}

// lines — строки карточки без ANSI.
func lines(out string) []string { return strings.Split(ansi.Strip(out), "\n") }

func TestHeader(t *testing.T) {
	out, _ := render(Input{Issue: base()})
	for _, want := range []string{"Эпик", "IN PROGRESS", "#ui", "created 2026-09-23 · updated 2026-09-23"} {
		if !strings.Contains(out, want) {
			t.Errorf("нет %q в:\n%s", want, out)
		}
	}
}

// Полный ID только в рамке панели (её рисует app): в карточке его нет, первая строка — заголовок.
func TestCardTitleNoDuplicateID(t *testing.T) {
	out, _ := render(Input{Issue: base()})
	ls := lines(out)
	if ls[0] != "Эпик" {
		t.Errorf("первая строка = %q, want заголовок", ls[0])
	}
	if strings.Contains(out, "p-1") {
		t.Errorf("ID повторяется в карточке:\n%s", out)
	}
}

// Под заголовком строка чипов: бейдж, приоритет, тип, исполнитель; затем метки, затем даты.
func TestCardChips(t *testing.T) {
	out, _ := render(Input{Issue: base()})
	ls := lines(out)
	chips := ls[1]
	last := -1
	for _, want := range []string{"IN PROGRESS", "P1", "epic", "@alice"} {
		i := strings.Index(chips, want)
		if i <= last {
			t.Fatalf("чипы %q: нет %q по порядку", chips, want)
		}
		last = i
	}
	if strings.Contains(chips, "in_progress") {
		t.Errorf("сырой статус в чипах: %q", chips)
	}
	if got := strings.TrimSpace(ls[2]); got != "#ui #bug" {
		t.Errorf("строка меток = %q", got)
	}
	if !strings.HasPrefix(ls[3], "created 2026-09-23") {
		t.Errorf("строка дат = %q", ls[3])
	}
	// Без исполнителя и меток чипов меньше, строки меток нет.
	is := base()
	is.Assignee, is.Labels = "", nil
	out, _ = render(Input{Issue: is})
	ls = lines(out)
	if strings.Contains(ls[1], "@") || !strings.HasPrefix(ls[2], "created") {
		t.Errorf("лишнее без исполнителя и меток:\n%s", out)
	}
}

// Заголовок секции — название и тонкая линия ровно до края панели.
func TestCardSectionRule(t *testing.T) {
	for _, w := range []int{40, 77} {
		is := base()
		is.Description = "текст"
		is.Notes = "заметка"
		out, _ := Render(Input{Issue: is, Width: w, Style: "notty"})
		for _, sec := range []string{"Description", "Notes"} {
			found := false
			for _, l := range lines(out) {
				if strings.HasPrefix(l, sec+" ") {
					found = true
					if ansi.StringWidth(l) != w || !strings.HasSuffix(l, "──") ||
						strings.Trim(strings.TrimPrefix(l, sec+" "), "─") != "" {
						t.Errorf("w=%d: линия секции %q (ширина %d)", w, l, ansi.StringWidth(l))
					}
				}
			}
			if !found {
				t.Errorf("w=%d: нет секции %s:\n%s", w, sec, out)
			}
		}
	}
}

// Комментарий: автор и дата строкой, текст с чертой ▏ слева, между комментариями одна пустая строка.
func TestCardCommentBar(t *testing.T) {
	cs := []bd.Comment{
		{Author: "alice", Text: "первый абзац\n\nвторой абзац", CreatedAt: ts("2026-09-23T10:00:00Z")},
		{Author: "bob", Text: "ответ", CreatedAt: ts("2026-09-24T11:30:00Z")},
	}
	out, _ := render(Input{Issue: base(), Comments: cs})
	ls := lines(out)
	start := slices.IndexFunc(ls, func(l string) bool { return strings.HasPrefix(l, "Comments ") })
	if start < 0 {
		t.Fatalf("нет секции Comments:\n%s", out)
	}
	got := ls[start+1:]
	want := []string{
		"alice  2026-09-23 10:00",
		"▏ первый абзац",
		"▏",
		"▏ второй абзац",
		"",
		"bob  2026-09-24 11:30",
		"▏ ответ",
	}
	for i := range got {
		got[i] = strings.TrimRight(got[i], " ")
	}
	if !slices.Equal(got, want) {
		t.Errorf("комментарии:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Бейджи из каталога в обоих языках; неизвестный статус — в верхнем регистре.
func TestStatusBadges(t *testing.T) {
	cases := []struct {
		text   *i18n.Catalog
		status string
		want   string
	}{
		{nil, "open", "OPEN"},
		{i18n.For(i18n.EN), "open", "OPEN"},
		{i18n.For(i18n.EN), "in_progress", "IN PROGRESS"},
		{i18n.For(i18n.EN), "blocked", "BLOCKED"},
		{i18n.For(i18n.EN), "deferred", "DEFERRED"},
		{i18n.For(i18n.EN), "closed", "CLOSED"},
		{i18n.For(i18n.EN), "pinned", "PINNED"},
		{i18n.For(i18n.RU), "open", "ОТКРЫТА"},
		{i18n.For(i18n.RU), "in_progress", "В РАБОТЕ"},
		{i18n.For(i18n.RU), "blocked", "БЛОК"},
		{i18n.For(i18n.RU), "deferred", "ОТЛОЖЕНА"},
		{i18n.For(i18n.RU), "closed", "ЗАКРЫТА"},
		{i18n.For(i18n.RU), "x\x1b]0;y", "X]0;Y"},
	}
	for _, c := range cases {
		if got := strings.TrimSpace(ansi.Strip(StatusBadge(c.status, c.text))); got != c.want {
			t.Errorf("StatusBadge(%q) = %q, want %q", c.status, got, c.want)
		}
	}
	// В карточке — бейдж языка карточки.
	out, _ := render(Input{Issue: base(), Text: i18n.For(i18n.RU)})
	if !strings.Contains(lines(out)[1], "В РАБОТЕ") {
		t.Errorf("нет русского бейджа:\n%s", out)
	}
	// Цвета статусов: open — обычный, остальные заданы и различаются.
	if StatusColor("open") != (lipgloss.AdaptiveColor{}) {
		t.Errorf("open окрашен: %v", StatusColor("open"))
	}
	seen := map[lipgloss.AdaptiveColor]string{}
	for _, st := range []string{"in_progress", "blocked", "deferred", "closed"} {
		c := StatusColor(st)
		if c == (lipgloss.AdaptiveColor{}) {
			t.Errorf("%s без цвета", st)
		}
		if prev, ok := seen[c]; ok {
			t.Errorf("%s и %s одного цвета", st, prev)
		}
		seen[c] = st
	}
}

// Ни одна строка карточки не шире Width: длинные заголовок, метки, пометка, markdown
// (абзац, список, код, цитата, длинное слово), ссылки, связь с типом и комментарии.
func TestCardLinesFitWidth(t *testing.T) {
	long := strings.Repeat("очень длинный текст ", 10)
	is := base()
	is.Title = long + "финал"
	is.Labels = []string{strings.Repeat("метка", 12), "ok"}
	is.Assignee = strings.Repeat("исполнитель", 6)
	is.Parent = "p-0"
	is.Description = "## Заголовок " + long + "\n\n" + wufDescription + "\n\n- пункт " + long +
		"\n  - вложенный " + long + "\n\n```\n" + strings.Repeat("код_без_пробелов", 10) + "\n```\n\n> цитата " + long +
		"\n\nhttps://example.com/" + strings.Repeat("path/", 30) +
		// Код в строке (фон под пробелами): ansi.Wrap оставлял пробел у границы SGR в конце строки.
		"\n\n- [ ] Ручной прогон на тестовом конфиге: `con ls`, `con ls hrm`, `con names`, `con names -a`, " +
		"`con -h`, неизвестный и неоднозначный запросы, `con pass -s`, отказ при правах 644, опечатка в поле, " +
		"конфликт `a/b` + `a/b/c`; вывод приложен в комментарий" +
		// Сплошные дефисы ansi.Wrap не ломает: нужен жёсткий перенос.
		"\n\nSeparator: " + strings.Repeat("-", 90)
	is.Notes = long
	lk := map[string]*bd.Issue{"p-0": {ID: "p-0", Title: long, Status: "blocked", Priority: 0}}
	in := Input{
		Issue:    is,
		Children: []*bd.Issue{{ID: "c-" + strings.Repeat("x", 50), Title: long, Status: "closed", Priority: 3}},
		Lookup:   func(id string) (*bd.Issue, bool) { i, ok := lk[id]; return i, ok },
		Detail: &bd.Detail{Dependencies: []bd.Related{
			{ID: "r-1", Title: long, Status: "open", Priority: 2, DependencyType: "discovered-from"},
		}},
		Comments: []bd.Comment{{Author: strings.Repeat("автор", 20), Text: long + "\n\n- " + long, CreatedAt: ts("2026-09-23T10:00:00Z")}},
		Notice:   long,
	}
	for _, style := range []string{"notty", "dark"} {
		for _, w := range []int{40, 60, 74, 77} {
			in.Width, in.Style = w, style
			out, _ := Render(in)
			for i, l := range strings.Split(out, "\n") {
				if lw := ansi.StringWidth(l); lw > w {
					t.Errorf("%s/%d: строка %d шириной %d: %q", style, w, i, lw, ansi.Strip(l))
				}
			}
			if !strings.Contains(ansi.Strip(out), "финал") {
				t.Errorf("%s/%d: заголовок обрезан", style, w)
			}
		}
	}
}

// Ссылка: «[n] иконка P id  Заголовок» — между ID и заголовком два пробела.
func TestCardLinkLine(t *testing.T) {
	out, _ := render(Input{Issue: base(), Children: []*bd.Issue{iss("c-1")}})
	if !strings.Contains(out, "[1] ○ P2 c-1  T c-1") {
		t.Errorf("строка ссылки:\n%s", out)
	}
}

func TestClosedHeader(t *testing.T) {
	is := base()
	c := ts("2026-09-24T10:00:00Z")
	is.ClosedAt = &c
	is.CloseReason = "Closed"
	out, _ := render(Input{Issue: is})
	if !strings.Contains(out, "closed 2026-09-24 (Closed)") {
		t.Errorf("нет закрытия:\n%s", out)
	}
}

// Подписи дат берутся из каталога: nil — английский, RU — русский.
func TestHeaderLang(t *testing.T) {
	is := base()
	c := ts("2026-09-24T10:00:00Z")
	is.ClosedAt = &c
	cases := []struct {
		text *i18n.Catalog
		want string
	}{
		{nil, "created 2026-09-23 · updated 2026-09-23 · closed 2026-09-24"},
		{i18n.For(i18n.EN), "created 2026-09-23 · updated 2026-09-23 · closed 2026-09-24"},
		{i18n.For(i18n.RU), "создана 2026-09-23 · обновлена 2026-09-23 · закрыта 2026-09-24"},
	}
	for _, tc := range cases {
		out, _ := render(Input{Issue: is, Text: tc.text})
		if !strings.Contains(out, tc.want) {
			t.Errorf("нет %q в:\n%s", tc.want, out)
		}
	}
}

func TestEmptySectionsOmitted(t *testing.T) {
	out, _ := render(Input{Issue: base()})
	for _, s := range []string{"Description", "Notes", "Parent", "Children", "Blocks", "Blocked by", "Related", "Comments"} {
		if strings.Contains(out, s) {
			t.Errorf("лишняя секция %q:\n%s", s, out)
		}
	}
}

func TestDescriptionMarkdown(t *testing.T) {
	is := base()
	is.Description = "## Заголовок\n- пункт"
	out, _ := render(Input{Issue: is})
	if !strings.Contains(out, "Заголовок") || !strings.Contains(out, "пункт") ||
		!strings.Contains(out, "•") || strings.Contains(out, "- пункт") {
		t.Errorf("markdown не отрендерен:\n%s", out)
	}
}

func iss(id string) *bd.Issue {
	return &bd.Issue{ID: id, Title: "T " + id, Status: "open", Priority: 2}
}

func TestLinksNumberedInSectionOrder(t *testing.T) {
	is := base()
	is.Parent = "p-0"
	lk := map[string]*bd.Issue{"p-0": iss("p-0")}
	in := Input{
		Issue:    is,
		Children: []*bd.Issue{iss("c-1"), iss("c-2")},
		Detail: &bd.Detail{
			Dependencies: []bd.Related{
				{ID: "b-1", Title: "B", Status: "open", Priority: 2, DependencyType: "blocks"},
				{ID: "r-1", Title: "R", Status: "open", Priority: 2, DependencyType: "discovered-from"},
				{ID: "p-0", Title: "P", DependencyType: "parent-child"},
			},
			Dependents: []bd.Related{{ID: "d-1", Title: "D", Status: "open", Priority: 2, DependencyType: "blocks"}},
		},
		Lookup: func(id string) (*bd.Issue, bool) { i, ok := lk[id]; return i, ok },
	}
	out, links := render(in)
	want := []string{"p-0", "c-1", "c-2", "b-1", "d-1", "r-1"}
	if fmt.Sprint(links) != fmt.Sprint(want) {
		t.Fatalf("links = %v", links)
	}
	for _, s := range []string{"[1] ", "[6] ", "(discovered-from)"} {
		if !strings.Contains(out, s) {
			t.Errorf("нет %q:\n%s", s, out)
		}
	}
	last := -1
	for _, h := range []string{"Parent", "Children", "Blocked by", "Blocks", "Related"} {
		i := strings.Index(out, "\n"+h+" ─")
		if i <= last {
			t.Errorf("порядок секций нарушен на %q:\n%s", h, out)
		}
		last = i
	}
}

func TestBlockedByBeforeDetailFromList(t *testing.T) {
	is := base()
	is.Dependencies = []bd.Dep{{IssueID: "p-1", DependsOnID: "b-1", Type: "blocks"}}
	lk := map[string]*bd.Issue{"b-1": iss("b-1")}
	out, links := render(Input{Issue: is, Lookup: func(id string) (*bd.Issue, bool) { i, ok := lk[id]; return i, ok }})
	if !strings.Contains(out, "Blocked by") || !strings.Contains(out, "T b-1") || strings.Contains(out, "\nBlocks ─") {
		t.Errorf("неверно:\n%s", out)
	}
	if fmt.Sprint(links) != "[b-1]" {
		t.Errorf("links = %v", links)
	}
}

func TestMoreThanNineLinks(t *testing.T) {
	var ch []*bd.Issue
	for i := 1; i <= 11; i++ {
		ch = append(ch, iss(fmt.Sprintf("c-%d", i)))
	}
	out, links := render(Input{Issue: base(), Children: ch})
	if len(links) != 9 {
		t.Fatalf("len(links) = %d", len(links))
	}
	if strings.Contains(out, "[10]") || !strings.Contains(out, "c-10") {
		t.Errorf("10-й ребёнок выведен неверно:\n%s", out)
	}
}

func TestParentMissingFromLookup(t *testing.T) {
	is := base()
	is.Parent = "x-9"
	out, links := render(Input{Issue: is, Lookup: func(string) (*bd.Issue, bool) { return nil, false }})
	if !strings.Contains(out, "[1] x-9") || len(links) == 0 || links[0] != "x-9" {
		t.Errorf("out=%s links=%v", out, links)
	}
}

func TestComments(t *testing.T) {
	out, _ := render(Input{Issue: base(), Comments: []bd.Comment{{Author: "A", Text: "hi", CreatedAt: ts("2026-09-23T10:00:00Z")}}})
	for _, s := range []string{"Comments", "A  2026-09-23 10:00", "▏ hi"} {
		if !strings.Contains(out, s) {
			t.Errorf("нет %q:\n%s", s, out)
		}
	}
}

func TestNotice(t *testing.T) {
	out, _ := render(Input{Issue: base(), Notice: "не удалось загрузить связи: boom"})
	if !strings.Contains(out, "⚠ не удалось загрузить связи: boom") {
		t.Errorf("нет notice:\n%s", out)
	}
}

func TestCleanStripsEscapes(t *testing.T) {
	if got := Clean("a\x1b[31mb\x9bc\nd\te"); got != "a[31mbc\nd\te" {
		t.Errorf("Clean = %q", got)
	}
}

func TestRenderStripsControlFromTitle(t *testing.T) {
	is := base()
	is.Title = "x\x1b]0;pwn\x07y"
	out, _ := render(Input{Issue: is})
	if strings.ContainsAny(out, "\x1b\x07") {
		t.Errorf("управляющие символы в %q", out)
	}
}

func TestRenderStripsControlFromDescription(t *testing.T) {
	is := base()
	is.Description = "x\x1b]0;pwn\x07y"
	out, _ := render(Input{Issue: is})
	if strings.ContainsAny(out, "\x1b\x07") {
		t.Errorf("управляющие символы в %q", out)
	}
}

func TestMarkdownEntityEscapesStripped(t *testing.T) {
	const evil = "a &#27;]0;x&#7; b &#x1b;[2J c `code &#27;]0;y&#7;`"
	cases := map[string]Input{}
	d := base()
	d.Description = evil
	cases["description"] = Input{Issue: d}
	n := base()
	n.Notes = evil
	cases["notes"] = Input{Issue: n}
	cases["comment"] = Input{Issue: base(), Comments: []bd.Comment{{Author: "A", Text: evil, CreatedAt: ts("2026-09-23T10:00:00Z")}}}
	for name, in := range cases {
		out, _ := render(in)
		for _, bad := range []string{"\x1b]", "\x07", "\x1b[2J"} {
			if strings.Contains(out, bad) {
				t.Errorf("%s: найдено %q в %q", name, bad, out)
			}
		}
	}
}

func TestSafeANSIKeepsSGR(t *testing.T) {
	in := "\x1b[1;31mX\x1b[0m"
	if got := safeANSI(in); got != in {
		t.Errorf("safeANSI = %q", got)
	}
}

// ST (ESC \) считается завершителем OSC и удаляется вместе с ним.
func TestSafeANSIDropsOSCAndCSI(t *testing.T) {
	if got := safeANSI("a\x1b]0;t\x07b\x1b[2Jc\x1b\\d"); got != "abcd" {
		t.Errorf("safeANSI = %q", got)
	}
	if got := safeANSI("a\x1b]0;t\x1b\\b"); got != "ab" {
		t.Errorf("OSC+ST: %q", got)
	}
}

// Длинные заголовок, метки, даты и пометка переносятся по ширине панели, а не обрезаются:
// последнее слово заголовка видно, ни одна строка не шире Width.
func TestLongHeaderWraps(t *testing.T) {
	is := base()
	is.Title = strings.Repeat("длинное слово ", 12) + "финал"
	if n := len([]rune(is.Title)); n < 170 {
		t.Fatalf("заголовок короткий: %d", n)
	}
	is.Labels = []string{strings.Repeat("метка", 3), strings.Repeat("x", 70), "последняя-метка"}
	is.Assignee = strings.Repeat("исполнитель ", 4) + "конец"
	c := ts("2026-09-24T10:00:00Z")
	is.ClosedAt = &c
	is.CloseReason = strings.Repeat("причина ", 8) + "итог"
	out, _ := Render(Input{Issue: is, Width: 60, Style: "notty",
		Notice: strings.Repeat("ошибка ", 10) + "хвост"})
	for _, want := range []string{"финал", "последняя-метка", "конец", "итог", "хвост"} {
		if !strings.Contains(out, want) {
			t.Errorf("нет %q в:\n%s", want, out)
		}
	}
	for i, l := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(l); w > 60 {
			t.Errorf("строка %d шириной %d > 60: %q", i, w, l)
		}
	}
}

// wufDescription — описание длинной задачи из тестового проекта: одна кириллическая строка с дефисами,
// на которой карточка ломала строки дважды.
const wufDescription = "Пользователь работает в Terminal.app, Ghostty и cmux (не iTerm2). Ghostty/cmux ставят TERM=xterm-ghostty; на серверах terminfo нет → ломаются backspace/clear/vim. Делаем как Ghostty ssh-env: ssh с TERM=xterm-256color для редких TERM, кроме хостов из ghostty +ssh-cache; CON_SSH_TERM — явное значение. Проверить UI в эмуляции трёх терминалов (Terminal.app не отвечает на OSC 11). Документы: убрать iTerm2."

// Абзац переносится один раз: ни одна строка не шире Width, и за короткой строкой
// не идёт слово, которое на ней помещалось (glamour держит справа поле в 2 колонки).
func TestDescriptionWrapsOnce(t *testing.T) {
	squash := func(s string) string { return strings.Join(strings.Fields(s), "") }
	for _, style := range []string{"notty", "dark"} {
		for _, width := range []int{77, 50} {
			t.Run(fmt.Sprintf("%s/%d", style, width), func(t *testing.T) {
				is := base()
				is.Description = wufDescription
				out, _ := Render(Input{Issue: is, Width: width, Style: style})
				lines := strings.Split(ansi.Strip(out), "\n")
				start := -1
				for i, l := range lines {
					if w := ansi.StringWidth(l); w > width {
						t.Errorf("строка %d шириной %d > %d: %q", i, w, width, l)
					}
					if strings.HasPrefix(l, "Description ─") {
						start = i + 1
					}
				}
				if start < 0 {
					t.Fatalf("нет секции Description:\n%s", out)
				}
				para := lines[start:]
				for len(para) > 0 && strings.TrimSpace(para[0]) == "" {
					para = para[1:]
				}
				if got := squash(strings.Join(para, " ")); got != squash(wufDescription) {
					t.Fatalf("текст абзаца искажён:\n%s", strings.Join(para, "\n"))
				}
				limit := width - 2
				for i := 0; i+1 < len(para); i++ {
					cur := strings.TrimRight(para[i], " ")
					next := strings.Fields(para[i+1])
					if len(next) == 0 {
						continue
					}
					w := ansi.StringWidth(cur)
					if w < width/2 && w+1+ansi.StringWidth(next[0]) <= limit {
						t.Errorf("строка %d недозаполнена (%d колонок), хотя %q помещалось:\n%s",
							i, w, next[0], strings.Join(para, "\n"))
					}
				}
			})
		}
	}
}

// Пробелы в конце строки убираются и между SGR, сами последовательности остаются.
func TestTrimRightSpaces(t *testing.T) {
	for in, want := range map[string]string{
		"ab  ":                       "ab",
		"a\x1b[31m b \x1b[0m":        "a\x1b[31m b\x1b[0m",
		"a \x1b[31m \x1b[0m \x1b[1m": "a\x1b[31m\x1b[0m\x1b[1m",
		"  ":                         "",
		"\x1b[31m x\x1b[0m":          "\x1b[31m x\x1b[0m",
		"кириллица \x1b[38;5;252m\x1b[0m": "кириллица\x1b[38;5;252m\x1b[0m",
	} {
		if got := trimRightSpaces(in); got != want {
			t.Errorf("trimRightSpaces(%q) = %q, want %q", in, got, want)
		}
	}
}

// tableMD — таблица в 4 колонки шире любой панели.
const tableMD = "Перед таблицей.\n\n" +
	"| Колонка один | Колонка два с длинным заголовком | Три | Четыре |\n" +
	"|---|:---|---|---|\n" +
	"| значение довольно длинное | ещё одно очень длинное значение в ячейке | x | yyyyyyyyyyyy |\n" +
	"| b | c | d | e |\n\nПосле таблицы."

// tablePipelessMD — та же таблица в форме GFM без крайних «|».
const tablePipelessMD = "Перед таблицей.\n\n" +
	"Колонка один | Колонка два с длинным заголовком | Три | Четыре\n" +
	"--- | :--- | --- | ---\n" +
	"значение довольно длинное | ещё одно очень длинное значение в ячейке | x | yyyyyyyyyyyy\n" +
	"b | c | d | e\n\nПосле таблицы."

// Таблица вписывается в панель: строки не шире Width, разделитель шапки — одна строка,
// вертикальные разделители колонок стоят на одних и тех же колонках во всех строках.
// Так же и таблица без крайних «|».
func TestCardTableFitsWidth(t *testing.T) {
	for _, md := range []string{tableMD, tablePipelessMD} {
		testTableFits(t, md)
	}
}

func testTableFits(t *testing.T, md string) {
	t.Helper()
	for _, style := range []string{"dark", "notty"} {
		sep, cross := "│", "┼"
		if style == "notty" {
			sep, cross = "|", "|"
		}
		for _, w := range []int{74, 44} {
			is := base()
			is.Description = md
			out, _ := Render(Input{Issue: is, Width: w, Style: style})
			ls := lines(out)
			var rule []int
			var cols []int
			for i, l := range ls {
				if lw := ansi.StringWidth(l); lw > w {
					t.Errorf("%s/%d: строка %d шириной %d: %q", style, w, i, lw, l)
				}
				// Разделитель шапки: только линии и перекрестья.
				if strings.Contains(l, "---") || strings.Contains(l, "───") {
					if strings.Trim(strings.TrimSpace(l), "-─|┼:") == "" {
						rule = append(rule, i)
					}
					continue
				}
				if !strings.Contains(l, sep) {
					continue
				}
				var pos []int
				x := 0
				for _, r := range l {
					if string(r) == sep {
						pos = append(pos, x)
					}
					x += ansi.StringWidth(string(r))
				}
				if cols == nil {
					cols = pos
				} else if !slices.Equal(pos, cols) {
					t.Errorf("%s/%d: колонки строки %d %v, у шапки %v:\n%s", style, w, i, pos, cols, strings.Join(ls, "\n"))
				}
			}
			if len(rule) != 1 {
				t.Errorf("%s/%d: строк-разделителей %d, want 1:\n%s", style, w, len(rule), strings.Join(ls, "\n"))
			} else if n := strings.Count(ls[rule[0]], cross); n != 3 {
				t.Errorf("%s/%d: в разделителе %d перекрестий: %q", style, w, n, ls[rule[0]])
			}
			if len(cols) != 3 {
				t.Errorf("%s/%d: колонок-разделителей %v:\n%s", style, w, cols, strings.Join(ls, "\n"))
			}
			if !strings.Contains(ansi.Strip(out), "После таблицы.") {
				t.Errorf("%s/%d: текст после таблицы пропал", style, w)
			}
		}
	}
}

// Шапка карточки переносится только по пробелам: даты и ID не рвутся на дефисе,
// жёстко режутся лишь слова длиннее ширины.
func TestHeaderWrapsOnSpacesOnly(t *testing.T) {
	is := base()
	is.Title = "Задача demo-app-2e9.6 и acme-shop-fjpw " + strings.Repeat("ж", 30)
	is.CreatedAt, is.UpdatedAt = ts("0001-01-01T00:00:00Z"), ts("2026-09-23T11:00:00Z")
	for w := 14; w <= 30; w++ {
		out, _ := Render(Input{Issue: is, Width: w, Style: "notty"})
		ls := lines(out)
		for i, l := range ls {
			if lw := ansi.StringWidth(l); lw > w {
				t.Errorf("w=%d: строка %d шириной %d: %q", w, i, lw, l)
			}
		}
		head := strings.Join(ls[:slices.IndexFunc(ls, func(l string) bool { return strings.Contains(l, "IN PROGRESS") })], "\n")
		dates := strings.Join(ls, "\n")
		for _, want := range []string{"demo-app-2e9.6", "acme-shop-fjpw"} {
			if !strings.Contains(head, want) {
				t.Errorf("w=%d: ID %q разорван:\n%s", w, want, head)
			}
		}
		for _, want := range []string{"0001-01-01", "2026-09-23"} {
			if !strings.Contains(dates, want) {
				t.Errorf("w=%d: дата %q разорвана:\n%s", w, want, dates)
			}
		}
		if !strings.Contains(strings.ReplaceAll(head, "\n", ""), strings.Repeat("ж", 30)) {
			t.Errorf("w=%d: длинное слово потеряно:\n%s", w, head)
		}
	}
}

// Пункт списка продолжается под текстом, а не под маркером: •, нумерация, чекбокс.
func TestListItemHangingIndent(t *testing.T) {
	long := strings.Repeat("слово ", 20)
	is := base()
	is.Description = "- маркер " + long + "\n\n1. номер " + long + "\n\n- [ ] чекбокс " + long + "\n- [x] готово " + long
	for _, style := range []string{"notty", "dark"} {
		for _, w := range []int{40, 60} {
			out, _ := Render(Input{Issue: is, Width: w, Style: style})
			ls := lines(out)
			for _, c := range []struct{ marker, first string }{
				{"• ", "маркер"}, {"1. ", "номер"}, {"[ ] ", "чекбокс"}, {"] ", "готово"},
			} {
				i := slices.IndexFunc(ls, func(l string) bool { return strings.Contains(l, c.marker+c.first) })
				if i < 0 {
					t.Fatalf("%s/%d: нет пункта %q:\n%s", style, w, c.first, strings.Join(ls, "\n"))
				}
				col := strings.Index(ls[i], c.first)
				want := ansi.StringWidth(ls[i][:col])
				if ls[i+1] == "" || strings.Contains(ls[i+1], "] ") || strings.HasPrefix(strings.TrimSpace(ls[i+1]), "•") {
					t.Fatalf("%s/%d: пункт %q не перенёсся:\n%s", style, w, c.first, strings.Join(ls, "\n"))
				}
				if got := len(ls[i+1]) - len(strings.TrimLeft(ls[i+1], " ")); got != want {
					t.Errorf("%s/%d: продолжение %q с отступом %d, want %d:\n%s", style, w, c.first, got, want, strings.Join(ls[i:i+2], "\n"))
				}
			}
		}
	}
}

// hasTable: таблица с крайними «|» и без них; блок кода в ``` или ~~~ с похожими
// строками таблицей не считается; одиночное «---» под строкой с «|» — не разделитель.
func TestHasTable(t *testing.T) {
	for _, c := range []struct {
		md   string
		want bool
	}{
		{"| a | b |\n|---|---|\n| 1 | 2 |", true},
		{"| a |\n|---|", true},
		{"a | b\n--- | ---\n1 | 2", true},
		{"a | b\n:---|---:\n1 | 2", true},
		{"  a | b\n  --- | :---: |", true},
		{"текст\n\n```\na | b\n|---|---|\n```\n", false},
		{"~~~text\n| a | b |\n|---|---|\n~~~", false},
		{"````\n```\na | b\n--- | ---\n````", false},
		{"```\nкод\n```\na | b\n--- | ---", true},
		{"a | b\n---", false},
		{"заголовок\n--- | ---", false},
		{"просто текст\n\n- пункт", false},
	} {
		if got := hasTable(c.md); got != c.want {
			t.Errorf("hasTable(%q) = %v, want %v", c.md, got, c.want)
		}
	}
}

// Блок кода с таблицей внутри не переключает текст на путь таблиц: слово абзаца
// длиннее ширины переносится (wrapLine), а не режется «…», как на пути таблиц.
func TestFencedTableNotTablePath(t *testing.T) {
	is := base()
	for _, code := range []string{"```\na | b\n|---|---|\n```", "~~~\n| a | b |\n|---|---|\n~~~"} {
		is.Description = code + "\n\n" + strings.Repeat("ж", 60)
		out, _ := Render(Input{Issue: is, Width: 40, Style: "notty"})
		if strings.Contains(out, "…") || strings.Count(ansi.Strip(out), "ж") != 60 {
			t.Errorf("абзац после блока кода обрезан:\n%s", out)
		}
	}
}

// CleanLine — однострочная очистка для app и card: управляющие символы убраны,
// \n и \t стали пробелами.
func TestCleanLine(t *testing.T) {
	if got := CleanLine("a\x1b[31mb\nc\td\x07"); got != "a[31mb c d" {
		t.Errorf("CleanLine = %q", got)
	}
}

// Контраст текста бейджей в обеих темах: не ниже 3:1 (жирный текст), у IN PROGRESS —
// не ниже 4.5:1 (в светлой теме белый на 136 давал 3.3:1).
func TestBadgeContrast(t *testing.T) {
	for _, st := range []string{"open", "in_progress", "blocked", "deferred", "closed", "pinned"} {
		bg, fg := badgeColors(st)
		min := 3.0
		if st == "in_progress" {
			min = 4.5
		}
		for _, c := range []struct{ theme, fg, bg string }{{"light", fg.Light, bg.Light}, {"dark", fg.Dark, bg.Dark}} {
			if r := testcolor.Contrast(c.fg, c.bg); r < min {
				t.Errorf("%s/%s: %s на %s — %.2f:1, want ≥ %.1f", st, c.theme, c.fg, c.bg, r, min)
			}
		}
	}
}
