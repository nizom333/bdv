package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/card"
	"github.com/nizom333/bdv/internal/tree"
)

// --- Вкладки (v1.4): Tasks, Specs, Decisions, Memory, Formulas ---
//
// Инвариант состояния. Поля дерева в Model — rows, cursor, offset, selected, opts.Query
// и preSearch — всегда принадлежат активной вкладке m.tab. Состояние неактивных вкладок
// лежит в m.views[t] и переносится туда и обратно только в enterTab. Пока активна другая
// вкладка, состояние Tasks читается так: запрос — views[tabTasks].query (tasksOpts),
// выбор и курсор — views[tabTasks], лес на момент ухода — views[tabTasks].forest.
// Общее для всех вкладок: переключатели opts (кроме Query), раскрытие m.exp (оно есть
// только у Tasks), фокус и кеши карточек.

// tab — вкладка главного экрана. Одна переменная вместо флагов режимов: клавиши
// и отрисовка спрашивают m.tab.
type tab int

const (
	tabTasks tab = iota
	tabSpecs
	tabDecisions
	tabMemory
	tabFormulas
	tabCount // число вкладок, не вкладка
)

// String — имя вкладки для Model.Tab: "tasks", "specs", "decisions", "memory", "formulas".
func (t tab) String() string {
	switch t {
	case tabSpecs:
		return "specs"
	case tabDecisions:
		return "decisions"
	case tabMemory:
		return "memory"
	case tabFormulas:
		return "formulas"
	}
	return "tasks"
}

// issues — строки вкладки — задачи леса (Tasks, Specs, Decisions): у них карточка задачи,
// комментарии, копирование и bd show. Строки Memory и Formulas — записи вне леса.
func (t tab) issues() bool { return t == tabTasks || t == tabSpecs || t == tabDecisions }

// lazy — вкладка загружается своим чтением bd при первом входе и по r.
func (t tab) lazy() bool { return t == tabMemory || t == tabFormulas }

// member — задача входит в плоскую вкладку t: Specs — эпик, но не шаблон (шаблоны —
// во вкладке Formulas), Decisions — задача типа decision. У прочих вкладок плоского
// списка из леса нет.
func (t tab) member(is *bd.Issue) bool {
	switch t {
	case tabSpecs:
		return is.IssueType == "epic" && !bd.IsTemplate(*is)
	case tabDecisions:
		return is.IssueType == "decision"
	}
	return false
}

// tabView — состояние неактивной вкладки: курсор, сдвиг дерева, выбор и поиск.
// Раскрытие веток и групп живёт в m.exp: оно есть только у Tasks.
type tabView struct {
	cursor, offset int
	selected       string
	query          string
	preSearch      string
	// forest — лес на момент ухода из вкладки: если выбор за это время исчез из списка,
	// замена ищется по нему (tree.Nearest знает соседей и родителя исчезнувшей задачи).
	forest *tree.Forest
}

// Ключи строк Memory и Formulas: префикс отделяет их от ID задач, чтобы поиск задачи
// в лесу (bd show, копирование, комментарий) по ним никогда не срабатывал.
const (
	memoryKey   = "memory:"
	formulaKey  = "formula:"
	templateKey = "template:"
)

// entry — строка вкладки Memory или Formulas: ключ строки, имя слева, пометка справа
// (formula/template), строка meta и текст карточки (markdown).
type entry struct {
	key, name, kind, meta, text string
}

// lazyTab — данные вкладки Memory или Formulas: загружаются при первом входе и по r.
type lazyTab struct {
	loaded, loading bool
	err             string // очищенная первая строка ошибки последней загрузки; "" — нет
	items           []entry
	// row — носители строк для дерева: tree.Row требует *bd.Issue, у записи его нет,
	// поэтому строка несёт задачу-пустышку только с ключом и именем. В лес она не попадает.
	row []bd.Issue
	gen int // растёт с каждой загрузкой: часть ключа кеша рендера карточки
}

// Сообщения загрузки вкладок.
type (
	memoriesMsg struct {
		items []bd.Memory
		err   error
	}
	formulasMsg struct {
		formulas  []bd.Formula
		templates []bd.Issue
		err       error
	}
)

// Tab — активная вкладка: "tasks", "specs", "decisions", "memory" или "formulas".
func (m Model) Tab() string { return m.tab.String() }

// lazyOf — данные ленивой вкладки t; nil для вкладок из леса.
func (m *Model) lazyOf(t tab) *lazyTab {
	switch t {
	case tabMemory:
		return &m.mem
	case tabFormulas:
		return &m.forms
	}
	return nil
}

// enterTab сохраняет состояние активной вкладки и восстанавливает состояние t; строки
// не пересчитывает и выбор не трогает — это делают switchTab или jump.
func (m *Model) enterTab(t tab) {
	m.views[m.tab] = tabView{m.cursor, m.offset, m.selected, m.opts.Query, m.preSearch, m.forest}
	m.tab = t
	v := m.views[t]
	m.cursor, m.offset, m.selected, m.opts.Query, m.preSearch = v.cursor, v.offset, v.selected, v.query, v.preSearch
	m.input.SetValue(v.query)
	m.vp.SetYOffset(0)
}

// switchTab — клавиши [ и ]: соседняя вкладка по кругу (d = -1 или +1). Ленивая
// вкладка при первом входе загружается. Исчезнувший за время отсутствия выбор
// заменяется по лесу на момент ухода (tabView.forest). Если во вкладке нечего выбрать,
// фокус возвращается к списку: в узком режиме иначе осталась бы пустая рамка карточки.
func (m *Model) switchTab(d int) tea.Cmd {
	m.enterTab(tab((int(m.tab) + d + int(tabCount)) % int(tabCount)))
	var load tea.Cmd
	if l := m.lazyOf(m.tab); l != nil && !l.loaded {
		load = m.loadTab(m.tab)
	}
	m.computeRows()
	f := m.views[m.tab].forest
	if f == nil {
		f = m.forest
	}
	cmd := m.selectIndex(m.targetIndex(f), true)
	if m.selected == "" {
		m.focus = focusTree
	}
	return tea.Batch(load, cmd)
}

// loadTab запускает чтение ленивой вкладки t; повторно, пока идёт прежнее, — нет.
// Предел — DetailTimeout, контекст корневой.
func (m *Model) loadTab(t tab) tea.Cmd {
	l := m.lazyOf(t)
	if l == nil || l.loading {
		return nil
	}
	l.loading = true
	src, root, d, tc := m.o.Source, m.ctx, m.o.DetailTimeout, m.t
	if t == tabMemory {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(root, d)
			defer cancel()
			items, err := src.Memories(ctx)
			return memoriesMsg{items: items, err: timeoutErr(ctx, err, d, tc)}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(root, d)
		defer cancel()
		fs, ferr := src.Formulas(ctx)
		ts, terr := src.Templates(ctx)
		return formulasMsg{formulas: fs, templates: ts, err: timeoutErr(ctx, errors.Join(ferr, terr), d, tc)}
	}
}

// onMemories — ответ bd memories: записи по ключам, текст — markdown.
func (m *Model) onMemories(msg memoriesMsg) tea.Cmd {
	var items []entry
	for _, it := range msg.items {
		items = append(items, entry{key: memoryKey + it.Key, name: it.Key, text: it.Text})
	}
	return m.applyLazy(tabMemory, items, msg.err)
}

// onFormulas — ответ bd formula list и bd list --include-templates: сначала формулы
// по имени, затем шаблоны по заголовку.
func (m *Model) onFormulas(msg formulasMsg) tea.Cmd {
	var fs, ts []entry
	for _, f := range msg.formulas {
		meta := joinMeta(m.t.KindFormula, f.Type, f.Source)
		fs = append(fs, entry{key: formulaKey + f.Name, name: f.Name, kind: m.t.KindFormula, meta: meta, text: f.Description})
	}
	for _, is := range msg.templates {
		meta := joinMeta(m.t.KindTemplate, is.ID)
		ts = append(ts, entry{key: templateKey + is.ID, name: is.Title, kind: m.t.KindTemplate, meta: meta, text: is.Description})
	}
	byName := func(s []entry) {
		sort.SliceStable(s, func(i, j int) bool { return card.CleanLine(s[i].name) < card.CleanLine(s[j].name) })
	}
	byName(fs)
	byName(ts)
	return m.applyLazy(tabFormulas, append(fs, ts...), msg.err)
}

// joinMeta — непустые части строки meta через « · ».
func joinMeta(parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " · ")
}

// applyLazy кладёт загруженные записи вкладки t. Ошибка очищает список: в панели
// видна только она, r повторяет чтение. Активная вкладка перерисовывается сразу.
func (m *Model) applyLazy(t tab, items []entry, err error) tea.Cmd {
	l := m.lazyOf(t)
	l.loading, l.loaded = false, true
	l.gen++
	l.err = ""
	if err != nil {
		l.err = card.CleanLine(firstLine(err))
		items = nil
	}
	l.items = items
	l.row = make([]bd.Issue, len(items))
	for i, e := range items {
		l.row[i] = bd.Issue{ID: e.key, Title: e.name}
	}
	if m.tab != t || m.forest == nil {
		return nil
	}
	m.computeRows()
	return m.selectIndex(m.targetIndex(m.forest), true)
}

// tabRows — строки вкладки t для текущих фильтров (Tasks — дерево, Specs и Decisions —
// плоские списки леса, Memory и Formulas — записи).
func (m *Model) tabRows(t tab) []tree.Row {
	switch t {
	case tabSpecs, tabDecisions:
		return tree.Flat(m.forest, m.opts, t.member)
	case tabMemory, tabFormulas:
		l := m.lazyOf(t)
		rows := make([]tree.Row, len(l.row))
		for i := range l.row {
			rows[i] = tree.Row{Issue: &l.row[i]}
		}
		return rows
	}
	return tree.Rows(m.forest, m.opts, m.exp)
}

// entryOf — запись выбранной строки ленивой вкладки.
func (m Model) entryOf(key string) (entry, bool) {
	l := m.lazyOf(m.tab)
	if l == nil {
		return entry{}, false
	}
	for _, e := range l.items {
		if e.key == key {
			return e, true
		}
	}
	return entry{}, false
}

// flatTarget — строка выбора плоской вкладки после пересчёта: та же запись, иначе
// строка на месте курсора (ближайшая снизу), иначе первая.
func (m Model) flatTarget() int {
	if i := m.indexOf(m.selected); i >= 0 {
		return i
	}
	return max(0, min(m.cursor, len(m.rows)-1))
}

// tabCountOf — число в строке вкладок: спеки и решения при текущем c (без поиска),
// записи загруженной ленивой вкладки; -1 — число неизвестно (не загружено) или не нужно.
func (m Model) tabCountOf(t tab) int {
	switch t {
	case tabSpecs, tabDecisions:
		return m.forest.Count(func(is *bd.Issue) bool { return t.member(is) && tree.Passes(is, m.opts) })
	case tabMemory, tabFormulas:
		l := m.lazyOf(t)
		if !l.loaded || l.err != "" {
			return -1
		}
		return len(l.items)
	}
	return -1
}

// tabBar — строка вкладок «Tasks │ Specs 2 │ Decisions 1 │ Memory │ Formulas»: активная
// жирная на фоне выделения, пустые (0) приглушены.
func (m Model) tabBar() string {
	names := []string{m.t.TabTasks, m.t.TabSpecs, m.t.TabDecisions, m.t.TabMemory, m.t.TabFormulas}
	sep := faintSt.Render(" │ ")
	var parts []string
	for i, name := range names {
		t := tab(i)
		n := m.tabCountOf(t)
		if n >= 0 {
			name += fmt.Sprintf(" %d", n)
		}
		st := lipgloss.NewStyle()
		switch {
		case t == m.tab:
			st = selOn.Bold(true)
		case n == 0:
			st = faintSt
		}
		parts = append(parts, st.Render(name))
	}
	return fit(" "+strings.Join(parts, sep), m.width)
}

// tabTitle — заголовок рамки левой панели: «Tasks (N)», «Specs (N)» …
func (m Model) tabTitle() string {
	n := m.taskRows()
	switch m.tab {
	case tabSpecs:
		return m.t.SpecsTitle(n)
	case tabDecisions:
		return m.t.DecisionsTitle(n)
	case tabMemory:
		return m.t.MemoryTitle(n)
	case tabFormulas:
		return m.t.FormulasTitle(n)
	}
	return m.t.Tasks(n)
}

// flatLine — строка плоской вкладки шириной w: иконка статуса, P<n> (withPrio), короткий
// ID, ⊘ у ждущей, заголовок; right (прогресс спеки) прижат к правому краю.
func (m Model) flatLine(r tree.Row, selected, focused bool, w int, withPrio bool, right string) string {
	base := rowBase(selected, focused)
	plain := lipgloss.NewStyle()
	idSt := faintSt
	if selected {
		idSt = plain
	}
	segs := []seg{{" ", plain}, {card.StatusIcon(r.Issue.Status), card.StatusStyle(r.Issue.Status)}, {" ", plain}}
	if withPrio {
		segs = append(segs, seg{fmt.Sprintf("P%d", r.Issue.Priority), card.PrioStyle(r.Issue.Priority)}, seg{" ", plain})
	}
	segs = append(segs, seg{card.CleanLine(strings.TrimPrefix(r.Issue.ID, m.prefix)), idSt})
	if m.forest.Waiting(r.Issue.ID) {
		segs = append(segs, seg{" " + blockedMark, card.StatusStyle("blocked")})
	}
	segs = append(segs, seg{" " + card.CleanLine(r.Issue.Title), plain})
	return withRight(base, w, segs, right)
}

// entryLine — строка записи Memory или Formulas: имя и приглушённая пометка вида справа.
func (m Model) entryLine(r tree.Row, selected, focused bool, w int) string {
	base := rowBase(selected, focused)
	right := ""
	if e, ok := m.entryOf(r.Issue.ID); ok && e.kind != "" {
		right = faintSt.Render(card.CleanLine(e.kind))
	}
	return withRight(base, w, []seg{{" " + card.CleanLine(r.Issue.Title), lipgloss.NewStyle()}}, right)
}

// withRight — строка row шириной w, у которой готовый текст right прижат вправо через
// пробел; если места нет, right не показывается.
func withRight(base lipgloss.Style, w int, segs []seg, right string) string {
	rw := ansi.StringWidth(right)
	if right == "" || w-rw-1 < 8 {
		return row(base, w, segs...)
	}
	return row(base, w-rw-1, segs...) + base.Render(" ") + base.Render(right)
}

// tabLine — строка дерева активной вкладки.
func (m Model) tabLine(r tree.Row, selected, focused bool, w int) string {
	switch m.tab {
	case tabSpecs:
		return m.flatLine(r, selected, focused, w, false, card.ProgressBar(m.forest.Progress(r.Issue.ID)))
	case tabDecisions:
		return m.flatLine(r, selected, focused, w, true, "")
	case tabMemory, tabFormulas:
		return m.entryLine(r, selected, focused, w)
	}
	return m.treeLine(r, selected, focused, w)
}

// tabEmpty — текст пустой панели вкладки: загрузка, ошибка, нет совпадений, нет записей.
func (m Model) tabEmpty() string {
	if l := m.lazyOf(m.tab); l != nil {
		switch {
		case l.loading && len(l.items) == 0:
			return m.t.TabLoading
		case l.err != "" && m.tab == tabMemory:
			return "⚠ " + m.t.MemoryFailed(l.err)
		case l.err != "":
			return "⚠ " + m.t.FormulasFailed(l.err)
		case m.tab == tabMemory:
			return m.t.NoMemories
		}
		return m.t.NoFormulas
	}
	if q := strings.TrimSpace(m.opts.Query); q != "" {
		return m.t.NoMatches(card.CleanLine(q))
	}
	none, noOpen := m.t.NoSpecs, m.t.NoOpenSpecs
	if m.tab == tabDecisions {
		none, noOpen = m.t.NoDecisions, m.t.NoOpenDecisions
	}
	if !m.opts.ShowClosed && m.forest.Count(m.tab.member) > 0 {
		return noOpen
	}
	return none
}

// specInput дополняет вход карточки спеки: прогресс в чипах, дети без событий —
// секцией Tickets.
func (m *Model) specInput(in *card.Input) {
	id := in.Issue.ID
	in.Children = m.forest.Tickets(id)
	in.ChildrenTitle = "Tickets"
	in.Progress = card.ProgressBar(m.forest.Progress(id))
}

// entryCard — карточка записи Memory или Formulas: имя, строка meta, текст как markdown
// (card.Doc). Кешируется по поколению загрузки и ширине.
func (m *Model) entryCard() {
	e, ok := m.entryOf(m.selected)
	if !ok {
		m.vp.SetContent("")
		return
	}
	l := m.lazyOf(m.tab)
	key := fmt.Sprintf("%s|%d|%d|%s", m.tab, l.gen, m.cardText(), e.key)
	r, ok := m.renders[key]
	if !ok {
		r = rendered{text: card.Doc(e.name, e.meta, e.text, m.o.MarkdownStyle, m.cardText())}
		m.renders[key] = r
	}
	m.vp.SetContent(r.text)
	m.vp.SetYOffset(m.vp.YOffset)
}

// cardTitleOf — заголовок рамки карточки: ID задачи, P<n> группы или имя записи.
func (m Model) cardTitleOf() string {
	if p, ok := tree.ParseGroupKey(m.selected); ok && m.tab == tabTasks {
		return fmt.Sprintf("P%d", p)
	}
	if m.tab.lazy() {
		if e, ok := m.entryOf(m.selected); ok {
			return card.CleanLine(e.name)
		}
		return ""
	}
	return card.CleanLine(m.selected)
}

// selectedIssue — выбранная задача леса; nil — заголовок группы, запись Memory или
// Formulas, пустой выбор.
func (m Model) selectedIssue() *bd.Issue {
	if !m.tab.issues() || m.selected == "" || m.forest == nil {
		return nil
	}
	if _, group := tree.ParseGroupKey(m.selected); group {
		return nil
	}
	is, _ := m.forest.Issue(m.selected)
	return is
}

// tabKeyAllowed — клавиша k действует в активной вкладке: R, p и e — только в Tasks,
// c и / — во вкладках из леса. Остальные клавиши одинаковы во всех вкладках.
func (m Model) tabKeyAllowed(k string) bool {
	switch k {
	case "R", "p", "e":
		return m.tab == tabTasks
	case "c", "/":
		return m.tab.issues()
	}
	return true
}

// hints — подсказки клавиш справа в статус-строке активной вкладки, от полных
// к коротким: статус-строка берёт первый вариант, который влезает.
func (m Model) hints() []string {
	tabs := m.t.TabsHint + " · "
	if m.tab.lazy() {
		return []string{tabs + m.t.ReloadHint + " · " + m.t.HelpHint, m.t.HelpHint}
	}
	right := m.t.SearchHint + " · " + m.t.HelpHint
	if m.selectedIssue() != nil {
		right = m.t.SearchHint + " · " + m.t.CommentHint + " · " + m.t.HelpHint
	}
	return []string{tabs + right, right, m.t.HelpHint}
}

// tasksOpts — фильтры дерева Tasks: общие переключатели и запрос вкладки Tasks.
func (m Model) tasksOpts() tree.Options {
	o := m.opts
	if m.tab != tabTasks {
		o.Query = m.views[tabTasks].query
	}
	return o
}
