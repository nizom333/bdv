package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/card"
	"github.com/nizom333/bdv/internal/i18n"
	"github.com/nizom333/bdv/internal/tree"
)

// activeMark — знак активной панели перед заголовком её рамки. Цвет рамки в тестах
// (профиль Ascii) не виден, знак виден всегда.
const activeMark = "◆"

// blockedMark — знак заблокированной задачи: в дереве, верхней строке и карточке.
const blockedMark = card.BlockedMark

var (
	faintSt  = lipgloss.NewStyle().Faint(true)
	boldSt   = lipgloss.NewStyle().Bold(true)
	accentSt = lipgloss.NewStyle().Foreground(card.Accent)
	errorSt  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "196"})
	// Рамки: активная акцентного цвета, неактивная приглушённая.
	borderOn  = lipgloss.NewStyle().Foreground(card.Accent)
	borderOff = lipgloss.NewStyle().Foreground(card.Muted)
	// Выбранная строка дерева: цветной фон в активной панели, серый — в неактивной.
	// Фон подобран по контрасту WCAG с цветами строки (иконки статусов, P0/P1): прежний
	// 60 давал красному 196 лишь 1.5:1. Активная: тёмная — тёмно-синий 17 (196 — 4.5:1,
	// 208 — 7.5:1), светлая — 195 (160 — 5.1:1, 136 — 3.1:1). Неактивная: 236 (196 —
	// 3.3:1; у 237 было 2.8:1) и 254 (160 — 4.3:1).
	selOn = lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "195", Dark: "17"}).
		Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "255"})
	selOff = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"})
)

// pad добивает строку пробелами до ширины w.
func pad(s string, w int) string {
	if d := w - ansi.StringWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// fit обрезает строку до ширины w и добивает пробелами.
func fit(s string, w int) string { return pad(ansi.Truncate(s, w, "…"), w) }

// Наименьшее окно, в котором рисуются рамки; в меньшем — одна строка TooSmall.
const (
	minWidth  = 20
	minHeight = 6
)

// View рисует текущий экран.
func (m Model) View() string {
	if m.width > 0 && m.height > 0 && (m.width < minWidth || m.height < minHeight) {
		return ansi.Truncate(m.t.TooSmall, m.width, "")
	}
	if m.forest == nil {
		if m.help {
			// Справка с экрана без базы (init.go).
			return m.helpView()
		}
		if v, ok := m.initView(); ok {
			return v
		}
		if m.loadErr != nil {
			return m.errorView()
		}
		return m.loadingView()
	}
	if m.set.open {
		return m.settingsView()
	}
	if m.help {
		return m.helpView()
	}
	return m.mainView()
}

func (m Model) loadingView() string {
	if m.width <= 0 || m.height <= 0 {
		return m.t.Loading
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, m.t.Loading)
}

// errorView — экран ошибки первой загрузки с полным stderr.
func (m Model) errorView() string {
	text := card.Clean(errText(m.loadErr))
	if m.width > 0 {
		text = ansi.Wrap(text, m.width, "")
	}
	return strings.Join([]string{m.t.LoadError, "", text, "", m.t.ErrorHint}, "\n")
}

// mainView — верхняя строка, строка вкладок, панели в рамках (список вкладки |
// карточка), статус-строка. В узком режиме показывается одна панель на всю ширину:
// та, что в фокусе.
func (m Model) mainView() string {
	tw, cw, h := m.treeWidth(), m.cardWidth(), m.treeHeight()
	lines := make([]string, 0, m.height)
	lines = append(lines, m.topBar())
	if !m.commenting || h > 0 {
		// Поле комментария на всё тело скрывает панели, а с ними и строку вкладок.
		lines = append(lines, m.tabBar())
	}
	// Открытое поле комментария активно само, панели над ним — нет.
	on := !m.commenting
	switch {
	case m.commenting && h == 0:
		// Поле занимает всё тело, панели скрыты.
	case m.narrow() && m.focus == focusDetail:
		lines = append(lines, m.cardPane(cw, h, on)...)
	case m.narrow():
		lines = append(lines, m.treePane(tw, h, on)...)
	default:
		left := m.treePane(tw, h, on && m.focus == focusTree)
		right := m.cardPane(cw, h, on && m.focus == focusDetail)
		for i := range left {
			lines = append(lines, left[i]+right[i])
		}
	}
	if m.commenting {
		lines = append(lines, m.commentPane()...)
	}
	lines = append(lines, m.statusLine())
	return strings.Join(lines, "\n")
}

// frame обводит body скруглённой рамкой общей шириной w; заголовок title вписан
// в верхнюю границу, у активной панели перед ним activeMark.
func frame(title string, body []string, w int, active bool) []string {
	b := lipgloss.RoundedBorder()
	st, tst := borderOff, faintSt
	if active {
		st, tst = borderOn, accentSt.Bold(true)
		title = strings.TrimSpace(activeMark + " " + title)
	}
	inner := max(0, w-2)
	head := ""
	// «─ title » и хотя бы один «─» до уголка.
	if title != "" && inner > 4 {
		head = st.Render(b.Top+" ") + tst.Render(ansi.Truncate(title, inner-4, "…")) + " "
	}
	rest := max(0, inner-ansi.StringWidth(head))
	lines := make([]string, 0, len(body)+2)
	lines = append(lines, st.Render(b.TopLeft)+head+st.Render(strings.Repeat(b.Top, rest)+b.TopRight))
	left, right := st.Render(b.Left), st.Render(b.Right)
	for _, l := range body {
		lines = append(lines, left+fit(l, inner)+right)
	}
	lines = append(lines, st.Render(b.BottomLeft+strings.Repeat(b.Bottom, inner)+b.BottomRight))
	return lines
}

// treePane — левая панель активной вкладки в рамке шириной w (вместе с рамкой), h строк
// тела: дерево Tasks или список вкладки. Текст пустой панели (в том числе ошибка
// загрузки вкладки) переносится по ширине.
func (m Model) treePane(w, h int, focused bool) []string {
	inner := max(0, w-2)
	var empty []string
	if len(m.rows) == 0 {
		empty = strings.Split(ansi.Wrap(m.emptyText(), max(1, inner-1), ""), "\n")
	}
	body := make([]string, 0, h)
	for i := 0; i < h; i++ {
		line := ""
		if idx := m.offset + i; idx < len(m.rows) {
			line = m.tabLine(m.rows[idx], idx == m.cursor, focused, inner)
		} else if i < len(empty) {
			line = " " + empty[i]
		}
		body = append(body, line)
	}
	return frame(m.tabTitle(), body, w, focused)
}

// taskRows — число видимых строк-задач, без заголовков групп.
func (m Model) taskRows() int {
	n := 0
	for _, r := range m.rows {
		if !r.IsGroup {
			n++
		}
	}
	return n
}

// emptyText — текст пустого дерева: нет совпадений поиска, нет готовых задач (R), нет
// задач вовсе (или закрытые уже показаны — подсказка про c ни к чему), иначе нет открытых.
func (m Model) emptyText() string {
	if m.tab != tabTasks {
		return m.tabEmpty()
	}
	if q := strings.TrimSpace(m.opts.Query); q != "" {
		return m.t.NoMatches(card.CleanLine(q))
	}
	if m.opts.ReadyOnly {
		return m.t.NoReady
	}
	if m.opts.ShowClosed || m.forest == nil || m.forest.Len() == 0 {
		return m.t.NoIssues
	}
	return m.t.EmptyTree
}

// cardPane — карточка в рамке шириной w (вместе с рамкой), h строк тела; в заголовке
// рамки полный ID задачи или P<n> группы. Слева и справа поле в одну колонку.
func (m Model) cardPane(w, h int, focused bool) []string {
	title := m.cardTitleOf()
	tw := max(0, w-4)
	lines := strings.Split(m.vp.View(), "\n")
	body := make([]string, 0, h)
	for i := 0; i < h; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		body = append(body, " "+fit(line, tw)+" ")
	}
	return frame(title, body, w, focused)
}

// rowBase — фон строки левой панели: выбранная — selOn в активной панели и selOff
// в неактивной, прочие — без фона. Общий для дерева и строк всех вкладок.
func rowBase(selected, focused bool) lipgloss.Style {
	switch {
	case selected && focused:
		return selOn
	case selected:
		return selOff
	}
	return lipgloss.NewStyle()
}

// seg — кусок строки дерева со своим стилем.
type seg struct {
	text string
	st   lipgloss.Style
}

// row собирает строку из кусков: каждый кусок получает недостающие свойства base (фон
// выбранной строки, faint), строка обрезается по ширине w и добивается фоном base.
func row(base lipgloss.Style, w int, segs ...seg) string {
	var sb strings.Builder
	for _, s := range segs {
		sb.WriteString(s.st.Inherit(base).Render(s.text))
	}
	line := ansi.Truncate(sb.String(), w, "…")
	if d := w - ansi.StringWidth(line); d > 0 {
		line += base.Render(strings.Repeat(" ", d))
	}
	return line
}

// treeLine — строка дерева шириной w: отступ, маркер, цветная иконка статуса,
// приоритет (при группах — только если он не равен приоритету группы), короткий ID,
// у ждущей задачи (tree.Forest.Waiting) красный blockedMark, и заголовок. Заголовок группы жирный: маркер, P<n> в цвете приоритета и число
// видимых задач группы. Выбранная строка — на фоне во всю ширину, контекст приглушён.
func (m Model) treeLine(r tree.Row, selected, focused bool, w int) string {
	base := rowBase(selected, focused)
	marker := " "
	if r.HasChildren {
		marker = "▸"
		if r.Expanded {
			marker = "▾"
		}
	}
	plain := lipgloss.NewStyle()
	if r.IsGroup {
		return row(base.Bold(true), w,
			seg{" " + marker + " ", plain},
			seg{fmt.Sprintf("P%d", r.Group), card.PrioStyle(r.Group)},
			seg{fmt.Sprintf(" · %d", r.GroupCount), plain})
	}
	if r.Context {
		base = base.Faint(true)
	}
	// Общий префикс проекта в дереве не показывается; карточка, ссылки и y — с полным ID.
	id, title := card.CleanLine(strings.TrimPrefix(r.Issue.ID, m.prefix)), card.CleanLine(r.Issue.Title)
	segs := []seg{
		{" " + strings.Repeat("  ", r.Depth) + marker + " ", plain},
		{card.StatusIcon(r.Issue.Status), card.StatusStyle(r.Issue.Status)},
		{" ", plain},
	}
	// При группах приоритет уже в заголовке группы: P<n> только у задач с другим
	// приоритетом (подзадачи). Без групп — у каждой строки.
	if !m.opts.GroupByPriority || r.Issue.Priority != r.Group {
		segs = append(segs, seg{fmt.Sprintf("P%d", r.Issue.Priority), card.PrioStyle(r.Issue.Priority)}, seg{" ", plain})
	}
	// ID приглушён, кроме выбранной строки: faint на фоне выделения плохо читается.
	idSt := faintSt
	if selected {
		idSt = plain
	}
	segs = append(segs, seg{id, idSt})
	if m.forest.Waiting(r.Issue.ID) {
		// Ждущая задача (не закрыта, есть открытый блокер): красный ⊘ между ID и заголовком.
		segs = append(segs, seg{" " + blockedMark, card.StatusStyle("blocked")})
	}
	return row(base, w, append(segs, seg{" " + title, plain})...)
}

// topBar — верхняя строка: имя проекта, сводка статусов задач, видимых при текущих
// переключателях c и e (запрос и R не учитываются), и «⊘ N waiting» — сколько из них
// ждёт (tree.WaitingCount), если есть. Сводка служит и легендой иконок.
func (m Model) topBar() string {
	s := m.summary(tree.StatusCounts(m.forest, m.opts), true)
	if n := tree.WaitingCount(m.forest, m.opts); n > 0 {
		s += " · " + card.StatusStyle("blocked").Render(blockedMark) + " " + m.t.Waiting(n)
	}
	if p := card.CleanLine(m.o.Project); p != "" {
		s = accentSt.Bold(true).Render(p) + "  " + s
	}
	return fit(s, m.width)
}

// statusLine — слева по порядку: строка ввода, подтверждение копирования, ошибка
// обновления (⚠, красным) или время последнего успешного обновления, режимы словами,
// фильтр, сообщение; справа подсказки клавиш. Короткоживущее идёт раньше, чтобы его
// не обрезало. Если места мало, из подсказок остаётся только справка. При открытом
// поле комментария подсказок нет: клавиши идут в поле, у него своя подсказка.
// Подсказка «m» видна, только когда выбрана задача (не заголовок группы).
func (m Model) statusLine() string {
	var parts []string
	q := strings.TrimSpace(m.opts.Query)
	if m.typing {
		// Строка ввода первой, чтобы её не обрезало.
		parts = append(parts, m.input.View())
	}
	if m.toast != "" {
		parts = append(parts, m.toast)
	}
	switch {
	case m.refreshErr != "":
		parts = append(parts, errorSt.Render("⚠ "+m.t.RefreshFailed(m.refreshErr)))
	case !m.lastOK.IsZero():
		parts = append(parts, m.t.Updated(m.lastOK.Format("15:04:05")))
	}
	// Режимы — только там, где они действуют: c — во вкладках из леса, e, p и R — в Tasks.
	if m.opts.ShowClosed && m.tab.issues() {
		parts = append(parts, m.t.ClosedToggle)
	}
	if m.tab == tabTasks {
		if m.opts.ShowEvents {
			parts = append(parts, m.t.EventsToggle)
		}
		if !m.opts.GroupByPriority {
			parts = append(parts, m.t.NoGroupsToggle)
		}
		if m.opts.ReadyOnly {
			parts = append(parts, m.t.ReadyToggle)
		}
	}
	if m.mouseOff {
		parts = append(parts, m.t.MouseOffToggle)
	}
	if !m.typing && q != "" {
		parts = append(parts, m.t.Filter(card.CleanLine(q)))
	}
	if m.status != "" {
		parts = append(parts, m.status)
	}
	left := strings.Join(parts, " · ")
	if m.commenting {
		return fit(left, m.width)
	}
	// Подсказки своей вкладки (hints): вкладки, поиск, комментарий, справка; «m» —
	// только при выбранной задаче. Берётся первый вариант, который влезает.
	opts := m.hints()
	right := opts[len(opts)-1]
	for _, r := range opts {
		if ansi.StringWidth(left)+1+ansi.StringWidth(r) <= m.width {
			right = r
			break
		}
	}
	lw := m.width - ansi.StringWidth(right) - 1
	if lw <= 0 {
		return fit(left, m.width)
	}
	return fit(left, lw) + " " + faintSt.Render(right)
}

// --- Карточка ---

// narrow — узкий режим: одна панель на всю ширину.
func (m Model) narrow() bool { return m.width < 80 }

// treeWidth — ширина дерева с рамкой: 40% экрана в пределах [30, 60], в узком режиме вся ширина.
func (m Model) treeWidth() int {
	if m.narrow() {
		return m.width
	}
	return max(30, min(m.width*40/100, 60))
}

// cardWidth — ширина карточки с рамкой: окно за вычетом дерева, в узком режиме вся ширина.
func (m Model) cardWidth() int {
	if m.narrow() {
		return m.width
	}
	return max(0, m.width-m.treeWidth())
}

// cardText — ширина текста карточки: без рамки и полей в одну колонку с каждой стороны.
func (m Model) cardText() int { return max(0, m.cardWidth()-4) }

// --- Справка ---

// helpGap — пробелы между колонками справки; сами секции — в каталоге (HelpLeft, HelpRight).
const helpGap = 3

// helpColumn — строки секций: заголовок жирным акцентным, клавиши с отступом в два
// пробела, колонка действий — через пробел после самой длинной клавиши колонки.
// sep добавляет пустую строку между секциями.
func helpColumn(secs []i18n.HelpSection, sep bool) []string {
	kw := 0
	for _, s := range secs {
		for _, k := range s.Keys {
			kw = max(kw, ansi.StringWidth(k.Key))
		}
	}
	var lines []string
	for i, s := range secs {
		if i > 0 && sep {
			lines = append(lines, "")
		}
		lines = append(lines, accentSt.Bold(true).Render(s.Title))
		for _, k := range s.Keys {
			lines = append(lines, "  "+pad(k.Key, kw+1)+k.Action)
		}
	}
	return lines
}

func maxWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, ansi.StringWidth(l))
	}
	return w
}

// helpBody — тело справки каталога t под ширину w и высоту h (0 — неизвестны): две
// колонки, если влезают по ширине, иначе одна плотная; под ними HelpNote. Варианты от
// просторного к плотному: пустые строки между секциями, сверху и перед заметкой; затем
// без пустой строки сверху, без пустой строки перед заметкой; затем то же без пустых
// строк между секциями. Выбирается первый, что входит в высоту h; не входит ни один —
// последний, и справка прокручивается.
func helpBody(t *i18n.Catalog, w, h int) []string {
	note := []string{t.HelpNote}
	if w > 0 && ansi.StringWidth(t.HelpNote) > w {
		note = strings.Split(ansi.Wrap(t.HelpNote, w, ""), "\n")
	}
	left, right := helpColumn(t.HelpLeft, true), helpColumn(t.HelpRight, true)
	if w > 0 && w < maxWidth(left)+helpGap+maxWidth(right) {
		lines := append(helpColumn(t.HelpLeft, false), helpColumn(t.HelpRight, false)...)
		return append(lines, note...)
	}
	var body []string
	for _, sep := range []bool{true, false} {
		left, right := helpColumn(t.HelpLeft, sep), helpColumn(t.HelpRight, sep)
		lw := maxWidth(left)
		var cols []string
		for i := 0; i < max(len(left), len(right)); i++ {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			cols = append(cols, strings.TrimRight(pad(l, lw+helpGap)+r, " "))
		}
		// air: 2 — пустые строки сверху и перед заметкой, 1 — только перед заметкой, 0 — нет.
		for air := 2; air >= 0; air-- {
			body = nil
			if air == 2 {
				body = append(body, "")
			}
			body = append(body, cols...)
			if air >= 1 {
				body = append(body, "")
			}
			body = append(body, note...)
			if h <= 0 || len(body) <= h {
				return body
			}
		}
	}
	return body
}

// helpWindow — тело справки, высота окна внутри рамки и наибольший сдвиг прокрутки.
// Тело с полем в одну колонку слева.
func (m Model) helpWindow() (body []string, page, maxOff int) {
	w := 0
	if m.width > 0 {
		w = max(1, m.width-3)
	}
	page = 0
	if m.height > 0 {
		page = max(1, m.height-2)
	}
	for _, l := range helpBody(m.t, w, page) {
		body = append(body, " "+l)
	}
	if page == 0 {
		page = len(body)
	}
	return body, page, max(0, len(body)-page)
}

// helpView — справка в рамке на весь терминал; не влезающее тело прокручивается.
func (m Model) helpView() string {
	body, page, maxOff := m.helpWindow()
	title := m.t.HelpTitle
	if maxOff > 0 {
		title = m.t.HelpScroll
	}
	off := min(m.helpOff, maxOff)
	shown := slices.Clone(body[off:min(len(body), off+page)])
	if m.width <= 0 || m.height <= 0 {
		return strings.Join(append([]string{title}, shown...), "\n")
	}
	for len(shown) < page {
		shown = append(shown, "")
	}
	return strings.Join(frame(title, shown, m.width, true), "\n")
}
