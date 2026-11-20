package app

import (
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/card"
	"github.com/nizom333/bdv/internal/tree"
)

// handleKey — обработка клавиш по состоянию экрана.
// Порядок: справка, режим ввода, глобальные клавиши, затем клавиши панели в фокусе.
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if m.commenting {
		// Поле комментария забирает всё, кроме Ctrl+C; склеенные руны — тоже текст.
		if msg.String() == "ctrl+c" {
			return m.quit()
		}
		return m.commentKey(msg)
	}
	if m.set.open {
		// Экран настроек забирает все клавиши: клавиши дерева не действуют.
		return m.settingsKey(msg)
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !m.typing {
		if msg.Paste {
			// Вставленный текст вне ввода — не команды.
			return nil
		}
		return m.splitRunes(msg)
	}
	k := msg.String()
	if k == "ctrl+c" {
		return m.quit()
	}
	if m.forest == nil {
		// Экран загрузки, ошибки первой загрузки или bd init (init.go).
		return m.startKey(k)
	}
	if m.help {
		m.helpKey(k)
		return nil
	}
	if m.typing {
		return m.searchKey(msg)
	}
	if !m.tabKeyAllowed(k) {
		// R, p, e — только в Tasks; c и / — во вкладках из леса (tabs.go).
		return nil
	}
	switch k {
	case "[":
		return m.switchTab(-1)
	case "]":
		return m.switchTab(1)
	case "q":
		return m.quit()
	case "?":
		m.help, m.helpOff = true, 0
		return nil
	case "/":
		m.startSearch()
		return nil
	case "r":
		if m.tab.lazy() {
			// Ленивая вкладка перечитывает только свои данные: список задач обновит опрос.
			return m.loadTab(m.tab)
		}
		m.forgetDetailErrors()
		return tea.Batch(m.reload(), m.ensureDetail())
	case "y", "Y", "D", "M":
		return m.copySelected(rune(k[0]))
	case "S":
		return m.toggleMouse()
	case "m":
		return m.openComment()
	case ",":
		return m.openSettings()
	case "c":
		m.opts.ShowClosed = !m.opts.ShowClosed
		return m.rebuild()
	case "e":
		m.opts.ShowEvents = !m.opts.ShowEvents
		return m.rebuild()
	case "p":
		m.opts.GroupByPriority = !m.opts.GroupByPriority
		return m.rebuild()
	case "R":
		m.opts.ReadyOnly = !m.opts.ReadyOnly
		return m.rebuild()
	case "tab":
		if m.focus == focusTree {
			m.focus = focusDetail
		} else {
			m.focus = focusTree
		}
		return nil
	case "esc":
		return m.escape()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		return m.jump(int(k[0] - '0'))
	}
	if m.focus == focusDetail {
		m.detailKey(k)
		return nil
	}
	if k == "enter" && m.narrow() {
		// В узком режиме Enter открывает карточку; раскрытие — через l/h/space.
		if m.selected != "" {
			m.focus = focusDetail
		}
		return nil
	}
	return m.treeKey(k)
}

// splitRunes разбирает склеенные руны на отдельные нажатия по порядку.
// Bubble Tea v1 отдаёт руны одного чтения одним KeyMsg («jjj» при быстром наборе,
// автоповторе или tmux send-keys), и msg.String() не совпал бы ни с одной клавишей.
// Каждая руна идёт через handleKey: если посреди пачки включился поиск («/кон»),
// остаток уходит в строку ввода, как при наборе по одной.
func (m *Model) splitRunes(msg tea.KeyMsg) tea.Cmd {
	var cmds []tea.Cmd
	for _, r := range msg.Runes {
		if m.ctx.Err() != nil {
			// После q остальные руны не нужны: программа завершается.
			break
		}
		cmds = append(cmds, m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: msg.Alt}))
	}
	return tea.Batch(cmds...)
}

// Что копируют клавиши y, Y, D и M.
const (
	copyID    = 'y'
	copyTitle = 'Y'
	copyDesc  = 'D'
	copyCard  = 'M'
)

// copySelected копирует из выбранной задачи то, что задаёт what (copyID…copyCard),
// и показывает тост на toastFor: что скопировано или ошибку. Текст bd идёт как есть,
// без управляющих символов (card.Clean; заголовок — одной строкой, card.CleanLine);
// описание — исходный markdown из списка.
// Заголовок группы, пустой выбор, пустой заголовок или описание — «нечего копировать».
func (m *Model) copySelected(what rune) tea.Cmd {
	iss := m.selectedIssue()
	text, done := "", ""
	if iss != nil {
		switch what {
		case copyID:
			text, done = card.Clean(iss.ID), m.t.Copied(card.CleanLine(iss.ID))
		case copyTitle:
			// Заголовок — одна строка, как в дереве и карточке.
			text, done = card.CleanLine(iss.Title), m.t.CopiedTitle
		case copyDesc:
			text, done = card.Clean(iss.Description), m.t.CopiedDesc
		case copyCard:
			text, done = card.Markdown(iss), m.t.CopiedCard
		}
	}
	switch {
	case strings.TrimSpace(text) == "":
		m.toast = m.t.NothingToCopy
	default:
		if err := m.o.Copy(text); err != nil {
			m.toast = m.t.CopyFailed(card.CleanLine(err.Error()))
		} else {
			m.toast = done
		}
	}
	m.toastSeq++
	return m.o.After(toastFor, toastClearMsg{seq: m.toastSeq})
}

// Последовательности alternate scroll (DECSET 1007): при нём терминал в alt screen
// превращает колесо в ↑/↓, если мышь не перехвачена. В Bubble Tea v1 команд для них нет.
const (
	AltScrollOn  = "\x1b[?1007h"
	altScrollOff = "\x1b[?1007l"
)

// toggleMouse — клавиша S: выключает перехват мыши терминалом, чтобы работало обычное
// выделение текста, и включает обратно. Bubble Tea v1 переключает режим командой.
// Вместе с перехватом выключается alternate scroll: иначе колесо пришло бы стрелками
// и двигало курсор дерева. Включается он обратно, только если его выключили здесь.
func (m *Model) toggleMouse() tea.Cmd {
	m.mouseOff = !m.mouseOff
	if m.mouseOff {
		m.altScrollOff = true
		return tea.Batch(tea.DisableMouse, m.termCmd(altScrollOff))
	}
	cmds := []tea.Cmd{tea.EnableMouseCellMotion}
	if m.altScrollOff {
		m.altScrollOff = false
		cmds = append(cmds, m.termCmd(AltScrollOn))
	}
	return tea.Batch(cmds...)
}

// termCmd пишет сырую последовательность seq в терминал (Options.Term). Кадр Bubble
// Tea пишется одним Write, короткая последовательность между кадрами его не рвёт.
func (m Model) termCmd(seq string) tea.Cmd {
	w := m.o.Term
	return func() tea.Msg {
		_, _ = io.WriteString(w, seq)
		return nil
	}
}

// helpKey — клавиши справки. Если она не влезает, j/k/PgUp/PgDn прокручивают её;
// любая другая клавиша закрывает справку и дальше не идёт.
func (m *Model) helpKey(k string) {
	_, page, maxOff := m.helpWindow()
	if maxOff == 0 {
		m.help = false
		return
	}
	switch k {
	case "j", "down":
		m.helpOff++
	case "k", "up":
		m.helpOff--
	case "pgdown":
		m.helpOff += page
	case "pgup":
		m.helpOff -= page
	default:
		m.help = false
	}
	m.helpOff = max(0, min(m.helpOff, maxOff))
}

// escape: из карточки узкого режима — к дереву, иначе сброс закреплённого фильтра.
func (m *Model) escape() tea.Cmd {
	if m.narrow() && m.focus == focusDetail {
		m.focus = focusTree
		return nil
	}
	if m.opts.Query != "" {
		return m.clearQuery()
	}
	return nil
}

// treeKey — клавиши дерева.
func (m *Model) treeKey(k string) tea.Cmd {
	switch k {
	case "j", "down":
		return m.selectIndex(m.cursor+1, false)
	case "k", "up":
		return m.selectIndex(m.cursor-1, false)
	case "g":
		return m.selectIndex(0, false)
	case "G":
		return m.selectIndex(len(m.rows)-1, false)
	case "l", "right":
		if r, ok := m.current(); ok && r.HasChildren && !r.Expanded {
			return m.setExpanded(r.Key(), true)
		}
	case "h", "left":
		r, ok := m.current()
		if !ok {
			return nil
		}
		if r.HasChildren && r.Expanded {
			return m.setExpanded(r.Key(), false)
		}
		if r.IsGroup || m.tab != tabTasks {
			// В плоских вкладках родителей и групп нет.
			return nil
		}
		// К родителю, а у корня в режиме групп — к заголовку его группы.
		up := m.forest.ParentID(r.Issue.ID)
		if up == "" && m.opts.GroupByPriority {
			up = tree.GroupKey(r.Group)
		}
		if i := m.indexOf(up); i >= 0 {
			return m.selectIndex(i, false)
		}
	case "enter", " ", "space":
		if r, ok := m.current(); ok && r.HasChildren {
			return m.setExpanded(r.Key(), !r.Expanded)
		}
	}
	return nil
}

// detailKey — прокрутка карточки в её фокусе; h/l и прочие клавиши ничего не делают.
func (m *Model) detailKey(k string) {
	switch k {
	case "j", "down":
		m.vp.ScrollDown(1)
	case "k", "up":
		m.vp.ScrollUp(1)
	case "pgdown":
		m.vp.PageDown()
	case "pgup":
		m.vp.PageUp()
	}
}

// handleMouse — колесо: над деревом двигает курсор, над карточкой прокручивает её.
// В узком режиме колесо действует на показанную панель. Фокус не меняется.
// При открытом поле комментария, экране настроек и выключенном перехвате (S) мышь ничего не делает:
// события, пришедшие до того, как терминал выключил режим, отбрасываются.
func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if m.mouseOff || m.forest == nil || m.help || m.commenting || m.set.open || msg.Action != tea.MouseActionPress {
		return nil
	}
	var d int
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		d = -1
	case tea.MouseButtonWheelDown:
		d = 1
	default:
		return nil
	}
	overTree := msg.X < m.treeWidth()
	if m.narrow() {
		overTree = m.focus == focusTree
	}
	if overTree {
		return m.selectIndex(m.cursor+d, false)
	}
	if d > 0 {
		m.vp.ScrollDown(3)
	} else {
		m.vp.ScrollUp(3)
	}
	return nil
}

// --- Поиск ---

// startSearch включает ввод; закреплённый запрос продолжает редактироваться.
func (m *Model) startSearch() {
	if m.opts.Query == "" {
		// Начало нового поиска: запоминаем выбор, чтобы Esc вернул его,
		// если запрос ничего не нашёл.
		m.preSearch = m.selected
	}
	m.typing = true
	m.input.SetValue(m.opts.Query)
	m.input.CursorEnd()
	m.input.Focus()
}

// stopSearch выключает ввод, запрос не трогает.
func (m *Model) stopSearch() {
	m.typing = false
	m.input.Blur()
}

// searchKey — клавиши режима ввода: Enter закрепляет, Esc сбрасывает, остальное правит запрос.
func (m *Model) searchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		return m.clearQuery()
	case "enter":
		m.stopSearch()
		if strings.TrimSpace(m.opts.Query) == "" {
			// Пустой запрос из пробелов не закрепляется.
			return m.clearQuery()
		}
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if q := m.input.Value(); q != m.opts.Query {
		return tea.Batch(cmd, m.setQuery(q))
	}
	return cmd
}

// setQuery применяет новый запрос и ставит курсор на первое совпадение
// (не строку-контекст и не заголовок группы). Запрос, стёртый до пустого, ведёт себя
// как Esc: выбор сохраняется (keepSelection), ввод продолжается.
func (m *Model) setQuery(q string) tea.Cmd {
	m.opts.Query = q
	if strings.TrimSpace(q) == "" {
		return m.keepSelection(m.preSearch)
	}
	m.computeRows()
	i := 0
	for j, r := range m.rows {
		if !r.Context && !r.IsGroup {
			i = j
			break
		}
	}
	return m.selectIndex(i, false)
}

// resetQuery сбрасывает запрос и выходит из ввода; строки не пересчитывает.
func (m *Model) resetQuery() {
	m.stopSearch()
	m.opts.Query = ""
	m.input.SetValue("")
	m.preSearch = ""
}

// clearQuery сбрасывает запрос и выходит из ввода; выбор — см. keepSelection.
func (m *Model) clearQuery() tea.Cmd {
	prev := m.preSearch
	m.resetQuery()
	return m.keepSelection(prev)
}

// keepSelection пересчитывает строки после снятия запроса. Выбор остаётся, если задача
// видна. Найденная задача, которая без запроса скрылась бы в свёрнутой ветке или группе,
// остаётся выбранной: её предки и группа раскрываются (ruling V3). Если запрос ничего
// не нашёл и выбора нет, возвращается выбор до поиска prev, когда он виден.
func (m *Model) keepSelection(prev string) tea.Cmd {
	if m.forest == nil {
		return nil
	}
	if _, group := tree.ParseGroupKey(m.selected); m.selected != "" && !group {
		m.computeRows()
		if m.indexOf(m.selected) < 0 && m.tab == tabTasks {
			m.expandTo(m.selected)
		}
		return m.rebuild()
	}
	if m.selected == "" && prev != "" {
		m.computeRows()
		if i := m.indexOf(prev); i >= 0 {
			return m.selectIndex(i, false)
		}
	}
	return m.rebuild()
}

// --- Переходы по связям ---

// jump выбирает links[n-1] текущей карточки, делая задачу видимой. Из вкладок Specs
// и Decisions переход ведёт во вкладку Tasks: там видны все задачи.
func (m *Model) jump(n int) tea.Cmd {
	if n < 1 || n > len(m.links) {
		return nil
	}
	id := m.links[n-1]
	// switched — переход сменил вкладку: Tasks могла помнить выбранной ту же задачу,
	// и без force карточка осталась бы карточкой прежней вкладки (тело и ссылки).
	switched := false
	if _, ok := m.forest.Issue(id); ok && m.tab != tabTasks {
		m.enterTab(tabTasks)
		switched = true
	}
	if !m.reveal(id) {
		return nil
	}
	m.computeRows()
	if m.indexOf(id) < 0 && m.opts.Query != "" {
		// Задача не подходит под запрос: запрос сбрасывается.
		m.resetQuery()
		m.computeRows()
	}
	if i := m.indexOf(id); i >= 0 {
		return m.selectIndex(i, switched)
	}
	return m.selectIndex(m.targetIndex(m.forest), switched)
}

// current — строка под курсором.
func (m Model) current() (row tree.Row, ok bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return tree.Row{}, false
	}
	return m.rows[m.cursor], true
}

// setExpanded запоминает ручное раскрытие узла или группы (ключ — Row.Key) и пересчитывает строки.
func (m *Model) setExpanded(id string, v bool) tea.Cmd {
	m.exp[id] = v
	return m.rebuild()
}
