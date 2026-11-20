package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/card"
	"github.com/nizom333/bdv/internal/tree"
)

// --- Подробности: загрузка и карточка ---

func (m *Model) onShow(msg showMsg) tea.Cmd {
	if msg.gen != m.gen[msg.id] {
		// Запрос выдан до инвалидации: данные могли устареть. Флаг не трогаем —
		// он принадлежит новому запросу.
		return nil
	}
	delete(m.showPending, msg.id)
	if msg.err != nil {
		m.showNotice[msg.id] = m.t.LinksFailed(firstLine(msg.err))
	} else {
		m.details[msg.id] = msg.d
	}
	// Устаревший ответ только кладётся в кеш.
	if msg.id == m.selected {
		m.refreshCard()
	}
	return nil
}

func (m *Model) onComments(msg commentsMsg) tea.Cmd {
	if msg.gen != m.gen[msg.id] {
		// См. onShow.
		return nil
	}
	delete(m.commentsPending, msg.id)
	if msg.err != nil {
		m.commentsNotice[msg.id] = m.t.CommentsFailed(firstLine(msg.err))
		m.comments[msg.id] = []bd.Comment{} // не повторять запрос
	} else {
		if msg.c == nil {
			msg.c = []bd.Comment{}
		}
		m.comments[msg.id] = msg.c
	}
	if msg.id == m.selected {
		m.refreshCard()
	}
	return nil
}

// forgetDetailErrors снимает пометки об ошибках show и comments выбранной задачи,
// чтобы ensureDetail повторил запросы. Только для ручного r: опрос ошибки не повторяет.
func (m *Model) forgetDetailErrors() {
	id := m.selected
	delete(m.showNotice, id)
	if m.commentsNotice[id] != "" {
		// Пустой список комментариев при ошибке лишь запрещал повтор.
		delete(m.commentsNotice, id)
		delete(m.comments, id)
	}
}

// ensureDetail запрашивает недостающие подробности выбранной задачи.
func (m *Model) ensureDetail() tea.Cmd {
	id := m.selected
	if id == "" || !m.tab.issues() {
		return nil
	}
	iss, ok := m.forest.Issue(id)
	if !ok {
		return nil
	}
	src, gen, root, limit, t := m.o.Source, m.gen[id], m.ctx, m.o.DetailTimeout, m.t
	var cmds []tea.Cmd
	if _, ok := m.details[id]; !ok && m.showNotice[id] == "" && !m.showPending[id] {
		m.showPending[id] = true
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(root, limit)
			defer cancel()
			d, err := src.Show(ctx, id)
			return showMsg{id: id, gen: gen, d: d, err: timeoutErr(ctx, err, limit, t)}
		})
	}
	if _, ok := m.comments[id]; !ok && (iss.CommentCount > 0 || m.forceComments[id]) && !m.commentsPending[id] {
		delete(m.forceComments, id)
		m.commentsPending[id] = true
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(root, limit)
			defer cancel()
			c, err := src.Comments(ctx, id)
			return commentsMsg{id: id, gen: gen, c: c, err: timeoutErr(ctx, err, limit, t)}
		})
	}
	return tea.Batch(cmds...)
}

// layout подгоняет viewport под размер окна и перерисовывает карточку.
func (m *Model) layout() {
	m.vp.Width, m.vp.Height = m.cardText(), m.treeHeight()
	m.ta.SetWidth(m.commentText())
	m.ta.SetHeight(max(1, m.boxHeight()-3))
	m.scrollTree()
	m.refreshCard()
}

// refreshCard ставит в viewport карточку выбранной задачи (рендер кешируется)
// или сводку выбранной группы.
func (m *Model) refreshCard() {
	m.links = nil
	if m.tab.lazy() {
		m.entryCard()
		return
	}
	if p, ok := tree.ParseGroupKey(m.selected); ok && m.forest != nil {
		text, links := m.groupCard(p)
		m.links = links
		m.vp.SetContent(text)
		m.vp.SetYOffset(m.vp.YOffset)
		return
	}
	var iss *bd.Issue
	if m.selected != "" && m.forest != nil {
		iss, _ = m.forest.Issue(m.selected)
	}
	if iss == nil {
		m.vp.SetContent("")
		return
	}
	id, w := iss.ID, m.cardText()
	detail, hasDetail := m.details[id]
	comments, hasComments := m.comments[id]
	// Число открытых блокеров меняется без updated_at задачи: закрылся блокер. Строка
	// «waiting on» — только у ждущей задачи: закрытой ждать нечего.
	blockers := 0
	if m.forest.Waiting(id) {
		blockers = len(m.forest.BlockedBy(id))
	}
	// Вкладка в ключе: у спеки своя карточка (Tickets, прогресс), а прогресс меняется
	// без updated_at эпика, когда закрывается тикет; список при этом сбрасывает кеш.
	key := fmt.Sprintf("%s|%s|%s|%d|%s|%s|%d", m.tab, id, iss.UpdatedAt.Format(time.RFC3339Nano), w,
		loadState(hasDetail, m.showNotice[id]), loadState(hasComments, m.commentsNotice[id]), blockers)
	r, ok := m.renders[key]
	if !ok {
		in := card.Input{
			Issue:    iss,
			Children: m.forest.Children(id),
			Lookup:   m.forest.Issue,
			Notice:   joinNotices(m.showNotice[id], m.commentsNotice[id]),
			Blockers: blockers,
			Width:    w,
			Style:    m.o.MarkdownStyle,
			Text:     m.t,
		}
		if m.tab == tabSpecs {
			m.specInput(&in)
		}
		if hasDetail {
			in.Detail = &detail
		}
		if hasComments {
			in.Comments = comments
		}
		text, links := card.Render(in)
		// Строки не шире панели, иначе viewport перенесёт их сам.
		lines := strings.Split(text, "\n")
		for i, l := range lines {
			lines[i] = ansi.Truncate(l, w, "…")
		}
		r = rendered{text: strings.Join(lines, "\n"), links: links}
		m.renders[key] = r
	}
	m.links = r.links
	m.vp.SetContent(r.text)
	// SetContent сдвигает окно, только если оно целиком ниже текста; дожимаем до maxYOffset,
	// чтобы после обновления укоротившаяся карточка не показывала пустоту.
	m.vp.SetYOffset(m.vp.YOffset)
}

// statusOrder — порядок статусов в сводках; неизвестные идут после, по алфавиту.
var statusOrder = []string{"in_progress", "open", "blocked", "deferred", "closed"}

// summary — сводка по статусам «◐ 1 in progress · ○ 2 open» с иконками в цвете статуса.
// Нулевые счётчики скрыты; keepBlocked оставляет blocked и при нуле (верхняя строка).
func (m Model) summary(counts map[string]int, keepBlocked bool) string {
	var unknown []string
	for st := range counts {
		if !slices.Contains(statusOrder, st) {
			unknown = append(unknown, st)
		}
	}
	slices.Sort(unknown)
	var parts []string
	for _, st := range append(slices.Clone(statusOrder), unknown...) {
		n := counts[st]
		if n == 0 && (!keepBlocked || st != "blocked") {
			continue
		}
		icon := card.StatusStyle(st).Render(card.StatusIcon(st))
		parts = append(parts, fmt.Sprintf("%s %d %s", icon, n, card.CleanLine(m.t.StatusLabel(st))))
	}
	return strings.Join(parts, " · ")
}

// groupCard — карточка группы p: жирный заголовок «3 tasks», число видимых задач по
// статусам и секция Tasks — сами задачи в порядке дерева, первые девять нумерованы для
// перехода цифрой (links). Задачи те же, что в счётчике заголовка: с фильтрами
// и поиском, без учёта сворачивания.
func (m *Model) groupCard(p int) (string, []string) {
	members := tree.GroupMembers(m.forest, m.opts, p)
	counts := map[string]int{}
	for _, is := range members {
		counts[is.Status]++
	}
	w := m.cardText()
	lines := []string{boldSt.Render(m.t.GroupTitle(len(members))), "", m.summary(counts, false)}
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, w, "…")
	}
	var links []string
	if len(members) > 0 {
		var list string
		// Название секции английское в обоих языках, как у секций карточки задачи.
		list, links = card.TaskList("Tasks", members, m.forest.Waiting, w)
		lines = append(lines, "", list)
	}
	return strings.Join(lines, "\n"), links
}

// loadState — часть ключа кеша: "d" загружено, "n" ошибка, "-" нет данных.
func loadState(loaded bool, notice string) string {
	switch {
	case notice != "":
		return "n"
	case loaded:
		return "d"
	}
	return "-"
}

func joinNotices(ns ...string) string {
	var out []string
	for _, n := range ns {
		if n != "" {
			out = append(out, n)
		}
	}
	return strings.Join(out, "; ")
}
