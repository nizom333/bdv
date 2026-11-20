package app

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/card"
	"github.com/nizom333/bdv/internal/settings"
)

// --- Экран настроек bd (v1.3, клавиша «,») ---
//
// Таблица настроек и проверки — в пакете settings; здесь состояние экрана, клавиши,
// чтение и запись через bd и отрисовка. Режимы (settingsState.mode):
//   - setMain: список настроек по секциям, свой курсор и прокрутка;
//   - setNumber: поле ввода int/float под строкой; Enter проверяет, Esc отменяет;
//   - setListEdit: редактор списка (status.custom, types.custom); a — поле добавления;
//   - setConfirm: подтверждение «<key>: <старое> → <новое>», применяет только y.
// Пока идёт запись и перечитывание (saving), работает только Ctrl+C.

type settingsMode int

const (
	setMain settingsMode = iota
	setNumber
	setListEdit
	setConfirm
)

// settingsReadConcurrency — сколько bd config get идёт одновременно: каждый — отдельный
// процесс bd с открытием базы.
const settingsReadConcurrency = 4

// reloadKeys — ключи, после изменения которых перечитывается и список задач.
var reloadKeys = []string{"issue_id_mode", "status.custom", "types.custom"}

// pendingChange — изменение в подтверждении.
type pendingChange struct {
	key      string
	value    string // новое значение для bd config set
	unset    bool   // bd config unset вместо set
	old, new string // как показать старое и новое
	warns    []string
}

// settingsState — состояние экрана настроек.
type settingsState struct {
	open    bool
	loading bool // первое чтение: на экране «loading settings…»
	saving  bool // идёт запись и следом перечитывание: клавиши, кроме Ctrl+C, не работают
	seq     int  // поколение чтения: ответ устаревшего чтения отбрасывается
	loaded  bool
	loadErr string // ошибка первого чтения

	snap settings.Snapshot
	rows []settings.Entry // видимые настройки
	cur  int
	off  int // первая видимая строка тела

	mode     settingsMode
	back     settingsMode // куда вернуться при отмене подтверждения
	input    textinput.Model
	inputErr string

	// Редактор списка: свои элементы и курсор по ним.
	items   []string
	itemCur int
	adding  bool

	pend    pendingChange
	saveKey string            // ключ идущей записи
	rowErr  map[string]string // ошибка записи под строкой, по ключу
	notice  string            // отказ под выбранной строкой (u сломал бы min ≤ max); до следующей клавиши
}

// Сообщения экрана настроек.
type (
	settingsMsg struct {
		seq  int
		snap settings.Snapshot
		err  error
	}
	settingSavedMsg struct {
		key string
		err error
	}
)

// newSettingsInput — однострочное поле ввода без приглашения, курсор без мигания.
func newSettingsInput() textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.CharLimit = 64
	in.Cursor.SetMode(cursor.CursorStatic)
	return in
}

// openSettings открывает экран и запускает чтение настроек.
func (m *Model) openSettings() tea.Cmd {
	seq := m.set.seq
	m.set = settingsState{open: true, seq: seq, input: newSettingsInput(), rowErr: map[string]string{}}
	return m.loadSettings(true)
}

// closeSettings — назад к дереву; поколение чтения сохраняется, чтобы опоздавший ответ отбросился.
func (m *Model) closeSettings() {
	m.set = settingsState{seq: m.set.seq + 1}
}

// loadSettings читает настройки одной командой: config list (ключи базы), config get
// каждого YAML-ключа параллельно и types. Всё с --readonly, общий предел — DetailTimeout.
// first — первое чтение («loading settings…»); иначе строки остаются на экране.
func (m *Model) loadSettings(first bool) tea.Cmd {
	m.set.seq++
	if first {
		m.set.loading = true
	}
	src, root, d, t, seq := m.o.Source, m.ctx, m.o.DetailTimeout, m.t, m.set.seq
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(root, d)
		defer cancel()
		var (
			wg       sync.WaitGroup
			mu       sync.Mutex
			list     map[string]string
			listErr  error
			types    []bd.TypeInfo
			typesErr error
		)
		gets := map[string]settings.GetResult{}
		sem := make(chan struct{}, settingsReadConcurrency)
		wg.Add(2)
		go func() {
			defer wg.Done()
			list, listErr = src.ConfigList(ctx)
		}()
		go func() {
			defer wg.Done()
			types, typesErr = src.Types(ctx)
		}()
		for _, k := range settings.GetKeys() {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				v, loc, err := src.ConfigGet(ctx, k)
				mu.Lock()
				gets[k] = settings.GetResult{Value: v, Location: loc, Err: err}
				mu.Unlock()
			}()
		}
		wg.Wait()
		if listErr != nil {
			return settingsMsg{seq: seq, err: timeoutErr(ctx, listErr, d, t)}
		}
		var names []string
		if typesErr == nil {
			for _, ti := range types {
				names = append(names, ti.Name)
			}
		}
		return settingsMsg{seq: seq, snap: settings.Build(list, gets, names)}
	}
}

// onSettings — ответ чтения. Курсор остаётся на той же настройке, если она видна.
func (m *Model) onSettings(msg settingsMsg) tea.Cmd {
	if !m.set.open || msg.seq != m.set.seq {
		return nil
	}
	m.set.loading, m.set.saving = false, false
	if msg.err != nil {
		e := cleanErr(msg.err)
		if !m.set.loaded {
			m.set.loadErr = e
		} else if m.set.saveKey != "" && m.set.rowErr[m.set.saveKey] == "" {
			m.set.rowErr[m.set.saveKey] = m.t.SettingsLoadFailed(e)
		}
		m.set.saveKey = ""
		return nil
	}
	key := ""
	if m.set.cur >= 0 && m.set.cur < len(m.set.rows) {
		key = m.set.rows[m.set.cur].Key
	}
	m.set.loaded, m.set.loadErr, m.set.saveKey = true, "", ""
	m.set.snap = msg.snap
	m.set.rows = msg.snap.Visible()
	if i := slices.IndexFunc(m.set.rows, func(e settings.Entry) bool { return e.Key == key }); i >= 0 {
		m.set.cur = i
	}
	m.set.cur = max(0, min(m.set.cur, len(m.set.rows)-1))
	m.settingsScroll()
	return nil
}

// cleanErr — первая строка ошибки bd для экрана: без ANSI и управляющих символов.
func cleanErr(err error) string { return card.CleanLine(ansi.Strip(firstLine(err))) }

// current — выбранная настройка.
func (s settingsState) current() (settings.Entry, bool) {
	if s.cur < 0 || s.cur >= len(s.rows) {
		return settings.Entry{}, false
	}
	return s.rows[s.cur], true
}

// inputActive — клавиши идут в поле ввода.
func (s settingsState) inputActive() bool {
	return s.mode == setNumber || (s.mode == setListEdit && s.adding)
}

// settingsKey — все клавиши, пока открыт экран настроек: клавиши дерева не действуют.
func (m *Model) settingsKey(msg tea.KeyMsg) tea.Cmd {
	k := msg.String()
	if k == "ctrl+c" {
		return m.quit()
	}
	if m.set.saving {
		return nil
	}
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !m.set.inputActive() {
		if msg.Paste {
			return nil
		}
		// Склеенные руны — отдельные нажатия, как в дереве (splitRunes).
		var cmds []tea.Cmd
		for _, r := range msg.Runes {
			cmds = append(cmds, m.settingsKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: msg.Alt}))
		}
		return tea.Batch(cmds...)
	}
	if m.set.loading || !m.set.loaded {
		switch k {
		case "esc":
			m.closeSettings()
		case "r":
			if m.set.loadErr != "" {
				m.set.loadErr = ""
				return m.loadSettings(true)
			}
		}
		return nil
	}
	var cmd tea.Cmd
	switch m.set.mode {
	case setConfirm:
		cmd = m.confirmSettingKey(k)
	case setNumber:
		cmd = m.numberKey(msg)
	case setListEdit:
		cmd = m.listEditKey(msg)
	default:
		cmd = m.settingsMainKey(k)
	}
	m.settingsScroll()
	return cmd
}

// settingsMainKey — клавиши списка настроек.
func (m *Model) settingsMainKey(k string) tea.Cmd {
	e, ok := m.set.current()
	m.set.notice = ""
	switch k {
	case "r":
		// Перечитать: например, после упавших config get.
		return m.loadSettings(false)
	case "esc":
		m.closeSettings()
	case "j", "down":
		m.set.cur = min(m.set.cur+1, len(m.set.rows)-1)
	case "k", "up":
		m.set.cur = max(m.set.cur-1, 0)
	case "g", "home":
		m.set.cur = 0
	case "G", "end":
		m.set.cur = max(0, len(m.set.rows)-1)
	case "enter", " ", "space":
		if ok {
			m.editSetting(e)
		}
	case "u":
		if ok {
			m.proposeUnset(e)
		}
	}
	return nil
}

// editSetting — Enter/Space на строке: bool переключается, enum перебирается (оба —
// через подтверждение), int/float открывают поле ввода, list — редактор списка.
func (m *Model) editSetting(e settings.Entry) {
	m.set.inputErr = ""
	switch e.Kind {
	case settings.Bool:
		m.propose(e, settings.Toggle(e, m.set.snap), setMain)
	case settings.Enum:
		m.propose(e, settings.Cycle(e, m.set.snap), setMain)
	case settings.Int, settings.Float:
		m.set.mode = setNumber
		m.set.input.SetValue(m.set.snap.Effective(e))
		m.set.input.CursorEnd()
		m.set.input.Focus()
	case settings.List:
		m.set.mode = setListEdit
		m.set.items = settings.ParseList(m.set.snap.Values[e.Key].Raw)
		if !m.set.snap.Values[e.Key].Set {
			m.set.items = nil
		}
		m.set.itemCur, m.set.adding = 0, false
	}
}

// displayValue — значение для показа и признак «умолчание»: заданное — как есть
// (список — через «, »), иначе умолчание с пометкой (default).
func (m Model) displayValue(e settings.Entry) (string, bool) {
	v := m.set.snap.Values[e.Key]
	if e.Kind == settings.ReadOnly {
		return card.CleanLine(v.Raw), false
	}
	if v.Set {
		if e.Kind == settings.List {
			return card.CleanLine(strings.Join(settings.ParseList(v.Raw), ", ")), false
		}
		return card.CleanLine(v.Raw), false
	}
	return m.defaultText(e), true
}

// defaultText — «<умолчание> (default)» или только «(default)», если умолчание неизвестно.
func (m Model) defaultText(e settings.Entry) string {
	if e.Default == "" || e.Kind == settings.List {
		return m.t.SettingDefault
	}
	return e.Default + " " + m.t.SettingDefault
}

// propose открывает подтверждение записи value; back — режим для отмены.
func (m *Model) propose(e settings.Entry, value string, back settingsMode) {
	old, _ := m.displayValue(e)
	shown := value
	if e.Kind == settings.List {
		shown = strings.Join(settings.ParseList(value), ", ")
	}
	p := pendingChange{key: e.Key, value: value, old: old, new: card.CleanLine(shown)}
	if e.WarnFor(value) {
		p.warns = append(p.warns, m.t.SettingWarns[e.Warn])
	}
	m.confirm(p, back)
}

// proposeUnset — u: сброс заданного ключа к умолчанию через bd config unset.
func (m *Model) proposeUnset(e settings.Entry) {
	if e.Kind == settings.ReadOnly || !m.set.snap.Values[e.Key].Set {
		return
	}
	if p := settings.CheckUnset(e, m.set.snap); p != nil {
		m.set.notice = m.problemText(p)
		return
	}
	old, _ := m.displayValue(e)
	p := pendingChange{key: e.Key, unset: true, old: old, new: m.defaultText(e)}
	if e.Default != "" && e.WarnFor(e.Default) {
		p.warns = append(p.warns, m.t.SettingWarns[e.Warn])
	}
	if e.Kind == settings.List {
		p.warns = append(p.warns, m.inUseWarns(e, settings.ParseList(m.set.snap.Values[e.Key].Raw))...)
	}
	m.confirm(p, setMain)
}

func (m *Model) confirm(p pendingChange, back settingsMode) {
	m.set.pend, m.set.back, m.set.mode = p, back, setConfirm
	m.set.input.Blur()
}

// inUseWarns — предупреждения об удаляемых элементах списка, которыми пользуются задачи
// текущего списка (все задачи, без фильтров): «N tasks use <name>».
func (m Model) inUseWarns(e settings.Entry, removed []string) []string {
	var out []string
	for _, it := range removed {
		name := settings.ItemName(it)
		n := m.forest.Count(func(iss *bd.Issue) bool {
			if e.Key == "types.custom" {
				return iss.IssueType == name
			}
			return iss.Status == name
		})
		if n > 0 {
			out = append(out, m.t.InUse(n, card.CleanLine(name)))
		}
	}
	return out
}

// numberKey — поле ввода int/float: Enter проверяет (ошибка — под полем, поле открыто),
// Esc отменяет, остальное правит текст.
func (m *Model) numberKey(msg tea.KeyMsg) tea.Cmd {
	e, _ := m.set.current()
	switch msg.String() {
	case "esc":
		m.set.mode, m.set.inputErr = setMain, ""
		m.set.input.Blur()
		return nil
	case "enter":
		v, p := settings.CheckNumber(e, m.set.snap, m.set.input.Value())
		if p != nil {
			m.set.inputErr = m.problemText(p)
			return nil
		}
		m.set.inputErr = ""
		if v == m.set.snap.Values[e.Key].Raw && m.set.snap.Values[e.Key].Set {
			// То же значение: писать нечего.
			m.set.mode = setMain
			m.set.input.Blur()
			return nil
		}
		m.propose(e, v, setNumber)
		return nil
	}
	var cmd tea.Cmd
	m.set.input, cmd = m.set.input.Update(msg)
	return cmd
}

// problemText — текст ошибки ввода.
func (m Model) problemText(p *settings.Problem) string {
	switch p.Kind {
	case settings.NotInt:
		return m.t.NotInt
	case settings.NotNumber:
		return m.t.NotNumber
	case settings.OutOfRange:
		// Границы — с точкой: так bd ждёт ввод на любом языке.
		return m.t.Range(p.A, p.B)
	case settings.AtLeast:
		return m.t.AtLeast(p.A)
	case settings.AboveOther:
		return m.t.AboveOther(p.A, p.B)
	case settings.BelowOther:
		return m.t.BelowOther(p.A, p.B)
	case settings.BadName:
		return m.t.BadName
	case settings.Duplicate:
		return m.t.Duplicate(card.CleanLine(p.A))
	case settings.IsBuiltin:
		return m.t.IsBuiltin(card.CleanLine(p.A))
	case settings.UnsetBreaks:
		return m.t.UnsetBreaks(p.A, card.CleanLine(p.B), p.C, card.CleanLine(p.D))
	}
	return ""
}

// listEditKey — редактор списка: j/k по своим элементам, a — добавить, x/d — удалить,
// Enter — подтвердить новый список, Esc — отмена. В поле добавления Enter проверяет имя.
func (m *Model) listEditKey(msg tea.KeyMsg) tea.Cmd {
	e, _ := m.set.current()
	k := msg.String()
	if m.set.adding {
		switch k {
		case "esc":
			m.set.adding, m.set.inputErr = false, ""
			m.set.input.Blur()
			return nil
		case "enter":
			name := strings.TrimSpace(m.set.input.Value())
			if p := settings.CheckName(e, m.set.snap, m.set.items, name); p != nil {
				m.set.inputErr = m.problemText(p)
				return nil
			}
			m.set.items = append(slices.Clone(m.set.items), name)
			m.set.itemCur = len(m.set.items) - 1
			m.set.adding, m.set.inputErr = false, ""
			m.set.input.Blur()
			return nil
		}
		var cmd tea.Cmd
		m.set.input, cmd = m.set.input.Update(msg)
		return cmd
	}
	switch k {
	case "esc":
		m.set.mode, m.set.items = setMain, nil
	case "j", "down":
		m.set.itemCur = min(m.set.itemCur+1, len(m.set.items)-1)
	case "k", "up":
		m.set.itemCur = max(m.set.itemCur-1, 0)
	case "g", "home":
		m.set.itemCur = 0
	case "G", "end":
		m.set.itemCur = len(m.set.items) - 1
	case "a":
		m.set.adding, m.set.inputErr = true, ""
		m.set.input.SetValue("")
		m.set.input.Focus()
	case "x", "d":
		if i := m.set.itemCur; i >= 0 && i < len(m.set.items) {
			m.set.items = slices.Delete(slices.Clone(m.set.items), i, i+1)
		}
	case "enter":
		m.proposeList(e)
	}
	m.set.itemCur = max(0, min(m.set.itemCur, len(m.set.items)-1))
	return nil
}

// proposeList — подтверждение нового списка: без изменений — просто закрыть редактор;
// пустой список заданного ключа — сброс (unset). Удалённые элементы, которыми
// пользуются задачи, дают предупреждения.
func (m *Model) proposeList(e settings.Entry) {
	v := m.set.snap.Values[e.Key]
	var old []string
	if v.Set {
		old = settings.ParseList(v.Raw)
	}
	items := m.set.items
	if slices.Equal(old, items) {
		m.set.mode, m.set.items = setMain, nil
		return
	}
	var removed []string
	for _, it := range old {
		if !slices.ContainsFunc(items, func(x string) bool { return settings.ItemName(x) == settings.ItemName(it) }) {
			removed = append(removed, it)
		}
	}
	warns := m.inUseWarns(e, removed)
	if len(items) == 0 {
		m.proposeUnset(e)
		m.set.pend.warns = warns
		m.set.back = setListEdit
		return
	}
	m.propose(e, settings.JoinList(items), setListEdit)
	m.set.pend.warns = append(m.set.pend.warns, warns...)
}

// confirmSettingKey — y применяет (если подтверждение видно целиком), остальное отменяет.
func (m *Model) confirmSettingKey(k string) tea.Cmd {
	if k == "y" {
		if m.settingsConfirmFits() {
			return m.applySetting()
		}
		return nil
	}
	m.set.mode = m.set.back
	if m.set.mode == setNumber {
		m.set.input.Focus()
	}
	return nil
}

// applySetting запускает bd config set или unset (DetailTimeout, корневой контекст).
func (m *Model) applySetting() tea.Cmd {
	p := m.set.pend
	m.set.mode, m.set.saving, m.set.saveKey = setMain, true, p.key
	m.set.items, m.set.adding = nil, false
	m.set.input.Blur()
	delete(m.set.rowErr, p.key)
	src, root, d, t := m.o.Source, m.ctx, m.o.DetailTimeout, m.t
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(root, d)
		defer cancel()
		var err error
		if p.unset {
			err = src.ConfigUnset(ctx, p.key)
		} else {
			err = src.SetConfig(ctx, p.key, p.value)
		}
		return settingSavedMsg{key: p.key, err: timeoutErr(ctx, err, d, t)}
	}
}

// onSettingSaved — ответ записи: ошибка — под строкой; в любом случае настройки
// перечитываются (saving держится до ответа); после успешной смены режима ID, статусов
// или типов перечитывается и список задач.
func (m *Model) onSettingSaved(msg settingSavedMsg) tea.Cmd {
	if !m.set.open || !m.set.saving || msg.key != m.set.saveKey {
		return nil
	}
	if msg.err != nil {
		m.set.rowErr[msg.key] = m.t.SettingFailed(cleanErr(msg.err))
	}
	cmds := []tea.Cmd{m.loadSettings(false)}
	if msg.err == nil && slices.Contains(reloadKeys, msg.key) {
		cmds = append(cmds, m.reloadFresh())
	}
	return tea.Batch(cmds...)
}
