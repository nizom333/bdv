package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/card"
)

// --- bd init: предложение создать базу в проекте без неё ---
//
// Состояния (все — пока дерева нет, m.forest == nil):
//   - noDB: экран «нет базы» — вместо экрана ошибки; i открывает подтверждение;
//   - initConfirm: вопрос «Initialize beads in <dir>?»; только y запускает bd init,
//     c переключает режим ID (хеш ↔ последовательные: после init — bd config set);
//   - initRunning: идёт bd init; работает только Ctrl+C;
//   - initDone: bd init успешен, идёт первая загрузка; виден хвост вывода.
// Ошибка bd init возвращает на экран noDB с выводом и текстом ошибки.

// initOutLines — сколько последних строк вывода успешного bd init показывается.
const initOutLines = 8

// noDBMarker — признак ошибки bd list в папке без базы (сравнение без учёта регистра).
const noDBMarker = "no beads database found"

// isNoDB — ошибка bd со stderr, говорящим, что базы beads нет.
func isNoDB(err error) bool {
	var be *bd.Error
	return errors.As(err, &be) && strings.Contains(strings.ToLower(be.Stderr), noDBMarker)
}

// cleanLines — вывод bd для экрана: без ANSI-последовательностей и управляющих
// символов, табуляция — пробелами, без пустых строк и пробелов в конце строк.
func cleanLines(s string) []string {
	s = card.Clean(ansi.Strip(s))
	s = strings.ReplaceAll(s, "\t", "    ")
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimRight(l, " "); strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// errText — полный текст ошибки bd: stderr (или первая строка), иначе текст ошибки.
func errText(err error) string {
	var be *bd.Error
	if errors.As(err, &be) {
		if text := strings.TrimSpace(be.Stderr); text != "" {
			return text
		}
		return be.FirstLine()
	}
	return err.Error()
}

// initErrText — причина неудачи bd init для заголовка: код выхода (сам вывод bd
// показывается ниже целиком) или текст прочей ошибки, например таймаута.
func (m Model) initErrText(err error) string {
	var be *bd.Error
	if errors.As(err, &be) {
		return m.t.ExitStatus(be.Code)
	}
	return card.CleanLine(firstLine(err))
}

// startKey — клавиши, пока дерева нет: экран загрузки, ошибки, без базы,
// подтверждение и запуск bd init. Ctrl+C обработан раньше.
func (m *Model) startKey(k string) tea.Cmd {
	switch {
	case m.help:
		// Справка с экрана без базы: любая клавиша закрывает её (j/k/PgUp/PgDn листают,
		// если не влезает) и дальше не идёт.
		m.helpKey(k)
		return nil
	case m.initRunning:
		return nil
	case m.initConfirm:
		switch k {
		case "y":
			// Путь не виден целиком (окно мало) — подтверждать нечего.
			if m.confirmFits() {
				return m.runInit()
			}
			return nil
		case "c":
			// Режим ID: хеш bd ↔ последовательные 1, 2, 3.
			m.initSeq = !m.initSeq
			return nil
		case "q":
			return m.quit()
		}
		// Любая другая клавиша — отмена.
		m.initConfirm = false
		return nil
	}
	switch k {
	case "q":
		return m.quit()
	case "r":
		if m.loadErr != nil || m.noDB {
			m.initFail, m.initFailMsg = nil, ""
			return m.reload()
		}
	case "i":
		// Пока идёт bd list, его ответ мог бы лечь поверх результата bd init.
		if m.noDB && m.o.InitAllowed && !m.inFlight {
			m.initConfirm = true
		}
	case "?":
		// Справка — там раздел про i; на экранах загрузки и ошибки её по-прежнему нет.
		if m.noDB {
			m.help, m.helpOff = true, 0
		}
	}
	return nil
}

// runInit запускает bd init, а после успеха при выбранных последовательных ID —
// bd config set issue_id_mode counter. Пределы — InitTimeout и DetailTimeout, контекст —
// корневой: выход из bdv (Ctrl+C) убивает bd.
func (m *Model) runInit() tea.Cmd {
	m.initConfirm, m.initRunning = false, true
	m.initFail, m.initFailMsg, m.initCfgErr = nil, "", ""
	src, root, d, cd, seq, t := m.o.Source, m.ctx, m.o.InitTimeout, m.o.DetailTimeout, m.initSeq, m.t
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(root, d)
		defer cancel()
		out, err := src.Init(ctx)
		msg := initMsg{out: out, err: timeoutErr(ctx, err, d, t)}
		if msg.err == nil && seq {
			cctx, ccancel := context.WithTimeout(root, cd)
			defer ccancel()
			msg.cfgErr = timeoutErr(cctx, src.SetConfig(cctx, "issue_id_mode", "counter"), cd, t)
		}
		return msg
	}
}

// onInit — ответ bd init. Ошибка: экран без базы с выводом и причиной, i повторяет.
// Успех: хвост вывода на экране и сразу первая загрузка; имя проекта — папка init
// всегда: имя от main могло прийти от чужого .beads выше по дереву.
func (m *Model) onInit(msg initMsg) tea.Cmd {
	if !m.initRunning {
		return nil
	}
	m.initRunning = false
	out := cleanLines(msg.out)
	if msg.err != nil {
		m.initFail, m.initFailMsg = out, m.initErrText(msg.err)
		return nil
	}
	m.noDB, m.loadErr = false, nil
	m.initDone = true
	if msg.cfgErr != nil {
		m.initCfgErr = m.t.InitIDsFailed(card.CleanLine(firstLine(msg.cfgErr)))
	}
	m.initOut = out[max(0, len(out)-initOutLines):]
	if m.o.InitDir != "" {
		m.o.Project = filepath.Base(m.o.InitDir)
	}
	return m.reloadFresh()
}

// noDBIdle — экран без базы (с подтверждением и запуском): опрос bd list не идёт,
// он только повторял бы ту же ошибку; r запускает его вручную.
func (m Model) noDBIdle() bool { return m.forest == nil && m.noDB }

// --- Экраны ---

// initView — экран состояния bd init, если оно активно.
func (m Model) initView() (string, bool) {
	switch {
	case m.initRunning:
		return m.centered(m.t.Initializing), true
	case m.initConfirm:
		return m.confirmView(), true
	case m.initDone:
		return m.initDoneView(), true
	case m.noDB:
		return m.noDBView(), true
	}
	return "", false
}

// centered — одна строка в центре окна; без размера окна — как есть.
func (m Model) centered(s string) string {
	if m.width <= 0 || m.height <= 0 {
		return s
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, ansi.Truncate(s, m.width, "…"))
}

// noDBView — нет базы: заголовок, stderr bd list, ошибка и вывод bd init, подсказка с i.
func (m Model) noDBView() string {
	parts := []part{{lines: []string{m.t.NoDB}, must: true, wrap: true, st: boldSt}}
	if m.loadErr != nil {
		parts = append(parts, part{lines: cleanLines(errText(m.loadErr)), wrap: true, prio: 2})
	}
	if m.initFailMsg != "" {
		parts = append(parts,
			part{lines: []string{"⚠ " + m.t.InitFailed(m.initFailMsg)}, must: true, wrap: true, st: errorSt},
			// Хвост вывода: причина неудачи у bd обычно в конце.
			part{lines: m.initFail, wrap: true, tail: true, glue: true, prio: 1})
	}
	parts = append(parts, part{lines: []string{m.t.InitHint}, must: true, wrap: true, keys: true, st: faintSt})
	return m.page(parts)
}

// confirmParts — куски подтверждения: вопрос с путём, префикс по умолчанию bd, режим ID
// (c переключает), клавиши.
func (m Model) confirmParts() []part {
	ids := m.t.InitIDsHash
	if m.initSeq {
		ids = m.t.InitIDsSeq
	}
	return []part{
		{lines: []string{m.t.InitConfirm(card.CleanLine(m.o.InitDir))}, must: true, wrap: true, st: boldSt},
		{lines: []string{m.t.InitPrefix(card.CleanLine(filepath.Base(m.o.InitDir)))}, must: true, wrap: true, glue: true},
		{lines: []string{ids}, must: true, wrap: true, glue: true},
		{lines: []string{m.t.InitConfirmHint}, must: true, wrap: true, keys: true, st: faintSt},
	}
}

// confirmFits — подтверждение помещается в окно целиком, с переносами, без обрезки:
// человек видит полный путь, где появится база. Размер окна неизвестен или меньше
// минимального — не помещается.
func (m Model) confirmFits() bool {
	// Меньше minWidth×minHeight View рисует только TooSmall: подтверждения не видно.
	if m.width < minWidth || m.height < minHeight {
		return false
	}
	n := 1 // пустая строка перед подсказкой клавиш
	for _, p := range m.confirmParts() {
		n += len(p.rows(m.width, true))
	}
	return n <= m.height
}

// confirmView — подтверждение bd init; в окне, где оно не помещается целиком, — TooSmall
// (y там не работает, см. startKey).
func (m Model) confirmView() string {
	if !m.confirmFits() {
		return m.centered(m.t.TooSmall)
	}
	return m.page(m.confirmParts())
}

// initDoneView — bd init успешен: последние строки вывода, ошибка bd config set
// (если была) и загрузка списка.
func (m Model) initDoneView() string {
	parts := []part{{lines: m.initOut, tail: true, st: faintSt}}
	if m.initCfgErr != "" {
		parts = append(parts, part{lines: []string{"⚠ " + m.initCfgErr}, must: true, wrap: true, st: errorSt})
	}
	return m.page(append(parts, part{lines: []string{m.t.Loading}, must: true, wrap: true}))
}

// part — кусок экрана без дерева.
type part struct {
	lines []string       // логические строки, уже очищенные
	must  bool           // обязательный: всегда виден; в тесном окне — по строке на логическую строку
	wrap  bool           // переносить по ширине; иначе обрезать с «…»
	keys  bool           // строка подсказки клавиш: перенос только между пунктами « · »
	tail  bool           // при нехватке высоты оставить последние строки, иначе первые
	glue  bool           // без пустой строки перед куском
	prio  int            // порядок раздачи высоты необязательным кускам: меньше — раньше
	st    lipgloss.Style // стиль каждой строки; нулевой — без стиля
}

// rows — строки куска шириной не больше w: перенос или обрезка.
func (p part) rows(w int, wrap bool) []string {
	var out []string
	for _, l := range p.lines {
		switch {
		case wrap && p.keys:
			out = append(out, packKeys(l, w)...)
		case wrap:
			out = append(out, strings.Split(ansi.Wrap(l, w, ""), "\n")...)
		default:
			out = append(out, ansi.Truncate(l, w, "…"))
		}
	}
	return out
}

// packKeys раскладывает подсказку «a · b · c» по строкам шириной w, не разрывая
// пункты; пункт шире w переносится сам по себе.
func packKeys(s string, w int) []string {
	const sep = " · "
	var out []string
	cur := ""
	for _, item := range strings.Split(s, sep) {
		switch {
		case cur == "":
			cur = item
		case ansi.StringWidth(cur+sep+item) <= w:
			cur += sep + item
		default:
			out = append(out, cur)
			cur = item
		}
		if ansi.StringWidth(cur) > w {
			rows := strings.Split(ansi.Wrap(cur, w, ""), "\n")
			out = append(out, rows[:len(rows)-1]...)
			cur = rows[len(rows)-1]
		}
	}
	return append(out, cur)
}

// page собирает экран из кусков (pageIn) размером с окно.
func (m Model) page(parts []part) string { return pageIn(parts, m.width, m.height) }

// pageIn собирает экран из кусков сверху вниз, между кусками — пустая строка, и вписывает
// его в область w×h: обязательные куски видны всегда (если с переносом не влезают — по строке
// без переноса, затем без пустых строк), необязательные получают оставшуюся высоту
// по prio. Результат — ровно h строк не шире w. Без размера — как есть.
func pageIn(parts []part, w, h int) string {
	if w <= 0 || h <= 0 {
		var lines []string
		for i, p := range parts {
			if len(p.lines) == 0 {
				continue
			}
			if i > 0 && !p.glue {
				lines = append(lines, "")
			}
			lines = append(lines, p.lines...)
		}
		return strings.Join(lines, "\n")
	}
	seps := 0
	for i, p := range parts {
		if i > 0 && !p.glue && len(p.lines) > 0 {
			seps++
		}
	}
	rendered := make([][]string, len(parts))
	mustRows := func(wrap bool) int {
		n := 0
		for i, p := range parts {
			if p.must {
				rendered[i] = p.rows(w, wrap && p.wrap)
				n += len(rendered[i])
			}
		}
		return n
	}
	used := mustRows(true)
	if used+seps > h {
		used = mustRows(false)
	}
	sepOn := used+seps <= h
	budget := h - used
	if sepOn {
		budget -= seps
	}
	for prio := 0; prio <= 2; prio++ {
		for i, p := range parts {
			if p.must || p.prio != prio {
				continue
			}
			r := p.rows(w, p.wrap)
			if n := max(0, budget); len(r) > n {
				if p.tail {
					r = r[len(r)-n:]
				} else {
					r = r[:n]
				}
			}
			rendered[i] = r
			budget -= len(r)
		}
	}
	lines := make([]string, 0, h)
	for i, p := range parts {
		if len(rendered[i]) == 0 {
			continue
		}
		if sepOn && len(lines) > 0 && !p.glue {
			lines = append(lines, "")
		}
		for _, r := range rendered[i] {
			lines = append(lines, p.st.Render(r))
		}
	}
	lines = lines[:min(len(lines), h)]
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}
