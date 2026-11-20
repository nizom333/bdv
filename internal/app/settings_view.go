package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/card"
	"github.com/nizom333/bdv/internal/settings"
)

// --- Экран настроек bd: отрисовка (состояние и клавиши — settings.go) ---

// settingsBox — ширина текста и высота тела внутри рамки экрана настроек.
func (m Model) settingsBox() (tw, bodyH int) { return max(1, m.width-4), max(1, m.height-2) }

// settingsConfirmParts — куски подтверждения: строка изменения, предупреждения, клавиши.
func (m Model) settingsConfirmParts() []part {
	p := m.set.pend
	parts := []part{{lines: []string{m.t.SettingConfirm(p.key, p.old, p.new)}, must: true, wrap: true, st: boldSt}}
	for i, w := range p.warns {
		parts = append(parts, part{lines: []string{"⚠ " + w}, must: true, wrap: true, glue: i > 0, st: warnSt})
	}
	return append(parts, part{lines: []string{m.t.SettingConfirmKeys}, must: true, wrap: true, keys: true, st: faintSt})
}

// warnSt — предупреждение в подтверждении.
var warnSt = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "130", Dark: "214"})

// settingsConfirmFits — подтверждение помещается в рамку целиком, с переносами: человек
// видит, что меняет, и все предупреждения. Размер окна неизвестен или меньше минимального —
// не помещается.
func (m Model) settingsConfirmFits() bool {
	// Меньше minWidth×minHeight View рисует только TooSmall: диалога не видно.
	if m.width < minWidth || m.height < minHeight {
		return false
	}
	tw, bodyH := m.settingsBox()
	n, seps := 0, 0
	for i, p := range m.settingsConfirmParts() {
		n += len(p.rows(tw, true))
		if i > 0 && !p.glue {
			seps++
		}
	}
	return n+seps <= bodyH
}

// settingsView — экран настроек в рамке на всё окно; последняя строка тела — подсказка.
func (m Model) settingsView() string {
	tw, bodyH := m.settingsBox()
	var body []string
	hint := m.t.SettingsKeys
	switch {
	case m.set.loading:
		body = m.centeredIn(m.t.SettingsLoading, tw, bodyH-1)
		hint = ""
	case !m.set.loaded:
		body = strings.Split(ansi.Wrap(errorSt.Render("⚠ "+m.t.SettingsLoadFailed(m.set.loadErr)), tw, ""), "\n")
		hint = m.t.SettingsLoadHint
	case m.set.mode == setConfirm:
		if m.settingsConfirmFits() {
			body = strings.Split(pageIn(m.settingsConfirmParts(), tw, bodyH), "\n")
		} else {
			body = m.centeredIn(m.t.TooSmall, tw, bodyH)
		}
		hint = ""
	case m.set.mode == setListEdit:
		body = m.listEditLines(tw, bodyH-1)
		hint = m.t.ListKeys
		if m.set.adding {
			hint = m.t.ListAddKeys
		}
	default:
		lines, a, b := m.settingsLines(tw)
		h := m.listHeight()
		off := scrollSpan(m.set.off, a, b, h, len(lines))
		body = lines[off:min(len(lines), off+h)]
		if m.set.mode == setNumber {
			hint = m.t.NumberKeys
		}
	}
	if m.set.saving {
		hint = m.t.SettingsSaving
	}
	full := m.set.loaded && !m.set.loading && m.set.mode == setConfirm
	if !full {
		// Последняя строка тела — подсказка клавиш (или «saving…»).
		body = body[:min(len(body), bodyH-1)]
		for len(body) < bodyH-1 {
			body = append(body, "")
		}
		if m.envNoteShown() {
			// Над подсказкой — напоминание об окружении; место под него отнято у списка.
			body[len(body)-1] = faintSt.Render(ansi.Truncate(m.t.SettingsEnvNote, tw, "…"))
		}
		body = append(body, faintSt.Render(ansi.Truncate(hint, tw, "…")))
	}
	padded := make([]string, 0, bodyH)
	for _, l := range body[:min(len(body), bodyH)] {
		padded = append(padded, " "+fit(l, tw)+" ")
	}
	for len(padded) < bodyH {
		padded = append(padded, "")
	}
	return strings.Join(frame(m.t.SettingsHeader(card.CleanLine(m.o.Project)), padded, m.width, true), "\n")
}

// centeredIn — строка s по центру области w×h.
func (m Model) centeredIn(s string, w, h int) []string {
	return strings.Split(lipgloss.Place(w, max(1, h), lipgloss.Center, lipgloss.Center, ansi.Truncate(s, w, "…")), "\n")
}

// scrollSpan — сдвиг прокрутки, при котором строки [a, b) видны в окне высотой h
// (если не помещаются — видно начало), в пределах n строк.
func scrollSpan(off, a, b, h, n int) int {
	if h <= 0 {
		return 0
	}
	if b-a > h {
		b = a + h
	}
	if a < off {
		off = a
	}
	if b > off+h {
		off = b - h
	}
	return max(0, min(off, n-h))
}

// settingsScroll запоминает прокрутку списка настроек так, чтобы выбранная строка
// с описанием была видна.
func (m *Model) settingsScroll() {
	if !m.set.open || !m.set.loaded || m.width <= 0 || m.height <= 0 {
		return
	}
	tw, _ := m.settingsBox()
	lines, a, b := m.settingsLines(tw)
	m.set.off = scrollSpan(m.set.off, a, b, m.listHeight(), len(lines))
}

// envNoteMinBody — с какой высоты тела над подсказкой есть строка про окружение.
const envNoteMinBody = 10

// envNoteShown — строка про BD_* и глобальный конфиг видна: список настроек и окно не низкое.
func (m Model) envNoteShown() bool {
	_, bodyH := m.settingsBox()
	return m.set.loaded && !m.set.loading && (m.set.mode == setMain || m.set.mode == setNumber) && bodyH >= envNoteMinBody
}

// listHeight — сколько строк тела занимает список настроек: без подсказки и строки про окружение.
func (m Model) listHeight() int {
	_, bodyH := m.settingsBox()
	h := bodyH - 1
	if m.envNoteShown() {
		h--
	}
	return h
}

// settingsLines — строки списка настроек шириной tw: заголовки секций, строки
// «название  значение», под выбранной — описание с ключом bd (или поле ввода с ошибкой),
// под строкой с ошибкой записи — ошибка. a, b — строки выбранной настройки (для прокрутки);
// заголовок секции первой строки секции входит в них.
func (m Model) settingsLines(tw int) (lines []string, a, b int) {
	// Ширина колонки названий — по всей таблице: она не прыгает, когда скрываются длины хеша.
	lw := 0
	for _, e := range settings.Table {
		lw = max(lw, ansi.StringWidth(m.t.SettingLabels[e.Key]))
	}
	lw = min(lw, max(6, tw*55/100))
	sect := settings.Section(-1)
	plain := lipgloss.NewStyle()
	if n := m.set.snap.Failed; n > 0 {
		// Упавшие config get не пропадают молча: число и r — повторить.
		lines = append(lines, faintSt.Render(ansi.Truncate("⚠ "+m.t.ReadFailed(n), tw, "…")), "")
	}
	for i, e := range m.set.rows {
		start := len(lines)
		if e.Section != sect {
			if sect >= 0 {
				lines = append(lines, "")
			}
			sect = e.Section
			lines = append(lines, accentSt.Bold(true).Render(ansi.Truncate(m.t.SettingsSections[e.Section], tw, "…")))
		}
		sel := i == m.set.cur
		val, def := m.displayValue(e)
		vst := plain
		if def {
			vst = faintSt
		}
		base := plain
		if sel {
			base = selOn
		}
		label := pad(ansi.Truncate(m.t.SettingLabels[e.Key], lw, "…"), lw)
		lines = append(lines, row(base, tw, seg{" " + label + "  ", plain}, seg{val, vst}))
		if sel {
			a = start
			if m.set.mode == setNumber {
				lines = append(lines, ansi.Truncate("   "+m.t.NumberPrompt+m.set.input.View(), tw, "…"))
				if m.set.inputErr != "" {
					lines = append(lines, wrapIndent(errorSt, "⚠ "+m.set.inputErr, tw)...)
				}
			} else {
				lines = append(lines, wrapIndent(faintSt, m.t.SettingDescs[e.Key]+" · "+e.Key, tw)...)
			}
		}
		if sel && m.set.notice != "" {
			lines = append(lines, wrapIndent(errorSt, "⚠ "+m.set.notice, tw)...)
		}
		if msg := m.set.rowErr[e.Key]; msg != "" {
			lines = append(lines, wrapIndent(errorSt, "⚠ "+msg, tw)...)
		}
		if sel {
			b = len(lines)
		}
	}
	return lines, a, b
}

// wrapIndent — текст s с отступом в три колонки, перенесённый по ширине tw, в стиле st.
func wrapIndent(st lipgloss.Style, s string, tw int) []string {
	const indent = "   "
	w := max(1, tw-len(indent))
	var out []string
	for _, l := range strings.Split(ansi.Wrap(s, w, ""), "\n") {
		out = append(out, indent+st.Render(l))
	}
	return out
}

// listEditLines — редактор списка в h строк шириной tw: название и ключ, встроенные
// элементы (только для чтения), свои элементы с курсором, поле добавления и ошибка.
// Прокрутка — от выбранного элемента или поля добавления.
func (m Model) listEditLines(tw, h int) []string {
	e, _ := m.set.current()
	head := []string{boldSt.Render(ansi.Truncate(m.t.SettingLabels[e.Key]+" · "+e.Key, tw, "…")), ""}
	builtin := m.t.ListBuiltin + " " + strings.Join(e.BuiltinItems(m.set.snap), ", ")
	for _, l := range strings.Split(ansi.Wrap(builtin, tw, ""), "\n") {
		head = append(head, faintSt.Render(l))
	}
	head = append(head, m.t.ListCustom)
	var items []string
	focus := 0
	for i, it := range m.set.items {
		st := lipgloss.NewStyle()
		if i == m.set.itemCur && !m.set.adding {
			st = selOn
			focus = len(items)
		}
		items = append(items, row(st, tw, seg{"  " + card.CleanLine(it), lipgloss.NewStyle()}))
	}
	if len(m.set.items) == 0 {
		items = append(items, faintSt.Render("  "+m.t.ListNone))
	}
	if m.set.adding {
		focus = len(items)
		items = append(items, ansi.Truncate("  "+m.t.ListAddPrompt+m.set.input.View(), tw, "…"))
	}
	if m.set.inputErr != "" {
		items = append(items, wrapIndent(errorSt, "⚠ "+m.set.inputErr, tw)...)
		if m.set.adding {
			focus = len(items) - 1
		}
	}
	// Шапка уступает место элементам, если окно низкое: элементы важнее.
	all := append(head, items...)
	focus += len(head)
	off := scrollSpan(0, focus, focus+1, h, len(all))
	return all[off:min(len(all), off+h)]
}
