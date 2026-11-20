package app

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/i18n"
)

// fixtureSpecs — спеки, решения и шаблон по схеме bd v0.59. Спека sp: три тикета
// (один закрыт) и служебное событие, которое тикетом не считается; tpl — эпик-шаблон.
const fixtureSpecs = `[
{"id":"sp","title":"Спека вкладок","status":"in_progress","priority":1,"issue_type":"epic","description":"# Спека\nописание спеки",
 "created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-10T10:00:00Z"},
{"id":"sp.1","title":"Тикет один","status":"open","priority":2,"issue_type":"task","parent":"sp",
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-02T10:00:00Z"},
{"id":"sp.2","title":"Тикет два","status":"in_progress","priority":2,"issue_type":"task","parent":"sp",
 "created_at":"2026-11-03T10:00:00Z","updated_at":"2026-11-03T10:00:00Z"},
{"id":"sp.3","title":"Тикет три","status":"closed","priority":2,"issue_type":"task","parent":"sp",
 "created_at":"2026-11-04T10:00:00Z","updated_at":"2026-11-04T10:00:00Z","closed_at":"2026-11-04T10:00:00Z"},
{"id":"sp.4","title":"State change","status":"closed","priority":4,"issue_type":"event","parent":"sp",
 "created_at":"2026-11-04T10:00:00Z","updated_at":"2026-11-04T10:00:00Z","closed_at":"2026-11-04T10:00:00Z"},
{"id":"sp2","title":"Пустая спека","status":"open","priority":2,"issue_type":"epic",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"},
{"id":"tpl","title":"Шаблон эпика","status":"open","priority":2,"issue_type":"epic","labels":["template"],
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"},
{"id":"old","title":"Закрытая спека","status":"closed","priority":2,"issue_type":"epic",
 "created_at":"2026-10-01T10:00:00Z","updated_at":"2026-10-20T10:00:00Z","closed_at":"2026-10-20T10:00:00Z"},
{"id":"dc","title":"Решение: Go","status":"open","priority":2,"issue_type":"decision","description":"Берём Go.",
 "created_at":"2026-11-06T10:00:00Z","updated_at":"2026-11-06T10:00:00Z"},
{"id":"dc2","title":"Старое решение","status":"closed","priority":2,"issue_type":"decision",
 "created_at":"2026-10-06T10:00:00Z","updated_at":"2026-10-06T10:00:00Z","closed_at":"2026-10-07T10:00:00Z"},
{"id":"t1","title":"Просто задача","status":"open","priority":3,"issue_type":"task",
 "created_at":"2026-11-07T10:00:00Z","updated_at":"2026-11-07T10:00:00Z"}
]`

// Правдоподобный ответ bd formula list --json (FormulaListEntry; на настоящем bd форма
// не проверена — в проектах пользователя формул нет) и ответ bd list --include-templates.
const (
	fixtureFormulas  = `[{"name":"mol-release","type":"workflow","description":"Release **checklist**: bump, tag, publish","source":".beads/formulas/mol-release.formula.json","steps":5,"vars":2}]`
	fixtureTemplates = `[
{"id":"tpl","title":"Шаблон эпика","status":"open","priority":2,"issue_type":"epic","labels":["template"],"description":"## Разделы\n- цель",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"},
{"id":"mol","title":"Молекула\u001b[31m красная","status":"open","priority":2,"issue_type":"task","is_template":true,
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"},
{"id":"sp2","title":"Пустая спека","status":"open","priority":2,"issue_type":"epic",
 "created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"}]`
	fixtureMemories = `{"zeta":"последняя запись","alpha":"## Первая\n- пункт памяти"}`
)

func specsHarness(t *testing.T, opts ...func(*Options)) *harness {
	t.Helper()
	return newHarness(t, fixtureSpecs, opts...)
}

// treeText — строки дерева без отступов по краям через пробел: перенесённый текст
// (ошибка в панели) ищется как одна строка.
func (h *harness) treeText() string {
	var out []string
	for _, l := range h.treeLines() {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, " ")
}

func (h *harness) wantTree(subs ...string) {
	h.t.Helper()
	tt := h.treeText()
	for _, s := range subs {
		if !strings.Contains(tt, s) {
			h.t.Fatalf("дерево не содержит %q:\n%s", s, tt)
		}
	}
}

// Строка вкладок под верхней строкой: Tasks, Specs N, Decisions N, Memory, Formulas;
// [ и ] переключают по кругу, вкладка по умолчанию — Tasks.
func TestTabsBarAndCycle(t *testing.T) {
	h := specsHarness(t)
	h.fake.SetMemories(fixtureMemories)
	h.fake.SetFormulas(fixtureFormulas)
	h.fake.SetTemplates(fixtureTemplates)
	h.wantTab("tasks")
	// До первого входа Memory и Formulas без числа: они ещё не загружены.
	if got := strings.TrimSpace(h.tabBar()); got != "Tasks │ Specs 2 │ Decisions 1 │ Memory │ Formulas" {
		t.Fatalf("строка вкладок %q", got)
	}
	for _, want := range []string{"specs", "decisions", "memory", "formulas", "tasks"} {
		h.keys("]")
		h.wantTab(want)
	}
	h.keys("[")
	h.wantTab("formulas")
	h.keys("[", "[", "[", "[")
	h.wantTab("tasks")
	// После загрузки Memory и Formulas строка показывает их число; c меняет Specs N.
	h.keys("c")
	if got := strings.TrimSpace(h.tabBar()); got != "Tasks │ Specs 3 │ Decisions 2 │ Memory 2 │ Formulas 3" {
		t.Fatalf("строка вкладок %q", got)
	}
}

// Вкладки недоступны в режиме ввода, в справке, в поле комментария и на экране настроек.
func TestTabsKeysOnlyOutsideModes(t *testing.T) {
	h := specsHarness(t)
	h.keys("/", "[", "]")
	h.wantTab("tasks")
	h.keys("esc", "?", "]")
	h.wantTab("tasks")
	h.keys("m")
	h.wantBox()
	h.keys("]", "esc")
	h.wantNoBox()
	h.wantTab("tasks")
	h.keys(",", "]", "esc")
	h.wantTab("tasks")
}

// Вкладка Specs: эпики без шаблонов, иконка, ID, заголовок и прогресс по прямым
// тикетам (события не считаются); c показывает закрытые; группы P не применяются.
func TestSpecsListAndProgress(t *testing.T) {
	h := specsHarness(t)
	h.keys("]")
	h.wantTab("specs")
	h.wantIDs("sp", "sp2")
	if got := strings.TrimPrefix(h.treeTitle(), activeMark+" "); got != "Specs (2)" {
		t.Fatalf("заголовок панели %q", got)
	}
	h.wantTree("◐ sp Спека вкладок", "███░░░░░ 1/3", "○ sp2 Пустая спека")
	h.wantNoView("Шаблон эпика")
	h.keys("c")
	h.wantIDs("sp", "sp2", "old")
	h.wantStatus("closed")
	h.keys("c")
	h.wantIDs("sp", "sp2")
}

// Карточка спеки: описание целиком, прогресс в чипах, Tickets со всеми тикетами
// (и закрытыми, без событий) по номерам; цифра переключает на Tasks и выбирает тикет.
func TestSpecCardTicketsJump(t *testing.T) {
	h := specsHarness(t)
	h.keys("]")
	h.wantSelected("sp")
	h.wantCardOf("sp", "Спека вкладок")
	h.wantCard("███░░░░░ 1/3", "Спека", "описание спеки", "Tickets",
		"[1] ◐ P2 sp.2  Тикет два", "[2] ○ P2 sp.1  Тикет один", "[3] ✓ P2 sp.3  Тикет три")
	h.wantNoCard("State change", "Children")
	h.keys("3")
	h.wantTab("tasks")
	h.wantSelected("sp.3")
	h.wantCardOf("sp.3", "Тикет три")
	// Закрытый тикет: включились закрытые.
	h.wantStatus("closed")
	// Вкладка Specs помнит свой выбор.
	h.keys("]")
	h.wantTab("specs")
	h.wantSelected("sp")
}

// Поиск во вкладке Specs фильтрует список и живёт в своей вкладке.
func TestSpecsSearchPerTab(t *testing.T) {
	h := specsHarness(t)
	h.keys("]", "/", "п", "у", "с", "т", "enter")
	h.wantIDs("sp2")
	h.wantStatus("filter: пуст")
	h.keys("[")
	h.wantTab("tasks")
	h.wantNoStatus("filter:")
	if ids := h.m.VisibleIDs(); len(ids) < 5 {
		t.Fatalf("Tasks отфильтрованы запросом Specs: %v", ids)
	}
	h.keys("]")
	h.wantIDs("sp2")
	h.wantStatus("filter: пуст")
	h.keys("esc")
	h.wantIDs("sp", "sp2")
}

// Каждая вкладка помнит курсор и выбор, Tasks — ещё и раскрытие веток и групп.
func TestTabsKeepOwnState(t *testing.T) {
	h := specsHarness(t)
	// Tasks: свернуть группу P1 (спека sp внутри неё).
	h.wantSelected("sp")
	h.keys("k", "h")
	h.wantSelected("#P1")
	tasks := h.m.VisibleIDs()
	h.keys("]", "j")
	h.wantSelected("sp2")
	h.keys("]", "c", "j")
	h.wantTab("decisions")
	h.wantIDs("dc", "dc2")
	h.wantSelected("dc2")
	h.keys("[", "[")
	h.wantTab("tasks")
	h.wantSelected("#P1")
	// c включён глобально: в Tasks видны и закрытые, но P1 по-прежнему свёрнута.
	if ids := h.m.VisibleIDs(); len(ids) <= len(tasks) || h.m.VisibleIDs()[1] != "#P2" {
		t.Fatalf("Tasks после вкладок: %v (до: %v)", ids, tasks)
	}
	h.keys("]")
	h.wantSelected("sp2")
	h.keys("]")
	h.wantSelected("dc2")
}

// R, p и e действуют только в Tasks; в Memory и Formulas не действуют и c, и /.
func TestTabsTaskOnlyKeys(t *testing.T) {
	h := specsHarness(t)
	before := h.m.VisibleIDs()
	h.keys("]", "R", "p", "e")
	h.wantIDs("sp", "sp2")
	h.wantNoStatus("ready", "no groups", "events")
	h.keys("[")
	h.wantIDs(before...)
	h.wantNoStatus("ready", "no groups", "events")
	h.keys("]", "]", "]", "c", "/", "x")
	h.wantTab("memory")
	h.wantNoStatus("closed", "filter")
	h.keys("]", "c", "/")
	h.wantTab("formulas")
	h.wantNoStatus("closed")
	h.keys("]")
	h.wantTab("tasks")
	h.wantIDs(before...)
}

// Вкладка Decisions: задачи типа decision плоским списком, обычная карточка, c и /.
func TestDecisionsTab(t *testing.T) {
	h := specsHarness(t)
	h.keys("]", "]")
	h.wantTab("decisions")
	h.wantIDs("dc")
	h.wantCardOf("dc", "Решение: Go")
	h.wantCard("Берём Go.")
	h.wantTree("○ P2 dc Решение: Go")
	h.keys("c")
	h.wantIDs("dc", "dc2")
	h.keys("/", "с", "т", "а", "р")
	h.wantIDs("dc2")
	// Карточка решения — обычная: bd show запрошен.
	h.keys("enter")
	h.wantShow("dc2", 1)
}

// Memory: загрузка при первом входе (не раньше), ключи слева по порядку, текст справа
// как markdown; повторный вход не перечитывает, r — перечитывает.
func TestMemoryTabLazyLoad(t *testing.T) {
	h := specsHarness(t)
	h.fake.SetMemories(fixtureMemories)
	h.wantCount("memories", 0)
	h.holdTab = true
	h.keys("]", "]", "]")
	h.wantTab("memory")
	h.wantCount("memories", 1)
	h.wantTree("Loading…")
	h.release()
	h.holdTab = false
	h.wantTree("alpha", "zeta")
	h.wantSelected("memory:alpha")
	h.wantCardLine0("alpha")
	h.wantCard("Первая", "• пункт памяти")
	h.wantStatus("[ ] tabs", "r reload", "? help")
	h.keys("j")
	h.wantCard("последняя запись")
	h.keys("[", "]")
	h.wantCount("memories", 1)
	h.wantSelected("memory:zeta")
	// r перечитывает только вкладку; список задач — нет (его обновляет опрос).
	h.fake.SetMemories(`{"zeta":"обновлённая запись"}`)
	lists := h.fake.Count("list")
	h.keys("r")
	h.wantCount("memories", 2)
	h.wantCount("list", lists)
	h.wantSelected("memory:zeta")
	h.wantCard("обновлённая запись")
	h.wantNoView("alpha")
	// Копировать и комментировать в памяти нечего: это не задача.
	h.keys("m")
	h.wantNoBox()
	h.wantShow("memory:zeta", 0)
}

// Пустая память — «No memories»; ошибка — в панели, остальные вкладки работают; r повторяет.
func TestMemoryTabEmptyAndError(t *testing.T) {
	h := specsHarness(t)
	h.keys("[", "[")
	h.wantTab("memory")
	h.wantTree("No memories")
	if !strings.Contains(h.tabBar(), "Memory 0") {
		t.Fatalf("строка вкладок %q", h.tabBar())
	}
	h.fake.Fail("memories", "Error: memory store locked\nподробности", 1)
	h.keys("r")
	h.wantTree("⚠ memories not loaded: Error: memory store locked")
	h.keys("[")
	h.wantTab("decisions")
	h.wantIDs("dc")
	h.keys("]")
	h.fake.Fail("memories", "", 0)
	h.fake.SetMemories(fixtureMemories)
	h.keys("r")
	h.wantTree("alpha", "zeta")
	h.wantNoView("not loaded")
}

// Formulas: формулы (bd formula list) и шаблоны (эпики с меткой template и молекулы
// из bd list --include-templates); имена очищены; справа описание как markdown.
func TestFormulasTab(t *testing.T) {
	h := specsHarness(t)
	h.fake.SetFormulas(fixtureFormulas)
	h.fake.SetTemplates(fixtureTemplates)
	h.keys("[")
	h.wantTab("formulas")
	h.wantCount("formula-list", 1)
	h.wantCount("list-templates", 1)
	h.wantIDs("formula:mol-release", "template:mol", "template:tpl")
	h.wantTree("mol-release", "formula", "Молекула[31m красная", "template", "Шаблон эпика")
	h.wantNoView("Пустая спека  ", "\x1b")
	h.wantCardLine0("mol-release")
	h.wantCard("formula · workflow · .beads/formulas/mol-release.formula.json", "Release", "bump, tag, publish")
	h.keys("G")
	h.wantCardLine0("Шаблон эпика")
	h.wantCard("template · tpl", "Разделы", "• цель")
	// Шаблон — не задача вкладки: bd show не запрашивается.
	h.wantShow("template:tpl", 0)
	h.wantShow("tpl", 0)
	h.wantStatus("r reload")
	h.fake.SetFormulas(`null`)
	h.keys("r")
	h.wantCount("formula-list", 2)
	h.wantIDs("template:mol", "template:tpl")
}

// Formulas: пусто — «No formulas or templates»; ошибка одного из чтений — в панели.
func TestFormulasTabEmptyAndError(t *testing.T) {
	h := specsHarness(t)
	h.keys("[")
	h.wantTree("No formulas or templates")
	h.fake.Fail("formula-list", "Error: bad formula file", 1)
	h.fake.SetTemplates(fixtureTemplates)
	h.keys("r")
	h.wantTree("⚠ formulas not loaded: Error: bad formula file")
	h.keys("]")
	h.wantTab("tasks")
	h.wantSelected("sp")
}

// Все вкладки — в 80x24 и в узком окне 70 на обоих языках, с карточкой в фокусе;
// окно меньше 20x6 — одна строка «window too small».
func TestTabsFit(t *testing.T) {
	for _, l := range []i18n.Lang{i18n.EN, i18n.RU} {
		h := specsHarness(t, func(o *Options) { o.Lang = l; o.Project = "very-long-project-name-for-the-top-bar" })
		h.fake.SetMemories(fixtureMemories)
		h.fake.SetFormulas(fixtureFormulas)
		h.fake.SetTemplates(fixtureTemplates)
		for _, sz := range [][2]int{{80, 24}, {70, 24}} {
			h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			for i := 0; i < 5; i++ {
				what := fmt.Sprintf("%s %dx%d %s", l, sz[0], sz[1], h.m.Tab())
				h.wantFits(sz[0], sz[1], what)
				if sz[0] < 80 {
					h.keys("enter")
				} else {
					h.keys("tab")
				}
				h.wantFits(sz[0], sz[1], what+" карточка")
				if sz[0] < 80 {
					h.keys("esc")
				} else {
					h.keys("tab")
				}
				h.keys("]")
			}
		}
		h.send(tea.WindowSizeMsg{Width: 19, Height: 5})
		h.keys("]")
		if v := h.view(); v != ansi.Truncate(i18n.For(l).TooSmall, 19, "") {
			t.Fatalf("маленькое окно: %q", v)
		}
	}
}

// Справка в 80x24 на обоих языках влезает и называет [ / ].
func TestHelpTabsFits80x24(t *testing.T) {
	for _, c := range []struct {
		lang   i18n.Lang
		action string
	}{{i18n.EN, "tabs"}, {i18n.RU, "вкладки"}} {
		h := specsHarness(t, func(o *Options) { o.Lang = c.lang })
		h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
		h.keys("?")
		h.wantFits(80, 24, "справка "+c.lang.String())
		h.wantHelpEntry("[ / ]", c.action)
		h.wantNoView("Help · j k", "Справка · j k")
	}
}

// Русские названия вкладок и тексты панелей.
func TestTabsRussian(t *testing.T) {
	h := specsHarness(t, func(o *Options) { o.Lang = i18n.RU })
	if got := strings.TrimSpace(h.tabBar()); got != "Задачи │ Спеки 2 │ Решения 1 │ Память │ Формулы" {
		t.Fatalf("строка вкладок %q", got)
	}
	h.keys("]")
	h.wantTree("███░░░░░ 1/3")
	if got := strings.TrimPrefix(h.treeTitle(), activeMark+" "); got != "Спеки (2)" {
		t.Fatalf("заголовок панели %q", got)
	}
	h.keys("]", "]")
	h.wantTree("Памяти нет")
	h.wantStatus("[ ] вкладки", "r обновить")
	h.keys("]")
	h.wantTree("Нет формул и шаблонов")
}

// Английский интерфейс без кириллицы во всех вкладках (данные латиницей).
func TestTabsNoCyrillicInEnglish(t *testing.T) {
	h := latinHarness(t)
	h.fake.SetList(fixtureLatin[:len(fixtureLatin)-1] + `,
{"id":"dz","title":"Use Go","status":"open","priority":2,"issue_type":"decision","created_at":"2026-11-05T10:00:00Z","updated_at":"2026-11-05T10:00:00Z"}]`)
	h.fake.SetMemories(`{"k":"some *memory*"}`)
	h.fake.SetFormulas(fixtureFormulas)
	h.fake.SetTemplates(`[{"id":"tp","title":"Spec template","status":"open","issue_type":"epic","labels":["template"]}]`)
	h.start()
	for i := 0; i < 5; i++ {
		h.noCyrillic("вкладка " + h.m.Tab())
		h.keys("tab")
		h.noCyrillic("карточка вкладки " + h.m.Tab())
		h.keys("tab", "]")
	}
	h.fake.Fail("memories", "Error: locked", 1)
	h.keys("[", "[", "r")
	h.wantTab("memory")
	h.noCyrillic("ошибка памяти")
}

// Новые чтения — с --readonly первым аргументом, записей нет.
func TestTabsReadonlyDiscipline(t *testing.T) {
	h := specsHarness(t)
	h.fake.SetMemories(fixtureMemories)
	h.fake.SetFormulas(fixtureFormulas)
	h.fake.SetTemplates(fixtureTemplates)
	h.keys("]", "]", "]", "r", "]", "r", "]", "r")
	h.tick()
	seen := wantWriteDiscipline(t, h.fake.Calls())
	if seen["memories"] != 2 || seen["formula-list"] != 2 || seen["list-templates"] != 2 {
		t.Fatalf("вызовы: %v", seen)
	}
	for _, w := range writeSubs {
		if seen[w] != 0 {
			t.Fatalf("запись %s: %v", w, seen)
		}
	}
}
