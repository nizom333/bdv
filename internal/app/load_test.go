package app

import (
	"testing"
	"time"
)

func TestLoadingScreen(t *testing.T) {
	h := prepareHarness(t, fixtureMain)
	h.holdList = true
	h.start()
	h.wantView("Loading tasks…")
	h.holdList = false
	h.release()
	h.wantNoView("Loading tasks…")
}

func TestInitialTree(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	// ep (in_progress) первым; old закрыт — последний и показан как контекст для old.1.
	h.wantIDs("ep", "ep.1", "bl", "lone", "old", "old.1")
}

func TestInitialSelectionAndDetail(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.wantSelected("ep")
	h.wantCardOf("ep", "Команды и таблица")
	h.wantCount("show", 1)
}

func TestFirstLoadError(t *testing.T) {
	h := prepareHarness(t, fixtureMain)
	h.fake.Fail("list", "Error: database \"x\" not found", 1)
	h.start()
	h.wantView("bd exited with an error", `Error: database "x" not found`, "r — retry · q — quit")
}

func TestRetryAfterError(t *testing.T) {
	h := prepareHarness(t, fixtureMain)
	h.fake.Fail("list", "Error: database \"x\" not found", 1)
	h.start()
	h.fake.Fail("list", "", 0)
	h.keys("r")
	if len(h.m.VisibleIDs()) == 0 {
		t.Fatalf("после r дерево пустое:\n%s", h.view())
	}
	h.wantNoView("bd exited with an error")
}

func TestInitialIDExpandsAncestors(t *testing.T) {
	const fx = `[
{"id":"p","title":"Родитель","status":"closed","priority":2,"issue_type":"epic"},
{"id":"p.1","title":"Ребёнок","status":"closed","priority":2,"issue_type":"task","parent":"p"}
]`
	h := newFlatHarness(t, fx, func(o *Options) { o.InitialID = "p.1" })
	h.wantIDs("p", "p.1")
	h.wantSelected("p.1")
}

func TestInitialIDClosedEnablesClosed(t *testing.T) {
	h := newHarness(t, fixtureMain, func(o *Options) { o.InitialID = "ep.2" })
	h.wantSelected("ep.2")
	h.wantStatus("closed")
}

func TestInitialIDEventEnablesEvents(t *testing.T) {
	h := newHarness(t, fixtureMain, func(o *Options) { o.InitialID = "ep.3" })
	h.wantSelected("ep.3")
	h.wantStatus("closed", "events")
}

func TestInitialIDNotFound(t *testing.T) {
	h := newFlatHarness(t, fixtureMain, func(o *Options) { o.InitialID = "zz" })
	h.wantSelected("ep")
	h.wantView("task zz not found")
}

func TestEmptyTreeMessage(t *testing.T) {
	h := newHarness(t, `[{"id":"x","title":"Закрытая","status":"closed","priority":2,"issue_type":"task"}]`)
	h.wantIDs()
	h.wantSelected("")
	h.wantView("No open tasks — press c to show closed")
	h.wantCount("show", 0)
}

func TestQuit(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("q")
	if !h.quit {
		t.Fatal("q не завершил программу")
	}
}

func TestQuitCtrlC(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("ctrl+c")
	if !h.quit {
		t.Fatal("ctrl+c не завершил программу")
	}
}

func TestStatusLine(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.wantView("updated 15:04:05", "? help", "Tasks (6)")
}

func TestInitSchedulesPoll(t *testing.T) {
	h := newHarness(t, fixtureMain)
	if len(h.timers) != 1 {
		t.Fatalf("timers = %v, want один pollMsg", h.timers)
	}
	if _, ok := h.timers[0].(pollMsg); !ok {
		t.Fatalf("timers[0] = %T, want pollMsg", h.timers[0])
	}
}

// Запасной стиль не "auto": он опрашивал бы терминал изнутри Update.
func TestDefaultMarkdownStyleIsDark(t *testing.T) {
	if got := New(Options{}).o.MarkdownStyle; got != "dark" {
		t.Fatalf("MarkdownStyle по умолчанию = %q, want dark", got)
	}
}

// Зависший bd list: первая загрузка обрывается по ListTimeout и показывает экран ошибки
// с фактическим таймаутом, а не «Загрузка задач…» навсегда.
func TestFirstLoadTimeout(t *testing.T) {
	h := prepareFlatHarness(t, fixtureMain, func(o *Options) { o.ListTimeout = 200 * time.Millisecond })
	h.fake.Sleep("list", 2)
	start := time.Now()
	h.start()
	if d := time.Since(start); d > time.Second {
		t.Errorf("первая загрузка ждала %v", d)
	}
	h.wantView("bd did not respond in 0.2 s", "r — retry · q — quit")
	h.wantNoView("Loading tasks…")
	// После таймаута r снова запускает загрузку. Предел — обычный: сам запуск фейка
	// занимает заметную долю от 200 мс.
	h.fake.Sleep("list", 0)
	h.m.o.ListTimeout = 30 * time.Second
	h.keys("r")
	h.wantSelected("ep")
}

// Таймаут фонового обновления идёт в статус-строку, данные остаются.
func TestRefreshTimeout(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	// Короткий предел — только после первой загрузки: запуск фейка сам по себе небыстрый.
	h.m.o.ListTimeout = 100 * time.Millisecond
	h.fake.Sleep("list", 2)
	h.keys("r")
	h.wantView("update failed: bd did not respond in 0.1 s")
	h.wantSelected("ep")
}

// Таймаут bd show — пометка в карточке.
func TestDetailTimeout(t *testing.T) {
	h := prepareFlatHarness(t, fixtureMain, func(o *Options) { o.DetailTimeout = 100 * time.Millisecond })
	h.fake.Sleep("show", 2)
	h.start()
	h.wantSelected("ep")
	h.wantView("failed to load links: bd did not respond in 0.1 s")
}

// Выход отменяет корневой контекст: идущие вызовы bd убиваются, а не остаются сиротами.
func TestQuitCancelsContext(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		h := newHarness(t, fixtureMain)
		h.keys(k)
		if !h.quit || h.m.ctx.Err() == nil {
			t.Fatalf("%s: quit=%v, ctx.Err()=%v", k, h.quit, h.m.ctx.Err())
		}
	}
}

// Выход с экрана загрузки тоже отменяет контекст.
func TestQuitFromLoadingCancelsContext(t *testing.T) {
	h := prepareHarness(t, fixtureMain)
	h.holdList = true
	h.start()
	h.keys("q")
	if !h.quit || h.m.ctx.Err() == nil {
		t.Fatalf("quit=%v, ctx.Err()=%v", h.quit, h.m.ctx.Err())
	}
}

// В проекте нет ни одной задачи: подсказка про c ни к чему.
func TestEmptyProjectMessage(t *testing.T) {
	h := newHarness(t, `[]`)
	h.wantIDs()
	h.wantView("No tasks")
	h.wantNoView("press c to show closed")
}

// Закрытые уже показаны, а дерево пусто (только скрытые события): подсказка про c неверна.
func TestEmptyTreeWithClosedShownMessage(t *testing.T) {
	h := newHarness(t, `[{"id":"x","title":"Событие","status":"closed","priority":2,"issue_type":"event"}]`)
	h.wantView("No open tasks — press c to show closed")
	h.keys("c")
	h.wantIDs()
	h.wantStatus("closed")
	h.wantView("No tasks")
	h.wantNoView("press c to show closed")
}
