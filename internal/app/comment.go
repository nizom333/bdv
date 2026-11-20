package app

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/card"
)

// --- Комментарий: единственная запись в bd ---

// Высота поля комментария: рамка (верхняя граница с заголовком и нижняя), commentTextH
// строк текста и строка подсказки под ним.
const (
	commentTextH = 5
	commentBoxH  = commentTextH + 3
	// minPanelBody — сколько строк тела панелей должно остаться над полем; иначе
	// поле занимает всё тело, а панели скрываются.
	minPanelBody = 3
)

// newTextarea — поле ввода комментария: без номеров строк и приглашения, курсор
// без мигания (никаких таймеров), стили без собственных цветов.
func newTextarea() textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	// Без предела строк: по умолчанию Enter перестаёт работать на 99-й строке.
	ta.MaxHeight = 0
	ta.Cursor.SetMode(cursor.CursorStatic)
	plain := lipgloss.NewStyle()
	ta.FocusedStyle = textarea.Style{
		Base: plain, CursorLine: plain, CursorLineNumber: plain, EndOfBuffer: plain,
		LineNumber: plain, Placeholder: faintSt, Prompt: plain, Text: plain,
	}
	// Во время отправки текст приглушён.
	ta.BlurredStyle = ta.FocusedStyle
	ta.BlurredStyle.Text = faintSt
	ta.BlurredStyle.CursorLine = faintSt
	// Blur переводит внутренний указатель стиля на наш BlurredStyle вместо стандартного.
	ta.Blur()
	return ta
}

// boxHeight — высота поля комментария с рамкой; 0, если оно закрыто. Если над полем
// не помещаются панели, поле занимает всё тело между верхней и статус-строкой (строки
// вкладок тогда нет, см. mainView).
func (m Model) boxHeight() int {
	switch {
	case !m.commenting:
		return 0
	case m.treeHeight() > 0:
		return commentBoxH
	}
	return max(0, m.height-2)
}

// commentText — ширина текста поля: без рамки и полей в одну колонку.
func (m Model) commentText() int { return max(1, m.width-4) }

// openComment открывает поле для выбранной задачи (не заголовка группы) и
// возвращает черновик этой задачи, если он есть.
func (m *Model) openComment() tea.Cmd {
	if m.selectedIssue() == nil {
		// Заголовок группы, запись Memory или Formulas, пустой выбор.
		return nil
	}
	id := m.selected
	m.commenting, m.commentID, m.commentErr, m.commentTimeout, m.sending = true, id, "", false, false
	m.ta.Reset()
	m.ta.SetValue(m.drafts[id])
	cmd := m.ta.Focus() // курсор статичный: команды нет
	m.layout()
	return cmd
}

// closeComment закрывает поле; keepDraft сохраняет непустой текст черновиком задачи,
// иначе черновик удаляется.
func (m *Model) closeComment(keepDraft bool) {
	id, v := m.commentID, m.ta.Value()
	if keepDraft && strings.TrimSpace(v) != "" {
		m.drafts[id] = v
	} else {
		delete(m.drafts, id)
	}
	m.commenting, m.commentID, m.commentErr, m.commentTimeout, m.sending = false, "", "", false, false
	m.ta.Reset()
	m.ta.Blur()
	m.layout()
}

// commentKey — клавиши открытого поля: Ctrl+S отправляет, Esc закрывает с сохранением
// черновика, остальное правит текст. Alt+Enter не отправляет (v1.4): быстрые Esc+Enter
// терминал склеивает в Alt+Enter, и комментарий уходил бы при отмене; textarea его
// не знает, так что он ничего не делает. Во время отправки не работает ничего
// (Ctrl+C обрабатывается раньше).
func (m *Model) commentKey(msg tea.KeyMsg) tea.Cmd {
	if m.sending {
		return nil
	}
	switch msg.String() {
	case "ctrl+s":
		return m.sendComment()
	case "esc":
		m.closeComment(true)
		return nil
	}
	if msg.Paste {
		msg.Runes = normalizeNewlines(msg.Runes)
	}
	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)
	return cmd
}

// normalizeNewlines заменяет во вставленном тексте \r\n и одиночный \r на \n:
// иначе вставка из Windows даёт пустую строку после каждой строки.
func normalizeNewlines(rs []rune) []rune {
	out := make([]rune, 0, len(rs))
	for i, r := range rs {
		if r == '\r' {
			if i+1 < len(rs) && rs[i+1] == '\n' {
				continue
			}
			r = '\n'
		}
		out = append(out, r)
	}
	return out
}

// sendComment запускает bd comments add с текстом без пробелов по краям; пустой текст
// не отправляется. Предел — DetailTimeout, контекст — корневой: выход из bdv убивает bd.
func (m *Model) sendComment() tea.Cmd {
	text := strings.TrimSpace(m.ta.Value())
	if text == "" {
		return nil
	}
	m.sending, m.commentErr, m.commentTimeout = true, "", false
	m.ta.Blur()
	src, root, d, t, id := m.o.Source, m.ctx, m.o.DetailTimeout, m.t, m.commentID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(root, d)
		defer cancel()
		err := src.AddComment(ctx, id, text)
		return addCommentMsg{id: id, err: timeoutErr(ctx, err, d, t)}
	}
}

// onAddComment — ответ bd comments add. Ошибка: поле остаётся с текстом, под ним
// первая строка ошибки (при таймауте — с предупреждением, что комментарий мог
// сохраниться), правка снова доступна. В обоих случаях комментарии задачи
// запрашиваются заново, а список перечитывается. Успех: поле закрывается, черновик
// удаляется, тост; кеш комментариев задачи сбрасывается с новым поколением,
// комментарии запрашиваются заново (даже при comment_count 0 в старом списке),
// список перечитывается (reloadFresh: и после уже идущего list).
func (m *Model) onAddComment(msg addCommentMsg) tea.Cmd {
	if !m.commenting || !m.sending || msg.id != m.commentID {
		return nil
	}
	m.sending = false
	id := msg.id
	// И при ошибке комментарии запрашиваются заново: bd мог успеть записать (особенно
	// при таймауте), и карточка покажет, сохранился ли комментарий.
	m.refetchComments(id)
	if msg.err != nil {
		m.commentErr = card.CleanLine(firstLine(msg.err))
		var te timeoutError
		m.commentTimeout = errors.As(msg.err, &te)
		m.refreshCard()
		return tea.Batch(m.ta.Focus(), m.ensureDetail(), m.reloadFresh())
	}
	m.closeComment(false) // layout перерисует карточку без старого рендера
	m.toast = m.t.CommentAdded
	m.toastSeq++
	return tea.Batch(m.o.After(toastFor, toastClearMsg{seq: m.toastSeq}), m.ensureDetail(), m.reloadFresh())
}

// refetchComments сбрасывает кеш комментариев задачи id после отправки, чтобы
// ensureDetail запросил их заново (даже при comment_count 0 в старом списке).
// Новое поколение отбросит ответы идущих show и comments; их флаги снимаются,
// как в dropStale, иначе ensureDetail не выдал бы запросы заново.
func (m *Model) refetchComments(id string) {
	m.gen[id]++
	delete(m.showPending, id)
	delete(m.comments, id)
	delete(m.commentsNotice, id)
	delete(m.commentsPending, id)
	m.forceComments[id] = true
	// Ключ рендера не знает о новом комментарии (updated_at в списке прежний до reload).
	clear(m.renders)
}

// commentPane — поле комментария в рамке на всю ширину: заголовок «Comment on <id>:»
// в верхней границе, строки текста, строка подсказки (отправка, ошибка или клавиши).
func (m Model) commentPane() []string {
	h, w := m.boxHeight(), m.commentText()
	textH := max(1, h-3)
	ta := strings.Split(m.ta.View(), "\n")
	body := make([]string, 0, textH+1)
	for i := 0; i < textH; i++ {
		l := ""
		if i < len(ta) {
			l = ta[i]
		}
		body = append(body, " "+fit(l, w)+" ")
	}
	body = append(body, " "+m.commentHint(w)+" ")
	return frame(m.t.CommentTitle(card.CleanLine(m.commentID)), body, m.width, true)
}

// commentHint — строка под текстом шириной w: «sending…» во время отправки; ошибка
// (⚠, красным) слева и клавиши справа; иначе клавиши.
func (m Model) commentHint(w int) string {
	keys := faintSt.Render(m.t.CommentKeys)
	switch {
	case m.sending:
		return fit(faintSt.Render(m.t.CommentSending), w)
	case m.commentErr != "":
		msg := m.t.CommentFailed(m.commentErr)
		if m.commentTimeout {
			msg = m.t.CommentTimeout(m.commentErr)
		}
		e := errorSt.Render("⚠ " + msg)
		lw := w - ansi.StringWidth(keys) - 1
		// Узко — ошибка важнее подсказки клавиш. Предупреждение таймаута важнее
		// всегда, если вместе с клавишами не помещается: без него легко отправить дубль.
		if lw < 10 || (m.commentTimeout && ansi.StringWidth(e) > lw) {
			return fit(e, w)
		}
		return fit(e, lw) + " " + keys
	}
	return fit(keys, w)
}
