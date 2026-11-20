package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/i18n"
)

// Группы fixtureMain по умолчанию: P0 bl; P1 ep, ep.1; P2 old (контекст), old.1; P3 lone.
var groupedMain = []string{"#P0", "bl", "#P1", "ep", "ep.1", "#P2", "old", "old.1", "#P3", "lone"}

func TestGroupHeadersShown(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.wantIDs(groupedMain...)
	// Курсор стартует на первой задаче, а не на заголовке.
	h.wantSelected("bl")
	h.wantView("▾ P0 · 1", "▾ P1 · 2", "▾ P2 · 2", "▾ P3 · 1", "Tasks (6)")
	h.wantNoView("no groups")
}

func TestToggleGroups(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("p")
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
	h.wantSelected("bl")
	h.wantView("no groups")
	h.wantNoView("P0 · 1")
	h.keys("p")
	h.wantIDs(groupedMain...)
	h.wantNoView("no groups")
	// В режиме ввода p — буква запроса.
	h.keys("/", "p")
	h.wantView("/p")
	h.wantNoView("no groups")

	ru := newHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	ru.keys("p")
	ru.wantView("без групп")
}

func TestToggleGroupsOffFromHeader(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("j", "j", "j", "j")
	h.wantSelected("#P2")
	// Без групп выбор переходит на первую задачу бывшей группы.
	h.keys("p")
	h.wantSelected("old")
}

func TestLeftOnRootSelectsGroup(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("h")
	h.wantSelected("#P0")
	h.wantIDs(groupedMain...)
	// ep раскрыт: первое h сворачивает его, второе ведёт к группе.
	h.keys("j", "j", "j", "h")
	h.wantSelected("ep")
	h.wantIDs("#P0", "bl", "#P1", "ep", "#P2", "old", "old.1", "#P3", "lone")
	h.keys("h")
	h.wantSelected("#P1")
	// Ребёнок по-прежнему ведёт к родителю.
	h.keys("j", "l", "j", "h")
	h.wantSelected("ep")
}

func TestCollapseGroup(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("#P1")
	h.keys("h")
	h.wantIDs("#P0", "bl", "#P1", "#P2", "old", "old.1", "#P3", "lone")
	h.wantView("▸ P1 · 2")
	h.keys("h")
	h.wantSelected("#P1")
	h.keys("l")
	h.wantIDs(groupedMain...)
	h.keys("enter")
	h.wantView("▸ P1 · 2")
	h.keys("space")
	h.wantView("▾ P1 · 2")
	h.keys("left")
	h.wantView("▸ P1 · 2")
	h.keys("right")
	h.wantIDs(groupedMain...)
}

func TestGroupCardSummary(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("j")
	h.wantSelected("#P1")
	h.wantCardLine0("2 tasks")
	h.wantCard("◐ 1 in progress · ○ 1 open")
	before := h.fake.Count("show")
	// Заголовок не дёргает bd, а y на нём ничего не делает (цифры с v1.4 переходят
	// к задачам группы — groupcard_test.go).
	h.keys("y")
	h.wantCount("show", before)
	h.wantSelected("#P1")
	if len(h.copied) != 0 {
		t.Fatalf("copied = %v", h.copied)
	}
	// Сводка следует за фильтрами.
	h.keys("c")
	h.wantCardLine0("3 tasks")
	h.wantCard("◐ 1 in progress · ○ 1 open · ✓ 1 closed")
	for _, l := range strings.Split(h.view(), "\n") {
		if w := ansi.StringWidth(l); w > 120 {
			t.Fatalf("ширина %d > 120: %q", w, l)
		}
	}

	ru := newHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	ru.keys("j")
	ru.wantCardLine0("2 задачи")
	ru.wantCard("◐ 1 в работе · ○ 1 открыто")
	ru.keys("k", "k")
	ru.wantSelected("#P0")
	ru.wantCardLine0("1 задача")
	if got := ru.cardTitle(); got != "P0" {
		t.Fatalf("заголовок рамки = %q", got)
	}
}

func TestDigitJumpOpensCollapsedGroup(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("h", "h")
	h.wantIDs("#P0", "#P1", "ep", "ep.1", "#P2", "old", "old.1", "#P3", "lone")
	h.keys("j", "j", "j")
	h.wantSelected("ep.1")
	// [2] — Blocked by bl в свёрнутой группе P0.
	h.keys("2")
	h.wantSelected("bl")
	h.wantIDs(groupedMain...)
}

func TestInitialIDOpensGroup(t *testing.T) {
	h := newHarness(t, fixtureMain, func(o *Options) { o.InitialID = "ep.2" })
	h.wantSelected("ep.2")
	h.wantIDs("#P0", "bl", "#P1", "ep", "ep.1", "ep.2", "#P2", "old", "old.1", "#P3", "lone")
	h.wantView("▾ P1 · 3")
}

// fixturePrefixed — у всех ID общий префикс acme-shop-.
const fixturePrefixed = `[
{"id":"acme-shop-fjpw","title":"Первая","status":"open","priority":1,"issue_type":"task"},
{"id":"acme-shop-fjpw.1","title":"Подзадача","status":"open","priority":1,"issue_type":"task","parent":"acme-shop-fjpw"},
{"id":"acme-shop-zz","title":"Закрытая","status":"closed","priority":2,"issue_type":"task"}
]`

func TestTreeShowsShortIDCardShowsFull(t *testing.T) {
	h := newHarness(t, fixturePrefixed)
	h.wantSelected("acme-shop-fjpw")
	h.wantIDs("#P1", "acme-shop-fjpw", "acme-shop-fjpw.1")
	tw := h.m.treeWidth()
	var tree []string
	for _, l := range strings.Split(h.view(), "\n") {
		tree = append(tree, ansi.Truncate(l, tw, ""))
	}
	left := strings.Join(tree, "\n")
	if !strings.Contains(left, "○ fjpw Первая") || !strings.Contains(left, "○ fjpw.1 Подзадача") {
		t.Fatalf("в дереве нет коротких ID:\n%s", left)
	}
	if strings.Contains(left, "acme-shop-") {
		t.Fatalf("в дереве полный ID:\n%s", left)
	}
	// Шапка карточки и сама карточка — с полным ID.
	h.wantCardOf("acme-shop-fjpw", "Первая")
	// Префикс считается по всем задачам, а не только по видимым; поиск идёт по полному ID.
	h.keys("/", "a", "c", "m", "e", "enter")
	h.wantIDs("#P1", "acme-shop-fjpw", "acme-shop-fjpw.1")
	// Не найдено — тоже полный ID.
	h2 := newHarness(t, fixturePrefixed, func(o *Options) { o.InitialID = "acme-shop-nope" })
	h2.wantView("task acme-shop-nope not found")
}

func TestCopyCopiesFullID(t *testing.T) {
	h := newHarness(t, fixturePrefixed)
	h.keys("y")
	if len(h.copied) != 1 || h.copied[0] != "acme-shop-fjpw" {
		t.Fatalf("copied = %v", h.copied)
	}
	h.wantView("copied ID: acme-shop-fjpw")
}

func TestGroupCollapseSurvivesRefresh(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("j", "h")
	h.wantSelected("#P1")
	h.fake.SetList(fixtureMainV2)
	h.keys("r")
	h.wantSelected("#P1")
	// V2: ep.1 закрыта, new (P1) добавлена — группа свёрнута, счётчик новый.
	h.wantIDs("#P0", "bl", "#P1", "#P2", "old", "old.1")
	h.wantView("▸ P1 · 2")
	// Раскрытые группы остаются раскрытыми.
	h.keys("l")
	h.wantIDs("#P0", "bl", "#P1", "ep", "new", "#P2", "old", "old.1")
}

func TestGroupVanishesSelectsNeighbour(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("G")
	h.wantSelected("lone")
	// V2: lone удалена, группа P3 пропала — выбор переходит на соседнюю задачу, а не заголовок.
	h.fake.SetList(fixtureMainV2)
	h.keys("r")
	sel := h.m.SelectedID()
	if strings.HasPrefix(sel, "#") || sel == "" {
		t.Fatalf("SelectedID = %q, want задачу", sel)
	}
}

func TestHelpMentionsGroups(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("?")
	h.wantView("groups")
	ru := newHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	ru.keys("?")
	ru.wantView("группы")
}

// Ruling V3: Esc после поиска оставляет найденную задачу выбранной, даже если без
// запроса она скрылась бы в свёрнутой группе или под свёрнутым родителем: они раскрываются.
func TestEscAfterSearchKeepsFoundTaskInCollapsedGroup(t *testing.T) {
	// Свёрнутая группа, Esc в режиме ввода.
	h := newHarness(t, fixtureMain)
	h.keys("j", "h")
	h.wantSelected("#P1")
	h.wantIDs("#P0", "bl", "#P1", "#P2", "old", "old.1", "#P3", "lone")
	h.keys("/", "з", "а", "п")
	h.wantSelected("ep.1")
	h.keys("esc")
	h.wantSelected("ep.1")
	h.wantIDs(groupedMain...)

	// Свёрнутая группа, Esc на закреплённом фильтре.
	h = newHarness(t, fixtureMain)
	h.keys("j", "h", "/", "з", "а", "п", "enter")
	h.wantSelected("ep.1")
	h.keys("esc")
	h.wantNoStatus("filter:")
	h.wantSelected("ep.1")
	h.wantIDs(groupedMain...)

	// Свёрнутый родитель.
	h = newHarness(t, fixtureMain)
	h.keys("j", "j", "h")
	h.wantSelected("ep")
	h.wantIDs("#P0", "bl", "#P1", "ep", "#P2", "old", "old.1", "#P3", "lone")
	h.keys("/", "з", "а", "п", "esc")
	h.wantSelected("ep.1")
	h.wantIDs(groupedMain...)

	// Без поиска свёрнутое не трогается: Esc на дереве ничего не раскрывает.
	h = newHarness(t, fixtureMain)
	h.keys("j", "h", "esc")
	h.wantSelected("#P1")
	h.wantIDs("#P0", "bl", "#P1", "#P2", "old", "old.1", "#P3", "lone")
}

// Поиск при группах: первым выбирается совпадение-задача, а не заголовок группы
// и не строка-контекст.
func TestSearchSkipsGroupHeaderGrouped(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("/", "х", "в", "о", "с", "т")
	// old — закрытый родитель-контекст, old.1 «Хвост старого эпика» — совпадение.
	h.wantIDs("#P2", "old", "old.1")
	h.wantSelected("old.1")
	h.keys("esc")
	h.wantSelected("old.1")
	h = newHarness(t, fixtureMain)
	h.keys("/", "б", "л")
	h.wantSelected("bl")
}

// Переключатель скрыл выбранную задачу: выбор уходит к соседу в той же группе.
func TestToggleHidesSelectedGrouped(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("c")
	h.wantIDs("#P0", "bl", "#P1", "ep", "ep.1", "ep.2", "#P2", "old", "old.1", "#P3", "lone")
	h.keys("j", "j", "j", "j")
	h.wantSelected("ep.2")
	h.keys("c")
	h.wantSelected("ep.1")
	h.wantCardOf("ep.1", "Карточка Записи")
}

// Обновление закрыло выбранную задачу без видимых соседей: выбор уходит к родителю.
func TestRefreshClosedSelectedMovesToParentGrouped(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("j", "j", "j")
	h.wantSelected("ep.1")
	h.fake.SetList(fixtureMainV2)
	h.keys("r")
	h.wantSelected("ep")
	h.wantIDs("#P0", "bl", "#P1", "ep", "new", "#P2", "old", "old.1")
}

// Обновление закрыло выбранную задачу, у которой есть видимый сосед: выбор уходит к нему.
func TestRefreshClosedSelectedMovesToSiblingGrouped(t *testing.T) {
	const v1 = `[
{"id":"a","title":"Эпик","status":"open","priority":1,"issue_type":"epic","created_at":"2026-11-01T10:00:00Z"},
{"id":"a.1","title":"Первая","status":"open","priority":2,"issue_type":"task","parent":"a","created_at":"2026-11-02T10:00:00Z"},
{"id":"a.2","title":"Вторая","status":"open","priority":2,"issue_type":"task","parent":"a","created_at":"2026-11-03T10:00:00Z"},
{"id":"b","title":"Другая группа","status":"open","priority":2,"issue_type":"task","created_at":"2026-11-04T10:00:00Z"}
]`
	h := newHarness(t, v1)
	h.wantIDs("#P1", "a", "a.1", "a.2", "#P2", "b")
	h.keys("j")
	h.wantSelected("a.1")
	h.fake.SetList(strings.Replace(v1, `"id":"a.1","title":"Первая","status":"open"`, `"id":"a.1","title":"Первая","status":"closed"`, 1))
	h.keys("r")
	h.wantSelected("a.2")
	h.wantIDs("#P1", "a", "a.2", "#P2", "b")
}

// При группах P<n> в строке дерева только там, где приоритет задачи не совпадает
// с приоритетом группы (подзадачи); без групп — у каждой строки, как в v1.
func TestTreePriorityOnlyWhenDiffers(t *testing.T) {
	h := newHarness(t, fixtureMain)
	tl := strings.Join(h.treeLines(), "\n")
	for _, want := range []string{"○ bl Блокер", "◐ ep Команды", "○ P2 ep.1 ⊘ Карточка", "✓ old Старый", "○ old.1 Хвост", "○ lone Контекст"} {
		if !strings.Contains(tl, want) {
			t.Errorf("в дереве нет %q:\n%s", want, tl)
		}
	}
	h.keys("p")
	tl = strings.Join(h.treeLines(), "\n")
	for _, want := range []string{"○ P0 bl Блокер", "◐ P1 ep Команды", "○ P2 ep.1 ⊘ Карточка", "✓ P2 old Старый", "○ P2 old.1 Хвост", "○ P3 lone Контекст"} {
		if !strings.Contains(tl, want) {
			t.Errorf("без групп в дереве нет %q:\n%s", want, tl)
		}
	}
}

// Запрос, стёртый Backspace до пустого, ведёт себя как Esc (ruling V3): выбранная
// задача остаётся выбранной и раскрывается, а не уступает первой строке.
func TestBackspaceToEmptyKeepsSelection(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("j", "h")
	h.wantSelected("#P1")
	h.keys("/", "з", "а", "п")
	h.wantSelected("ep.1")
	h.keys("backspace", "backspace")
	h.wantSelected("ep.1")
	h.keys("backspace")
	h.wantSelected("ep.1")
	h.wantIDs(groupedMain...)
	h.wantStatus("/")
	// Ввод продолжается: новый запрос снова ищет.
	h.keys("б", "л", "о", "к")
	h.wantSelected("bl")
	h.keys("enter")

	// Запрос без совпадений стёрт: возвращается выбор до поиска.
	h = newHarness(t, fixtureMain)
	h.keys("G")
	h.wantSelected("lone")
	h.keys("/", "q", "q")
	h.wantSelected("")
	h.keys("backspace", "backspace")
	h.wantSelected("lone")
}
