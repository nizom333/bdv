package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/i18n"
	"github.com/nizom333/bdv/internal/testbd"
)

// noDBStderr — stderr bd list в папке без базы (bd 0.59).
const noDBStderr = "Error: no beads database found\n" +
	"Hint: run 'bd init' to create a database in the current directory\n" +
	"      or set BEADS_DIR to point to your .beads directory\n"

// initAllowed — main разрешил предложение bd init: --db, BEADS_DIR и BEADS_DB пусты.
func initAllowed(o *Options) {
	o.InitAllowed = true
	o.InitDir = "/w/proj"
}

// prepareNoDB — модель в папке без базы: bd list падает с «no beads database found».
// Init не запущен: тест может ещё настроить фейк.
func prepareNoDB(t *testing.T, opts ...func(*Options)) *harness {
	t.Helper()
	h := prepareHarness(t, `[]`, append([]func(*Options){initAllowed}, opts...)...)
	h.fake.Fail("list", noDBStderr, 1)
	return h
}

func newNoDB(t *testing.T, opts ...func(*Options)) *harness {
	t.Helper()
	return prepareNoDB(t, opts...).start()
}

// wantNoDBScreen — экран без базы: заголовок, stderr bd и подсказка с i.
func (h *harness) wantNoDBScreen() {
	h.t.Helper()
	h.wantView("No beads database in this project.", "i — initialize (bd init) · r — retry · q — quit")
	h.wantNoView("bd exited with an error", "Initialize beads in", "initializing…")
}

// Первая загрузка упала с «no beads database found»: экран без базы вместо экрана
// ошибки, stderr bd виден под заголовком. Сравнение без учёта регистра.
func TestNoDBScreenFromListError(t *testing.T) {
	h := newNoDB(t)
	h.wantNoDBScreen()
	h.wantView("Error: no beads database found", "Hint: run 'bd init'", "or set BEADS_DIR")
	h.wantCount("init", 0)

	h = prepareNoDB(t)
	h.fake.Fail("list", "ERROR: No Beads Database Found\n", 1)
	h.start()
	h.wantNoDBScreen()
	h.wantView("ERROR: No Beads Database Found")
}

// --db, BEADS_DIR или BEADS_DB заданы (InitAllowed выключен): обычный экран ошибки,
// клавиша i ничего не делает.
func TestNoDBNotOfferedWhenInitNotAllowed(t *testing.T) {
	h := newNoDB(t, func(o *Options) { o.InitAllowed = false })
	h.wantView("bd exited with an error", "Error: no beads database found", "r — retry · q — quit")
	h.wantNoView("No beads database in this project.", "i — initialize")
	h.keys("i")
	h.wantNoView("Initialize beads in")
	h.keys("i", "y")
	h.keys("iy")
	h.wantCount("init", 0)
	h.wantView("bd exited with an error")
}

// Другая ошибка первой загрузки: обычный экран, без i.
func TestOtherErrorNoInitKey(t *testing.T) {
	h := prepareHarness(t, `[]`, initAllowed)
	h.fake.Fail("list", `Error: failed to open database: database "x" not found`, 1)
	h.start()
	h.wantView("bd exited with an error", "r — retry · q — quit")
	h.wantNoView("No beads database in this project.", "i — initialize")
	h.keys("i", "y", "iy")
	h.wantNoView("Initialize beads in")
	h.wantCount("init", 0)
	// Таймаут первой загрузки — тоже не повод предлагать bd init.
	h = prepareHarness(t, `[]`, initAllowed, func(o *Options) { o.ListTimeout = 200 * time.Millisecond })
	h.fake.Sleep("list", 2)
	h.start()
	h.wantView("bd did not respond in 0.2 s", "r — retry · q — quit")
	h.wantNoView("i — initialize")
}

// i открывает подтверждение с путём и префиксом; n и любая другая клавиша (кроме y, c,
// q и Ctrl+C) отменяют его без запуска bd init. c переключает режим ID (initids_test.go).
func TestInitConfirmCancel(t *testing.T) {
	h := newNoDB(t)
	for _, k := range []string{"n", "x", "esc", "enter", "i", "space", "Y", "r", "N"} {
		h.keys("i")
		h.wantView("Initialize beads in /w/proj?", "prefix: proj", "IDs: hash (bd default)",
			"y — run bd init · c — switch IDs · n — cancel")
		h.wantNoView("i — initialize")
		h.keys(k)
		if h.quit {
			t.Fatalf("%q в подтверждении завершил программу", k)
		}
		h.wantNoDBScreen()
	}
	h.wantCount("init", 0)
	// Отмена не запускает и bd list: r в подтверждении — тоже отмена.
	h.wantCount("list", 1)
	// q в подтверждении — выход.
	h.keys("i", "q")
	if !h.quit || h.m.ctx.Err() == nil {
		t.Fatalf("q в подтверждении: quit=%v ctx=%v", h.quit, h.m.ctx.Err())
	}
	h.wantCount("init", 0)
}

// Ctrl+C в подтверждении — выход.
func TestInitConfirmCtrlCQuits(t *testing.T) {
	h := newNoDB(t)
	h.keys("i", "ctrl+c")
	if !h.quit {
		t.Fatal("Ctrl+C в подтверждении не завершил программу")
	}
	h.wantCount("init", 0)
}

// bd init запускается только клавишей y в подтверждении: y на экране без базы ничего
// не делает, склеенные руны идут по одной (n отменяет, y после неё — уже не в подтверждении).
func TestInitRunsOnlyOnY(t *testing.T) {
	h := newNoDB(t)
	h.keys("y", "Y")
	h.wantNoDBScreen()
	h.wantCount("init", 0)
	h.keys("i", "ny")
	h.wantNoDBScreen()
	h.wantCount("init", 0)
	// Вставка — не нажатия: ни подтверждения, ни запуска.
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("iy"), Paste: true})
	h.wantNoDBScreen()
	h.wantCount("init", 0)
	h.holdInit = true
	h.keys("i", "y")
	h.wantCount("init", 1)
	h.wantView("initializing…")
	if c := h.fake.Calls(); !slices.Equal(c[len(c)-1], []string{"init"}) {
		t.Fatalf("вызов bd init = %q, want [init]", c[len(c)-1])
	}
}

// Успех: «initializing…», во время работы клавиши (кроме Ctrl+C) ничего не делают;
// затем последние строки вывода (не больше 8, очищенные) и сразу bd list; новый
// проект открывается с пустым деревом «No tasks», имя проекта — папка init.
func TestInitSuccessLoadsTree(t *testing.T) {
	h := newNoDB(t)
	var out strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&out, "line%02d\n", i)
	}
	out.WriteString("\x1b[32m✓\x1b[0m bd initialized\x07\n")
	h.fake.SetInitOutput(out.String(), "")
	h.holdInit = true
	h.keys("i", "y")
	h.wantView("initializing…")
	h.wantNoView("No beads database in this project.", "Initialize beads in")
	h.keys("q", "r", "i", "y", "n", "iy")
	if h.quit {
		t.Fatal("q во время bd init завершил программу")
	}
	h.wantCount("init", 1)
	h.wantCount("list", 1)
	h.wantView("initializing…")

	// bd init создал базу: list теперь отвечает пустым списком.
	h.fake.Fail("list", "", 0)
	h.holdInit, h.holdList = false, true
	h.release()
	h.wantCount("list", 2)
	h.wantView("line06", "line12", "✓ bd initialized", "Loading tasks…")
	h.wantNoView("line05", "\x1b", "\x07", "initializing…", "No beads database")

	h.holdList = false
	h.release()
	h.wantView("No tasks", "Tasks (0)", "proj")
	h.wantNoView("press c to show closed", "line12", "No beads database", "Loading tasks…")
	h.wantCount("init", 1)
}

// Ошибка bd init: экран без базы с очищенным выводом и кодом выхода; i повторяет.
func TestInitFailureShowsOutputAndRetry(t *testing.T) {
	h := newNoDB(t)
	h.fake.Fail("init", "Error: cannot reach dolt server\x1b[31m at 127.0.0.1:3307\nhint: start dolt sql-server", 1)
	h.keys("i", "y")
	h.wantNoDBScreen()
	h.wantView("bd init failed: exit status 1", "Error: cannot reach dolt server at 127.0.0.1:3307",
		"hint: start dolt sql-server")
	h.wantNoView("\x1b")
	h.wantCount("init", 1)
	h.wantCount("list", 1)

	h.keys("i")
	h.wantView("Initialize beads in /w/proj?")
	h.fake.Fail("init", "", 0)
	h.fake.Fail("list", "", 0)
	h.keys("y")
	h.wantCount("init", 2)
	h.wantView("No tasks")
	h.wantNoView("bd init failed")
}

// Таймаут bd init — текст таймаута из каталога.
func TestInitTimeout(t *testing.T) {
	h := newNoDB(t, func(o *Options) { o.InitTimeout = 300 * time.Millisecond })
	h.fake.Sleep("init", 5)
	start := time.Now()
	h.keys("i", "y")
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("bd init ждал %v", d)
	}
	h.wantNoDBScreen()
	h.wantView("bd init failed: bd did not respond in 0.3 s")
	if d := New(Options{}).o.InitTimeout; d != 120*time.Second {
		t.Errorf("InitTimeout по умолчанию = %v, want 120s", d)
	}
}

// Ctrl+C во время bd init: выход, корневой контекст отменён, bd убит.
func TestInitCtrlCCancels(t *testing.T) {
	h := newNoDB(t)
	h.fake.Sleep("init", 5)
	h.keys("i")
	// Команду bd init не выполняем синхронно: она идёт в горутине, как в Bubble Tea.
	m, cmd := h.m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	h.m = m.(Model)
	if cmd == nil {
		t.Fatal("y не запустил bd init")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	deadline := time.Now().Add(5 * time.Second)
	for h.fake.Count("init") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("фейк не получил bd init")
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.wantView("initializing…")
	start := time.Now()
	h.keys("ctrl+c")
	if !h.quit || h.m.ctx.Err() == nil {
		t.Fatalf("Ctrl+C: quit=%v ctx=%v", h.quit, h.m.ctx.Err())
	}
	select {
	case msg := <-done:
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("bd init после отмены жил %v", d)
		}
		if im, ok := msg.(initMsg); !ok || im.err == nil {
			t.Errorf("msg = %#v, want initMsg с ошибкой отмены", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("bd init не прервался после Ctrl+C")
	}
}

// main не нашёл .beads: модель стартует сразу на экране без базы, bd list не вызывается
// ни при старте, ни по таймеру опроса; r запускает его вручную.
func TestStartNoDBSkipsList(t *testing.T) {
	h := prepareNoDB(t, func(o *Options) { o.StartNoDB = true })
	h.start()
	h.wantCount("list", 0)
	h.wantNoDBScreen()
	h.wantNoView("Loading tasks…")
	h.tick()
	h.tick()
	h.wantCount("list", 0)
	h.keys("r")
	h.wantCount("list", 1)
	h.wantNoDBScreen()
	h.wantView("Error: no beads database found")
	// И отсюда i → y создаёт базу и открывает дерево.
	h.fake.Fail("list", "", 0)
	h.keys("i", "y")
	h.wantCount("init", 1)
	h.wantView("No tasks")
}

// Экраны без базы, подтверждения, запуска, ошибки и успеха помещаются в 80x24 и в узкие
// окна на обоих языках; меньше 20x6 — одна строка TooSmall.
func TestInitScreensFit80x24AndRU(t *testing.T) {
	long := strings.Repeat("very long stderr line that wraps around the screen ", 4)
	var lots strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&lots, "init output line %02d %s\n", i, long)
	}
	for _, c := range []struct {
		lang                              i18n.Lang
		noDB, hint, confirm, prefix, keys string
		hash, seq                         string
		running, failed, loading, empty   string
	}{
		{i18n.EN, "No beads database in this project.", "i — initialize (bd init) · r — retry · q — quit",
			"Initialize beads in", "prefix: " + strings.Repeat("p", 10), "y — run bd init · c — switch IDs · n — cancel",
			"IDs: hash (bd default)", "IDs: sequential 1, 2, 3",
			"initializing…", "bd init failed: exit status 1", "Loading tasks…", "No tasks"},
		{i18n.RU, "В этом проекте нет базы beads.", "i — инициализировать (bd init) · r — повторить · q — выход",
			"Инициализировать beads в", "префикс: " + strings.Repeat("p", 10), "y — запустить bd init · c — сменить ID · n — отмена",
			"ID: хеш (по умолчанию bd)", "ID: последовательные 1, 2, 3",
			"инициализация…", "bd init не удался: код выхода 1", "Загрузка задач…", "Нет задач"},
	} {
		for _, sz := range [][2]int{{80, 24}, {60, 15}, {40, 12}, {20, 6}} {
			w, ht := sz[0], sz[1]
			what := fmt.Sprintf("%v %dx%d", c.lang, w, ht)
			h := prepareNoDB(t, func(o *Options) {
				o.Lang = c.lang
				o.InitDir = "/very/long/path/to/some/deeply/nested/project/" + strings.Repeat("p", 10)
			})
			h.fake.Fail("list", noDBStderr+long+"\n"+long, 1)
			h.start()
			h.send(tea.WindowSizeMsg{Width: w, Height: ht})
			h.wantFits(w, ht, what+" без базы")
			if w >= 60 {
				h.wantView(c.noDB, c.hint)
			}

			// y в окне, где путь целиком не виден, не работает (TestInitConfirmTinyWindowBlocksY):
			// там подтверждение нажимается в 80x24 и окно возвращается.
			pressY := func() {
				h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
				h.keys("y")
				h.send(tea.WindowSizeMsg{Width: w, Height: ht})
			}
			h.keys("i")
			h.wantFits(w, ht, what+" подтверждение")
			if w >= 40 {
				h.wantView(c.confirm, c.prefix, c.hash)
			}
			if w >= 60 {
				// В 40 колонок подсказка переносится между пунктами (TestPackKeys).
				h.wantView(c.keys)
			}
			// Последовательные ID и обратно: строка режима тоже входит в окно.
			h.keys("c")
			h.wantFits(w, ht, what+" подтверждение, последовательные ID")
			if w >= 40 {
				h.wantView(c.confirm, c.seq)
			}
			h.keys("c")

			h.fake.Fail("init", lots.String(), 1)
			pressY()
			h.wantFits(w, ht, what+" ошибка")
			if w >= 40 {
				h.wantView(c.failed)
			}
			if w >= 60 {
				h.wantView(c.noDB, c.hint)
			}

			h.fake.Fail("init", "", 0)
			h.fake.SetInitOutput(lots.String(), "")
			h.holdInit = true
			h.keys("i")
			pressY()
			h.wantFits(w, ht, what+" запуск")
			h.wantView(c.running)

			h.fake.Fail("list", "", 0)
			h.holdInit, h.holdList = false, true
			h.release()
			h.wantFits(w, ht, what+" успех")
			h.wantView(c.loading, "line 40")

			h.holdList = false
			h.release()
			h.wantFits(w, ht, what+" дерево")
			h.wantView(c.empty)
		}
		// Окно меньше 20x6 — одна строка TooSmall, и в подтверждении тоже.
		h := newNoDB(t, func(o *Options) { o.Lang = c.lang })
		h.keys("i")
		h.send(tea.WindowSizeMsg{Width: 19, Height: 5})
		if v := h.view(); v == "" || strings.Contains(v, "\n") || !strings.HasPrefix(i18n.For(c.lang).TooSmall, v) {
			t.Fatalf("%v крошечное окно: %q", c.lang, v)
		}
	}
}

// Во всём сценарии bd init все прочие вызовы bd идут с --readonly первым аргументом;
// записи (wantWriteDiscipline): bd init — ровно [init], bd config set — ровно [config set
// issue_id_mode counter] (последовательные ID), comments add — без --readonly.
func TestInitFlowOtherCallsReadonly(t *testing.T) {
	h := newNoDB(t)
	h.keys("r", "i", "n", "i", "c")
	h.fake.Fail("init", "boom", 1)
	h.keys("y", "r", "i")
	h.fake.Fail("init", "", 0)
	h.fake.Fail("list", "", 0)
	h.fake.SetList(fixtureMain)
	h.keys("y")
	h.wantSelected("bl")
	h.keys("m")
	h.typeText("после init")
	h.keys("ctrl+s", "r")
	h.tick()
	seen := wantWriteDiscipline(t, h.fake.Calls())
	for _, c := range h.fake.Calls() {
		switch testbd.Subcommand(c) {
		case "init":
			if !slices.Equal(c, []string{"init"}) {
				t.Errorf("bd init = %q, want [init]", c)
			}
		case "config-set":
			if !slices.Equal(c, []string{"config", "set", "issue_id_mode", "counter"}) {
				t.Errorf("bd config = %q, want [config set issue_id_mode counter]", c)
			}
		}
	}
	if seen["init"] != 2 || seen["config-set"] != 1 || seen["comments-add"] != 1 || seen["list"] < 4 || seen["show"] == 0 {
		t.Fatalf("вызовы: %v", seen)
	}
}

// Подсказка клавиш переносится между пунктами « · », а не посреди пункта.
func TestPackKeys(t *testing.T) {
	const s = "i — initialize (bd init) · r — retry · q — quit"
	if got := packKeys(s, 80); !slices.Equal(got, []string{s}) {
		t.Errorf("80: %q", got)
	}
	if got, want := packKeys(s, 40), []string{"i — initialize (bd init) · r — retry", "q — quit"}; !slices.Equal(got, want) {
		t.Errorf("40: %q, want %q", got, want)
	}
	for _, r := range packKeys(s, 10) {
		if w := ansi.StringWidth(r); w > 10 {
			t.Errorf("10: строка %q шириной %d", r, w)
		}
	}
}

// Имя проекта после bd init — папка InitDir, даже если main уже определил другое:
// ~/.beads или .beads выше дали бы чужое имя (например, имя пользователя).
func TestInitProjectNameIsInitDir(t *testing.T) {
	h := newNoDB(t, func(o *Options) { o.Project = "alice" })
	h.wantView("No beads database")
	h.fake.Fail("list", "", 0)
	h.keys("i", "y")
	h.wantView("No tasks")
	if top := h.topBar(); !strings.HasPrefix(top, "proj ") || strings.Contains(top, "alice") {
		t.Fatalf("верхняя строка %q, want имя proj", top)
	}
}

// ? на экране без базы открывает справку с разделом про i; любая клавиша закрывает её
// и возвращает на экран без базы, ничего больше не делая.
func TestHelpFromNoDBScreen(t *testing.T) {
	for _, k := range []string{"x", "i", "y", "q", "r"} {
		h := newNoDB(t)
		h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
		h.keys("?")
		h.wantFits(80, 24, "справка без базы")
		h.wantView("Help · any key closes", "No database", "bd init, then y")
		h.keys(k)
		if h.quit {
			t.Fatalf("%q, закрывший справку, завершил программу", k)
		}
		h.wantNoDBScreen()
		h.wantNoView("Help ·")
		h.wantCount("init", 0)
		h.wantCount("list", 1)
	}
	// В подтверждении ? — как любая клавиша: отмена, без справки.
	h := newNoDB(t)
	h.keys("i", "?")
	h.wantNoDBScreen()
	h.wantNoView("Help ·")
}

// Окно, в котором путь подтверждения целиком не помещается: вместо вопроса TooSmall,
// y не запускает bd init. В окне побольше вопрос виден, и y работает.
func TestInitConfirmTinyWindowBlocksY(t *testing.T) {
	h := newNoDB(t, func(o *Options) {
		o.InitDir = "/very/long/path/to/some/deeply/nested/project/" + strings.Repeat("p", 10)
	})
	h.send(tea.WindowSizeMsg{Width: 20, Height: 6})
	h.keys("i")
	h.wantView("window too small")
	h.wantNoView("Initialize beads in")
	h.wantFits(20, 6, "подтверждение 20x6")
	h.keys("y")
	h.wantCount("init", 0)
	if h.m.initRunning {
		t.Fatal("y запустил bd init в окне, где путь не виден")
	}
	// Подтверждение осталось открытым: в большом окне вопрос виден целиком.
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.wantView("Initialize beads in /very/long/path/to/some/deeply/nested/project/pppppppppp?")
	h.fake.Fail("list", "", 0)
	h.keys("y")
	h.wantCount("init", 1)
	h.wantView("No tasks")
}
