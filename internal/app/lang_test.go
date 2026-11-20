package app

import (
	"strings"
	"testing"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/i18n"
)

// fixtureLatin — фикстура без кириллицы: в английском интерфейсе кириллицы не должно быть вовсе.
const fixtureLatin = `[
{"id":"ep","title":"Commands and table","status":"in_progress","priority":1,"issue_type":"epic",
 "created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-10T10:00:00Z"},
{"id":"ep.1","title":"Record card","status":"open","priority":2,"issue_type":"task","parent":"ep","comment_count":1,
 "dependencies":[{"issue_id":"ep.1","depends_on_id":"ep","type":"parent-child"},{"issue_id":"ep.1","depends_on_id":"bl","type":"blocks"}],
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-11T10:00:00Z"},
{"id":"ep.2","title":"Framed search","status":"closed","priority":2,"issue_type":"task","parent":"ep",
 "created_at":"2026-11-03T10:00:00Z","updated_at":"2026-11-12T10:00:00Z","closed_at":"2026-11-12T10:00:00Z","close_reason":"done"},
{"id":"bl","title":"Blocker","status":"open","priority":0,"issue_type":"bug",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"},
{"id":"x","title":"Closed one","status":"closed","priority":2,"issue_type":"task","parent":"zz",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z","closed_at":"2026-11-06T10:00:00Z"}
]`

const fixtureLatinShow = `[{"id":"ep.1",
 "dependencies":[{"id":"bl","title":"Blocker","status":"open","priority":0,"dependency_type":"blocks"}],
 "dependents":[]}]`

const fixtureLatinComments = `[{"id":1,"issue_id":"ep.1","author":"agent","text":"First comment","created_at":"2026-11-12T10:00:00Z"}]`

func hasCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

// noCyrillic проверяет текущий экран; stage — где мы, для сообщения.
func (h *harness) noCyrillic(stage string) {
	h.t.Helper()
	if v := h.view(); hasCyrillic(v) {
		h.t.Fatalf("%s: кириллица в английском интерфейсе:\n%s", stage, v)
	}
}

func latinHarness(t *testing.T, opts ...func(*Options)) *harness {
	t.Helper()
	h := prepareFlatHarness(t, fixtureLatin, opts...)
	h.fake.SetShow("ep.1", fixtureLatinShow)
	h.fake.SetComments("ep.1", fixtureLatinComments)
	return h
}

func TestNoCyrillicInEnglishUI(t *testing.T) {
	// Загрузка.
	h := latinHarness(t)
	h.noCyrillic("до размера окна")
	h.holdList = true
	h.start()
	h.wantView("Loading tasks…")
	h.noCyrillic("загрузка")

	// Экран ошибки первой загрузки и таймаут.
	h = latinHarness(t)
	h.fake.Fail("list", `Error: database "x" not found`, 1)
	h.start()
	h.wantView("bd exited with an error", "r — retry · q — quit")
	h.noCyrillic("экран ошибки")

	// Без базы: предложение, подтверждение, запуск, ошибка и успех bd init.
	h = latinHarness(t, initAllowed)
	h.fake.Fail("list", noDBStderr, 1)
	h.start()
	h.wantView("No beads database in this project.", "i — initialize (bd init)")
	h.noCyrillic("без базы")
	h.keys("i")
	h.wantView("Initialize beads in /w/proj?", "prefix: proj", "IDs: hash (bd default)",
		"y — run bd init · c — switch IDs · n — cancel")
	h.noCyrillic("подтверждение bd init")
	h.keys("c")
	h.wantView("IDs: sequential 1, 2, 3")
	h.noCyrillic("подтверждение bd init, последовательные ID")
	h.keys("c")
	h.fake.Fail("init", "Error: dolt down", 1)
	h.keys("y")
	h.wantView("bd init failed: exit status 1", "Error: dolt down")
	h.noCyrillic("ошибка bd init")
	h.fake.Fail("init", "", 0)
	h.holdInit = true
	h.keys("i", "y")
	h.wantView("initializing…")
	h.noCyrillic("bd init идёт")
	h.fake.Fail("list", "", 0)
	h.holdInit, h.holdList = false, true
	h.release()
	h.wantView("bd initialized", "Loading tasks…")
	h.noCyrillic("bd init успешен")
	h.holdList = false
	h.release()
	h.noCyrillic("дерево после bd init")
	h = latinHarness(t, initAllowed, func(o *Options) { o.InitTimeout = 300 * time.Millisecond })
	h.fake.Fail("list", noDBStderr, 1)
	h.fake.Sleep("init", 5)
	h.start()
	h.keys("i", "y")
	h.wantView("bd init failed: bd did not respond in 0.3 s")
	h.noCyrillic("таймаут bd init")

	h = latinHarness(t, func(o *Options) { o.ListTimeout = 200 * time.Millisecond })
	h.fake.Sleep("list", 2)
	h.start()
	h.wantView("bd did not respond in 0.2 s")
	h.noCyrillic("таймаут")

	// v1.2: ошибка bd config set после init — на экране после init и в статус-строке.
	h = latinHarness(t, initAllowed)
	h.fake.Fail("list", noDBStderr, 1)
	h.start()
	h.fake.Fail("config-set", "Error: unknown key", 1)
	h.fake.Fail("list", "", 0)
	h.holdList = true
	h.keys("i", "c", "y")
	h.wantView("⚠ sequential IDs not enabled: Error: unknown key")
	h.noCyrillic("ошибка bd config")
	h.holdList = false
	h.release()
	h.wantStatus("sequential IDs not enabled")
	h.noCyrillic("дерево после ошибки bd config")

	// Главный экран, карточка с датами, связями и комментариями.
	h = latinHarness(t)
	h.start()
	h.wantView("Tasks (", "updated 15:04:05", "? help")
	h.keys("j")
	h.wantSelected("ep.1")
	h.wantView("created 2026-11-02", "updated 2026-11-11", "Blocked by", "First comment")
	h.noCyrillic("карточка")
	// v1.2: знак ⊘, счётчик ждущих, строка карточки, фильтр R и его пустое дерево.
	h.wantView("ep.1 ⊘ Record card", "⊘ 1 waiting", "⊘ waiting on 1 open blocker")

	// Закрытые: дата закрытия в карточке и переключатель в статус-строке.
	h.keys("c")
	h.wantStatus("closed")
	h.keys("j")
	h.wantSelected("ep.2")
	h.wantView("closed 2026-11-12 (done)")
	h.noCyrillic("закрытая карточка")
	h.keys("c")

	// Поиск без совпадений и закреплённый фильтр.
	h.keys("/", "q", "q", "q")
	h.wantView("Nothing found: qqq")
	h.noCyrillic("поиск без совпадений")
	h.keys("esc", "/", "b", "l", "o", "enter")
	h.wantView("filter: blo")
	h.noCyrillic("фильтр")
	h.keys("esc")

	// Справка.
	h.keys("?")
	h.wantView("copy ID", "back to tree", "quit")
	h.noCyrillic("справка")
	h.keys("x")

	// Тост копирования и ошибка копирования.
	h.keys("y")
	h.wantView("copied ID: ")
	h.noCyrillic("тост копирования")
	// v1.2: тосты Y/D/M, «нечего копировать» и пометка выключенного перехвата мыши.
	for _, c := range []struct{ key, want string }{
		{"Y", "copied title"}, {"M", "copied card"}, {"D", "nothing to copy"}, {"S", "mouse off"},
	} {
		h.keys(c.key)
		h.wantView(c.want)
		h.noCyrillic("v1.2: " + c.want)
	}
	h.keys("S")

	// Ошибка обновления.
	h.fake.Fail("list", "Error: dolt down", 1)
	h.keys("r")
	h.wantView("update failed: Error: dolt down")
	h.noCyrillic("ошибка обновления")

	// Ошибки подробностей в карточке.
	h = latinHarness(t)
	h.fake.Fail("show", "boom", 1)
	h.fake.Fail("comments", "boom", 1)
	h.start()
	h.keys("j")
	h.wantView("failed to load links: boom", "failed to load comments: boom")
	h.noCyrillic("ошибки подробностей")

	// Пустой ответ bd show: техническая ошибка bd тоже без кириллицы.
	h = latinHarness(t)
	h.fake.SetShow("ep.1", `[]`)
	h.start()
	h.keys("j")
	h.wantSelected("ep.1")
	h.wantView("failed to load links: bd show ep.1: empty response")
	h.noCyrillic("пустой ответ show")

	// Битый JSON списка: экран ошибки первой загрузки и ошибка фонового обновления.
	h = prepareFlatHarness(t, "not json")
	h.start()
	h.wantView("bd exited with an error", "parse JSON")
	h.noCyrillic("битый JSON, первая загрузка")
	h = latinHarness(t)
	h.start()
	h.fake.SetList("not json")
	h.keys("r")
	h.wantView("update failed: ", "parse JSON")
	h.noCyrillic("битый JSON, обновление")

	// Справка в узком окне (одна колонка) и с прокруткой (не влезает по высоте).
	h = latinHarness(t)
	h.start()
	h.send(tea.WindowSizeMsg{Width: 50, Height: 40})
	h.keys("?")
	h.wantView("move", "copy ID")
	h.noCyrillic("справка в узком окне")
	// v1.2: в одну колонку справка длиннее 40 строк — конец на второй странице.
	h.keys("pgdown")
	h.wantView("back to tree", "Shift+drag")
	h.noCyrillic("справка в узком окне, вторая страница")
	h.keys("x")
	h.send(tea.WindowSizeMsg{Width: 80, Height: 10})
	h.keys("?")
	h.wantView("Help · j k PgUp PgDn scroll, other keys close")
	h.noCyrillic("справка с прокруткой")
	h.keys("pgdown")
	h.noCyrillic("справка с прокруткой, вторая страница")
	h.keys("x")

	// Пустое дерево, нет задач, задача не найдена.
	h = newFlatHarness(t, `[{"id":"x","title":"Closed","status":"closed","priority":2,"issue_type":"task"}]`)
	h.wantView("No open tasks — press c to show closed")
	h.noCyrillic("пустое дерево")
	h = newFlatHarness(t, `[]`)
	h.wantView("No tasks")
	h.noCyrillic("нет задач")
	h = newFlatHarness(t, `[{"id":"w","title":"Waits","status":"open","priority":1,"issue_type":"task",
 "dependencies":[{"issue_id":"w","depends_on_id":"gone","type":"blocks"}]}]`)
	h.keys("R")
	h.wantView("No ready tasks — press R to show all")
	h.noCyrillic("нет готовых")
	h = newFlatHarness(t, fixtureLatin, func(o *Options) { o.InitialID = "nope" })
	h.wantView("task nope not found")
	h.noCyrillic("не найдена")

	// Ошибка копирования.
	h = newFlatHarness(t, fixtureLatin, func(o *Options) {
		o.Copy = func(string) error { return errTest("nope") }
	})
	h.keys("y")
	h.wantView("copy failed: nope")
	h.noCyrillic("ошибка копирования")

	// Группы: заголовки, сводка группы, пометка «без групп».
	h = latinHarness(t)
	h.start()
	h.wantView("no groups")
	h.noCyrillic("без групп")
	h.keys("p")
	h.wantView("▾ P0 · 1", "▾ P1 · 2")
	h.keys("k")
	h.wantSelected("#P1")
	h.wantCardLine0("2 tasks")
	h.wantCard("◐ 1 in progress · ○ 1 open")
	h.noCyrillic("сводка группы")

	// v1.2: фильтр R (последним: новый фейк bd встаёт первым в PATH).
	h = latinHarness(t)
	h.start()
	h.keys("R")
	h.wantStatus("ready")
	h.noCyrillic("фильтр готовых")
}

// То же с группами по умолчанию: заголовки групп, карточка группы, карточки задач,
// верхняя строка, режимы, поиск, справка, узкий режим и маленькое окно.
func TestNoCyrillicInEnglishUIGrouped(t *testing.T) {
	h := prepareHarness(t, fixtureLatin, func(o *Options) { o.Project = "latin" })
	h.fake.SetShow("ep.1", fixtureLatinShow)
	h.fake.SetComments("ep.1", fixtureLatinComments)
	h.start()
	h.wantView("▾ P0 · 1", "▾ P1 · 2", "Tasks (", "in progress", "blocked", "/ search · m comment · ? help")
	h.noCyrillic("главный экран")
	h.keys("k")
	h.wantSelected("#P0")
	h.wantCardLine0("1 task")
	h.noCyrillic("карточка группы P0")
	h.keys("j", "j")
	h.wantSelected("#P1")
	h.wantCardLine0("2 tasks")
	h.noCyrillic("карточка группы P1")
	h.keys("j", "j")
	h.wantSelected("ep.1")
	h.wantCard("OPEN", "Blocked by", "First comment")
	h.noCyrillic("карточка задачи")
	h.keys("c", "e")
	h.wantStatus("closed", "events")
	h.wantView("✓ ")
	h.noCyrillic("закрытые и события")
	h.keys("/", "b", "l")
	h.noCyrillic("ввод")
	h.keys("enter")
	h.wantStatus("filter: bl")
	h.noCyrillic("фильтр")
	h.keys("esc", "k", "h")
	h.noCyrillic("свёрнутая группа")
	h.keys("?")
	h.noCyrillic("справка")
	h.keys("x")
	h.send(tea.WindowSizeMsg{Width: 70, Height: 24})
	h.noCyrillic("узкий режим")
	h.send(tea.WindowSizeMsg{Width: 10, Height: 3})
	h.wantView("window too")
	h.noCyrillic("маленькое окно")
}

type errTest string

func (e errTest) Error() string { return string(e) }

func TestRussianUI(t *testing.T) {
	ru := func(o *Options) { o.Lang = i18n.RU }
	h := newFlatHarness(t, fixtureMain, ru)
	h.wantView("Задачи (6)", "обновлено 15:04:05", "? справка")
	h.keys("j")
	h.wantView("создана 2026-11-02", "обновлена 2026-11-11")
	h.keys("y")
	h.wantView("скопирован ID: ep.1")
	h.keys("?")
	h.wantView("копировать ID", "назад к дереву")

	h = prepareFlatHarness(t, fixtureMain, ru)
	h.holdList = true
	h.start()
	h.wantView("Загрузка задач…")

	h = newFlatHarness(t, `[]`, ru)
	h.wantView("Нет задач")
}

// Английский по умолчанию: нулевой Lang.
func TestDefaultLangEN(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.wantView("Tasks (6)")
	if strings.Contains(h.view(), "Задачи") {
		t.Fatal("по умолчанию русский")
	}
}
