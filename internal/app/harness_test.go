package app

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/testbd"
)

func TestMain(m *testing.M) {
	// Без цветов: View сравнивается как простой текст.
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

// fixtureMain — основная фикстура по схеме bd v0.59.
const fixtureMain = `[
{"id":"ep","title":"Команды и таблица","status":"in_progress","priority":1,"issue_type":"epic",
 "created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-10T10:00:00Z"},
{"id":"ep.1","title":"Карточка Записи","status":"open","priority":2,"issue_type":"task","parent":"ep","comment_count":1,
 "dependencies":[{"issue_id":"ep.1","depends_on_id":"ep","type":"parent-child"},{"issue_id":"ep.1","depends_on_id":"bl","type":"blocks"}],
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-11T10:00:00Z"},
{"id":"ep.2","title":"Поиск в рамке","status":"closed","priority":2,"issue_type":"task","parent":"ep",
 "created_at":"2026-11-03T10:00:00Z","updated_at":"2026-11-12T10:00:00Z","closed_at":"2026-11-12T10:00:00Z","close_reason":"done"},
{"id":"ep.3","title":"State change","status":"closed","priority":4,"issue_type":"event","parent":"ep",
 "created_at":"2026-11-04T10:00:00Z","updated_at":"2026-11-04T10:00:00Z","closed_at":"2026-11-04T10:00:00Z"},
{"id":"old","title":"Старый эпик","status":"closed","priority":2,"issue_type":"epic",
 "created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-20T10:00:00Z","closed_at":"2026-10-20T10:00:00Z"},
{"id":"old.1","title":"Хвост старого эпика","status":"open","priority":2,"issue_type":"task","parent":"old",
 "created_at":"2026-10-02T10:00:00Z","updated_at":"2026-10-02T10:00:00Z"},
{"id":"bl","title":"Блокер","status":"open","priority":0,"issue_type":"bug",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"},
{"id":"lone","title":"Контекст компании для длинного заголовка, который точно не влезет в колонку","status":"open","priority":3,"issue_type":"task",
 "created_at":"2026-11-06T10:00:00Z","updated_at":"2026-11-06T10:00:00Z"}
]`

// fixtureMainV2 — вторая версия списка (Task 7): ep.1 закрыта, добавлена new, lone удалена.
const fixtureMainV2 = `[
{"id":"ep","title":"Команды и таблица","status":"in_progress","priority":1,"issue_type":"epic",
 "created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-10T10:00:00Z"},
{"id":"ep.1","title":"Карточка Записи","status":"closed","priority":2,"issue_type":"task","parent":"ep","comment_count":1,
 "dependencies":[{"issue_id":"ep.1","depends_on_id":"ep","type":"parent-child"},{"issue_id":"ep.1","depends_on_id":"bl","type":"blocks"}],
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-19T15:00:00Z","closed_at":"2026-11-19T15:00:00Z"},
{"id":"ep.2","title":"Поиск в рамке","status":"closed","priority":2,"issue_type":"task","parent":"ep",
 "created_at":"2026-11-03T10:00:00Z","updated_at":"2026-11-12T10:00:00Z","closed_at":"2026-11-12T10:00:00Z","close_reason":"done"},
{"id":"ep.3","title":"State change","status":"closed","priority":4,"issue_type":"event","parent":"ep",
 "created_at":"2026-11-04T10:00:00Z","updated_at":"2026-11-04T10:00:00Z","closed_at":"2026-11-04T10:00:00Z"},
{"id":"old","title":"Старый эпик","status":"closed","priority":2,"issue_type":"epic",
 "created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-20T10:00:00Z","closed_at":"2026-10-20T10:00:00Z"},
{"id":"old.1","title":"Хвост старого эпика","status":"open","priority":2,"issue_type":"task","parent":"old",
 "created_at":"2026-10-02T10:00:00Z","updated_at":"2026-10-02T10:00:00Z"},
{"id":"bl","title":"Блокер","status":"open","priority":0,"issue_type":"bug",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"},
{"id":"new","title":"Новая задача","status":"open","priority":1,"issue_type":"task",
 "created_at":"2026-11-19T14:00:00Z","updated_at":"2026-11-19T14:00:00Z"}
]`

// fixtureShowEp1 — ответ bd show ep.1 (ruling R3).
const fixtureShowEp1 = `[{"id":"ep.1",
 "dependencies":[
  {"id":"bl","title":"Блокер","status":"open","priority":0,"dependency_type":"blocks"},
  {"id":"ep","title":"Команды и таблица","status":"in_progress","priority":1,"dependency_type":"parent-child"}],
 "dependents":[]}]`

const fixtureCommentsEp1 = `[{"id":1,"issue_id":"ep.1","author":"agent","text":"Первый комментарий","created_at":"2026-11-12T10:00:00Z"}]`

// fixedNow — момент «сейчас» во всех тестах.
var fixedNow = time.Date(2026, 11, 19, 15, 4, 5, 0, time.Local)

type harness struct {
	t        *testing.T
	m        Model
	fake     *testbd.Fake
	timers   []tea.Msg
	held     []tea.Msg
	holdList bool
	holdAdd  bool // ответы bd comments add задерживаются в held, как list при holdList
	holdShow bool // ответы bd show задерживаются в held
	holdInit bool // ответы bd init задерживаются в held
	holdSet  bool // ответы чтения настроек задерживаются в held
	holdSave bool // ответы bd config set/unset с экрана настроек задерживаются в held
	holdTab  bool // ответы чтений вкладок Memory и Formulas задерживаются в held
	quit     bool
	copied   []string     // что модель отдала в Options.Copy (подмена по умолчанию)
	term     bytes.Buffer // что модель написала в терминал мимо Bubble Tea (Options.Term)
}

// prepareHarness создаёт фейк и модель, но не запускает Init: тест может
// настроить фейк или holdList до первой загрузки и затем вызвать start.
func prepareHarness(t *testing.T, fixture string, opts ...func(*Options)) *harness {
	t.Helper()
	h := &harness{t: t, fake: testbd.New(t)}
	h.fake.SetList(fixture)
	h.fake.SetShow("ep.1", fixtureShowEp1)
	h.fake.SetComments("ep.1", fixtureCommentsEp1)
	o := Options{
		Source:        bd.Client{},
		After:         h.record,
		MarkdownStyle: "notty",
		Now:           func() time.Time { return fixedNow },
		// Настоящий буфер обмена тестам не нужен: по умолчанию копирование только записывается.
		Copy: func(s string) error {
			h.copied = append(h.copied, s)
			return nil
		},
		// Сырые последовательности терминала — в буфер, а не в stdout теста.
		Term: &h.term,
	}
	for _, f := range opts {
		f(&o)
	}
	h.m = New(o)
	return h
}

// start отправляет размер окна и выполняет Init.
func (h *harness) start() *harness {
	h.send(tea.WindowSizeMsg{Width: 120, Height: 40})
	h.run(h.m.Init())
	return h
}

func newHarness(t *testing.T, fixture string, opts ...func(*Options)) *harness {
	t.Helper()
	return prepareHarness(t, fixture, opts...).start()
}

// prepareFlatHarness — prepareHarness с выключенными группами (клавиша p до старта):
// для тестов, ожидания которых построены на порядке v1.
func prepareFlatHarness(t *testing.T, fixture string, opts ...func(*Options)) *harness {
	t.Helper()
	h := prepareHarness(t, fixture, opts...)
	h.m.opts.GroupByPriority = false
	return h
}

// newFlatHarness — newHarness с выключенными группами.
func newFlatHarness(t *testing.T, fixture string, opts ...func(*Options)) *harness {
	t.Helper()
	return prepareFlatHarness(t, fixture, opts...).start()
}

// record — подмена Options.After: таймер не идёт, сообщение копится в h.timers.
func (h *harness) record(_ time.Duration, msg tea.Msg) tea.Cmd {
	h.timers = append(h.timers, msg)
	return nil
}

// run синхронно выполняет команду и отправляет её результат в модель.
func (h *harness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			h.run(c)
		}
	case tea.QuitMsg:
		h.quit = true
	case listMsg:
		if h.holdList {
			h.held = append(h.held, msg)
			return
		}
		h.send(msg)
	case showMsg:
		if h.holdShow {
			h.held = append(h.held, msg)
			return
		}
		h.send(msg)
	case initMsg:
		if h.holdInit {
			h.held = append(h.held, msg)
			return
		}
		h.send(msg)
	case settingsMsg:
		if h.holdSet {
			h.held = append(h.held, msg)
			return
		}
		h.send(msg)
	case settingSavedMsg:
		if h.holdSave {
			h.held = append(h.held, msg)
			return
		}
		h.send(msg)
	case memoriesMsg, formulasMsg:
		if h.holdTab {
			h.held = append(h.held, msg)
			return
		}
		h.send(msg)
	case addCommentMsg:
		if h.holdAdd {
			h.held = append(h.held, msg)
			return
		}
		h.send(msg)
	default:
		h.send(msg)
	}
}

func (h *harness) send(msg tea.Msg) {
	m, cmd := h.m.Update(msg)
	h.m = m.(Model)
	h.run(cmd)
}

var namedKeys = map[string]tea.KeyType{
	"down": tea.KeyDown, "up": tea.KeyUp, "left": tea.KeyLeft, "right": tea.KeyRight,
	"enter": tea.KeyEnter, "tab": tea.KeyTab, "esc": tea.KeyEsc,
	"pgdown": tea.KeyPgDown, "pgup": tea.KeyPgUp, "ctrl+c": tea.KeyCtrlC, "backspace": tea.KeyBackspace,
	"ctrl+s": tea.KeyCtrlS,
}

func (h *harness) keys(ks ...string) {
	for _, k := range ks {
		if k == "space" {
			h.send(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		} else if k == "alt+enter" {
			h.send(tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
		} else if t, ok := namedKeys[k]; ok {
			h.send(tea.KeyMsg{Type: t})
		} else {
			h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		}
	}
}

// tick отправляет накопленные таймеры.
func (h *harness) tick() {
	ts := h.timers
	h.timers = nil
	for _, m := range ts {
		h.send(m)
	}
}

// release отправляет задержанные ответы list, show, comments add, init и настроек.
func (h *harness) release() {
	hs := h.held
	h.held = nil
	for _, m := range hs {
		h.send(m)
	}
}

func (h *harness) view() string { return h.m.View() }

// --- экран по частям ---
// Раскладка: верхняя строка, строка вкладок (v1.4), рамки панелей (верхняя граница,
// тело, нижняя граница), статус-строка. Хелперы снимают рамки, чтобы проверки смотрели
// в содержимое панели.

// viewLines — строки View.
func (h *harness) viewLines() []string { return strings.Split(h.view(), "\n") }

// topBar — верхняя строка: имя проекта и сводка статусов.
func (h *harness) topBar() string { return h.viewLines()[0] }

// tabBar — строка вкладок под верхней строкой.
func (h *harness) tabBar() string { return h.viewLines()[1] }

// frameTop — номер строки верхней границы панелей: первая строка с «╭» после верхней.
func (h *harness) frameTop() int {
	for i, l := range h.viewLines() {
		if i > 0 && strings.HasPrefix(l, "╭") {
			return i
		}
	}
	return 1
}

// statusLine — нижняя статус-строка.
func (h *harness) statusLine() string {
	ls := h.viewLines()
	return ls[len(ls)-1]
}

// panes — колонки [left, right) панелей: дерево и карточка; в узком режиме одна,
// вторая пустая (left == right).
func (h *harness) panes() (tree, card [2]int) {
	w := h.m.width
	if h.m.narrow() {
		if h.m.focus == focusDetail {
			return [2]int{0, 0}, [2]int{0, w}
		}
		return [2]int{0, w}, [2]int{w, w}
	}
	tw := h.m.treeWidth()
	return [2]int{0, tw}, [2]int{tw, w}
}

// paneBody — строки тела панели [l, r) без рамки и без отступов ширины pad по краям.
// Тело — treeHeight строк под верхней границей: ниже может стоять поле комментария.
func (h *harness) paneBody(c [2]int, pad int) []string {
	if c[0] == c[1] {
		return nil
	}
	ls := h.viewLines()
	n := h.m.treeHeight()
	top := h.frameTop() + 1
	if len(ls) < 5 || n == 0 || top+n > len(ls) {
		return nil
	}
	var out []string
	for _, l := range ls[top : top+n] {
		out = append(out, ansi.Cut(l, c[0]+1+pad, c[1]-1-pad))
	}
	return out
}

// paneTitle — заголовок рамки панели [l, r): текст верхней границы без линий и уголков.
func (h *harness) paneTitle(c [2]int) string {
	if c[0] == c[1] {
		return ""
	}
	top := ansi.Cut(h.viewLines()[h.frameTop()], c[0], c[1])
	return strings.TrimSpace(strings.Trim(top, "╭╮─"))
}

// treeLines — строки дерева без рамки; nil, если дерево не показано.
func (h *harness) treeLines() []string {
	t, _ := h.panes()
	return h.paneBody(t, 0)
}

// cardLines — строки карточки без рамки и полей; nil, если карточка не показана.
func (h *harness) cardLines() []string {
	_, c := h.panes()
	return h.paneBody(c, 1)
}

func (h *harness) treeTitle() string { t, _ := h.panes(); return h.paneTitle(t) }
func (h *harness) cardTitle() string { _, c := h.panes(); return h.paneTitle(c) }

// --- проверки ---

// wantCardOf — карточка задачи id: полный ID в рамке, заголовок задачи первой строкой.
func (h *harness) wantCardOf(id, title string) {
	h.t.Helper()
	if got := strings.TrimPrefix(h.cardTitle(), activeMark+" "); got != id {
		h.t.Fatalf("заголовок рамки карточки = %q, want %q:\n%s", got, id, h.view())
	}
	if ls := h.cardLines(); len(ls) == 0 || strings.TrimSpace(ls[0]) != title {
		h.t.Fatalf("первая строка карточки не %q:\n%s", title, h.view())
	}
}

// wantCardLine0 — первая строка карточки ровно want.
func (h *harness) wantCardLine0(want string) {
	h.t.Helper()
	if ls := h.cardLines(); len(ls) == 0 || strings.TrimSpace(ls[0]) != want {
		h.t.Fatalf("первая строка карточки не %q:\n%s", want, h.view())
	}
}

func (h *harness) wantCard(subs ...string) {
	h.t.Helper()
	c := strings.Join(h.cardLines(), "\n")
	for _, s := range subs {
		if !strings.Contains(c, s) {
			h.t.Fatalf("карточка не содержит %q:\n%s", s, c)
		}
	}
}

func (h *harness) wantNoCard(subs ...string) {
	h.t.Helper()
	c := strings.Join(h.cardLines(), "\n")
	for _, s := range subs {
		if strings.Contains(c, s) {
			h.t.Fatalf("карточка содержит %q:\n%s", s, c)
		}
	}
}

func (h *harness) wantStatus(subs ...string) {
	h.t.Helper()
	l := h.statusLine()
	for _, s := range subs {
		if !strings.Contains(l, s) {
			h.t.Fatalf("статус-строка %q не содержит %q", l, s)
		}
	}
}

func (h *harness) wantNoStatus(subs ...string) {
	h.t.Helper()
	l := h.statusLine()
	for _, s := range subs {
		if strings.Contains(l, s) {
			h.t.Fatalf("статус-строка %q содержит %q", l, s)
		}
	}
}

func (h *harness) wantIDs(want ...string) {
	h.t.Helper()
	if got := h.m.VisibleIDs(); !slices.Equal(got, want) {
		h.t.Fatalf("VisibleIDs = %v, want %v", got, want)
	}
}

func (h *harness) wantSelected(want string) {
	h.t.Helper()
	if got := h.m.SelectedID(); got != want {
		h.t.Fatalf("SelectedID = %q, want %q", got, want)
	}
}

func (h *harness) wantView(subs ...string) {
	h.t.Helper()
	v := h.view()
	for _, s := range subs {
		if !strings.Contains(v, s) {
			h.t.Fatalf("view не содержит %q:\n%s", s, v)
		}
	}
}

func (h *harness) wantNoView(subs ...string) {
	h.t.Helper()
	v := h.view()
	for _, s := range subs {
		if strings.Contains(v, s) {
			h.t.Fatalf("view содержит %q:\n%s", s, v)
		}
	}
}

// wantTab — активная вкладка (Model.Tab).
func (h *harness) wantTab(want string) {
	h.t.Helper()
	if got := h.m.Tab(); got != want {
		h.t.Fatalf("Tab = %q, want %q", got, want)
	}
}

// countShow — сколько раз вызван bd show для задачи id.
func (h *harness) countShow(id string) int {
	n := 0
	for _, c := range h.fake.Calls() {
		if i := slices.Index(c, "show"); i >= 0 && i+1 < len(c) && c[i+1] == id {
			n++
		}
	}
	return n
}

func (h *harness) wantShow(id string, want int) {
	h.t.Helper()
	if got := h.countShow(id); got != want {
		h.t.Fatalf("countShow(%q) = %d, want %d", id, got, want)
	}
}

func (h *harness) wantCount(cmd string, want int) {
	h.t.Helper()
	if got := h.fake.Count(cmd); got != want {
		h.t.Fatalf("Count(%q) = %d, want %d", cmd, got, want)
	}
}
