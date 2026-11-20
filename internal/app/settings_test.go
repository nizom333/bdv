package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/nizom333/bdv/internal/i18n"
	"github.com/nizom333/bdv/internal/settings"
	"github.com/nizom333/bdv/internal/testbd"
)

// fixtureSettings — задачи со своим статусом awaiting_review и своим типом spike: для
// предупреждения при удалении элемента списка.
const fixtureSettings = `[
{"id":"a","title":"Review me","status":"awaiting_review","priority":1,"issue_type":"task",
 "created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-10T10:00:00Z"},
{"id":"b","title":"Spike one","status":"open","priority":2,"issue_type":"spike",
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-10T10:00:00Z"},
{"id":"c","title":"Spike two","status":"closed","priority":2,"issue_type":"spike",
 "created_at":"2026-11-03T10:00:00Z","updated_at":"2026-11-10T10:00:00Z","closed_at":"2026-11-10T10:00:00Z"}
]`

// configListReal — ответ bd config list --json по образцу настоящего bd 0.59 плюс запрещённые ключи.
const configListReal = `{
  "auto_compact_enabled": "false",
  "compact_batch_size": "50",
  "compact_tier1_days": "30",
  "compact_tier2_days": "90",
  "compaction_enabled": "false",
  "issue_prefix": "demo-app",
  "schema_version": "6",
  "jira.api_token": "s3cr3t-token",
  "sync.remote": "dolthub://secret/remote"
}`

// settingsHarness — дерево загружено, фейк отвечает на чтение настроек как настоящий bd 0.59.
func settingsHarness(t *testing.T, fixture string, opts ...func(*Options)) *harness {
	t.Helper()
	h := newHarness(t, fixture, append([]func(*Options){func(o *Options) { o.Project = "demo-app" }}, opts...)...)
	h.fake.SetConfigList(configListReal)
	// YAML-ключи bd 0.59 отдаёт с location config.yaml даже незаданными; export.auto —
	// без location (в 0.59 не YAML-ключ), как у настоящего bd.
	for _, e := range settings.Table {
		if e.Store == settings.YAML && e.Key != "export.auto" {
			h.fake.SetConfigGet(e.Key, `{"key":"`+e.Key+`","location":"config.yaml","value":""}`)
		}
	}
	h.fake.SetConfigGet("create.require-description", `{"key":"create.require-description","location":"config.yaml","value":"false"}`)
	h.fake.SetConfigGet("dolt.auto-commit", `{"key":"dolt.auto-commit","location":"config.yaml","value":"on"}`)
	h.fake.SetConfigGet("hierarchy.max-depth", `{"key":"hierarchy.max-depth","location":"config.yaml","value":"3"}`)
	return h
}

// openSettings — «,» и ответ чтения.
func (h *harness) openSettings() {
	h.t.Helper()
	h.keys(",")
	if !h.m.set.open || h.m.set.loading {
		h.t.Fatalf("настройки не открылись:\n%s", h.view())
	}
}

// settingKey — ключ выбранной строки настроек.
func (h *harness) settingKey() string {
	if h.m.set.cur < 0 || h.m.set.cur >= len(h.m.set.rows) {
		return ""
	}
	return h.m.set.rows[h.m.set.cur].Key
}

// gotoSetting ставит курсор настроек на ключ key.
func (h *harness) gotoSetting(key string) {
	h.t.Helper()
	h.keys("g")
	for i := 0; i < 40 && h.settingKey() != key; i++ {
		h.keys("j")
	}
	if h.settingKey() != key {
		h.t.Fatalf("нет строки %s:\n%s", key, h.view())
	}
}

// callsOf — вызовы bd с подкомандой sub по порядку.
func (h *harness) callsOf(sub string) [][]string {
	var out [][]string
	for _, c := range h.fake.Calls() {
		if testbd.Subcommand(c) == sub {
			out = append(out, c)
		}
	}
	return out
}

// wantWrites — записи bd ровно want (подкоманда и аргументы без --db).
func (h *harness) wantWrites(sub string, want ...[]string) {
	h.t.Helper()
	got := h.callsOf(sub)
	if len(got) != len(want) {
		h.t.Fatalf("%s: вызовов %d, want %d: %q", sub, len(got), len(want), got)
	}
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			h.t.Fatalf("%s[%d] = %q, want %q", sub, i, got[i], want[i])
		}
	}
}

// writeSubs — подкоманды, которые пишут: только они идут без --readonly.
var writeSubs = []string{"comments-add", "init", "config-set", "config-unset"}

// wantWriteDiscipline — записи без --readonly, все прочие вызовы — с --readonly первым.
func wantWriteDiscipline(t *testing.T, calls [][]string) map[string]int {
	t.Helper()
	seen := map[string]int{}
	for _, c := range calls {
		sub := testbd.Subcommand(c)
		seen[sub]++
		if slices.Contains(writeSubs, sub) {
			if slices.Contains(c, "--readonly") {
				t.Errorf("запись %s с --readonly: %q", sub, c)
			}
			continue
		}
		if len(c) == 0 || c[0] != "--readonly" {
			t.Errorf("вызов без --readonly: %q", c)
		}
	}
	return seen
}

// Тексты каждой настройки таблицы и каждого предупреждения есть в обоих каталогах.
func TestSettingsCatalogCoversTable(t *testing.T) {
	for _, l := range []i18n.Lang{i18n.EN, i18n.RU} {
		c := i18n.For(l)
		for _, e := range settings.Table {
			if c.SettingLabels[e.Key] == "" || c.SettingDescs[e.Key] == "" {
				t.Errorf("%v: нет названия или описания %s", l, e.Key)
			}
			if e.Warn != "" && c.SettingWarns[e.Warn] == "" {
				t.Errorf("%v: нет предупреждения %s", l, e.Warn)
			}
		}
		if len(c.SettingsSections) != int(settings.Storage)+1 {
			t.Errorf("%v: секций %d", l, len(c.SettingsSections))
		}
	}
}

// «,» открывает экран: сначала «loading settings…», затем строки; Esc — назад к дереву.
// Чтение: config list, config get каждого YAML-ключа, types — все с --readonly.
func TestSettingsOpenLoadEsc(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.holdSet = true
	h.keys(",")
	h.wantView("Settings · demo-app", "loading settings…")
	h.holdSet = false
	h.release()
	h.wantView("IDs", "Statuses & types", "Creating issues", "Storage (advanced)",
		"ID prefix", "demo-app", "ID mode", "hash (default)", "Require description", "false (default)",
		"j k move · Enter change · u reset · Esc back")
	// Под выбранной строкой — описание и ключ bd.
	h.wantView("change with bd rename-prefix", "issue_prefix")
	h.keys("j")
	h.wantView("hash: random IDs", "issue_id_mode")
	if n := len(h.callsOf("config-list")); n != 1 {
		t.Fatalf("config list: %d", n)
	}
	if n := len(h.callsOf("config-get")); n != len(settings.GetKeys()) {
		t.Fatalf("config get: %d, want %d", n, len(settings.GetKeys()))
	}
	if n := len(h.callsOf("types")); n != 1 {
		t.Fatalf("types: %d", n)
	}
	wantWriteDiscipline(t, h.fake.Calls())
	h.keys("esc")
	if h.m.set.open {
		t.Fatal("Esc не закрыл настройки")
	}
	h.wantView("Tasks (")
}

// Запрещённые и неизвестные ключи не показываются никогда.
func TestSettingsForbiddenNeverShown(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	for i := 0; i < 25; i++ {
		h.wantNoView("schema_version", "s3cr3t", "api_token", "sync.remote", "dolthub", "compact_batch_size")
		h.keys("j")
	}
}

// Ключ, чей config get упал, скрыт; остальные видны.
func TestSettingsGetErrorHidesKey(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.fake.FailID("config-get", "backup.enabled", "Error: unknown key", 1)
	h.openSettings()
	h.wantNoView("Backups")
	h.wantView("Dolt auto-commit", "Dolt auto-push")
}

// Ошибка config list — экран ошибки чтения; r повторяет, Esc — к дереву.
func TestSettingsLoadError(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.fake.Fail("config-list", "Error: dolt locked\nmore", 1)
	h.keys(",")
	h.wantView("⚠ settings not loaded: Error: dolt locked", "r — retry · Esc — back")
	h.fake.Fail("config-list", "", 0)
	h.keys("r")
	h.wantView("ID prefix", "demo-app")
	h.keys("esc")
	h.wantView("Tasks (")
}

// В режиме counter длины хеша и вероятность коллизий скрыты.
func TestSettingsCounterHidesHashKeys(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.wantView("Min hash length", "Max hash length", "Collision probability")
	h.fake.SetConfigList(`{"issue_prefix":"p","issue_id_mode":"counter"}`)
	h.keys("esc")
	h.openSettings()
	h.wantView("ID mode", "counter")
	h.wantNoView("Min hash length", "Max hash length", "Collision probability")
}

// «,» не работает в поиске, справке, поле комментария и до загрузки дерева.
func TestSettingsKeyBlockedInOtherModes(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.keys("/", ",")
	if h.m.set.open || h.m.opts.Query != "," {
		t.Fatalf("поиск: open=%v query=%q", h.m.set.open, h.m.opts.Query)
	}
	h.keys("esc", "?", ",")
	if h.m.set.open {
		t.Fatal("из справки")
	}
	h.keys("m", ",")
	if !h.m.commenting || h.m.set.open || h.m.ta.Value() != "," {
		t.Fatalf("комментарий: open=%v text=%q", h.m.set.open, h.m.ta.Value())
	}
	h.keys("esc")
	h.wantCount("config-list", 0)

	// До загрузки дерева и на экране без базы.
	h = prepareHarness(t, fixtureMain)
	h.holdList = true
	h.start()
	h.keys(",")
	if h.m.set.open {
		t.Fatal("до загрузки")
	}
	n := newNoDB(t)
	n.keys(",")
	if n.m.set.open {
		t.Fatal("без базы")
	}
	n.wantCount("config-list", 0)
}

// Пока открыты настройки, клавиши дерева не действуют: q не выходит, c/p/R/e не
// переключают, / не открывает поиск, m не открывает комментарий.
func TestSettingsTreeKeysInert(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	before := h.m.opts
	h.keys("q", "c", "p", "R", "e", "/", "m", "?", "S", "tab", "y")
	if h.quit || h.m.opts != before || h.m.typing || h.m.commenting || h.m.help || h.m.mouseOff || len(h.copied) != 0 {
		t.Fatalf("клавиши дерева сработали: quit=%v opts=%+v typing=%v", h.quit, h.m.opts, h.m.typing)
	}
	if !h.m.set.open {
		t.Fatal("настройки закрылись")
	}
	// Склеенные руны тоже не команды дерева.
	h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jjq")})
	if h.quit || h.settingKey() != "min_hash_length" {
		t.Fatalf("склеенные руны: quit=%v key=%s", h.quit, h.settingKey())
	}
	h.keys("ctrl+c")
	if !h.quit {
		t.Fatal("Ctrl+C не выходит")
	}
}

// issue_prefix только для чтения: Enter и u ничего не делают.
func TestSettingsReadonlyPrefix(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("issue_prefix")
	h.keys("enter", "space", "u")
	if h.m.set.mode != setMain {
		t.Fatalf("режим %v", h.m.set.mode)
	}
	h.wantView("change with bd rename-prefix")
	h.wantCount("config-set", 0)
	h.wantCount("config-unset", 0)
}

// bool: Enter даёт подтверждение «ключ: старое → новое»; y применяет и перечитывает.
func TestSettingsBoolToggleApply(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("create.require-description")
	h.keys("enter")
	h.wantView("create.require-description: false (default) → true", "y — apply · any other key — cancel")
	lists := h.fake.Count("list")
	h.keys("y")
	h.wantWrites("config-set", []string{"config", "set", "create.require-description", "true"})
	if n := len(h.callsOf("config-list")); n != 2 {
		t.Fatalf("после записи не перечитано: config list %d", n)
	}
	// Ключ не из тех, что меняют дерево: список не перечитывается.
	h.wantCount("list", lists)
	if h.m.set.mode != setMain || h.m.set.saving {
		t.Fatalf("после записи: mode=%v saving=%v", h.m.set.mode, h.m.set.saving)
	}
	// Space тоже переключает.
	h.keys("space")
	h.wantView("create.require-description: false (default) → true")
}

// Подтверждение применяется только по y: n, Esc, Y, Enter отменяют.
func TestSettingsConfirmOnlyY(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("backup.enabled")
	for _, k := range []string{"n", "esc", "Y", "enter", "q", "u"} {
		h.keys("enter")
		h.wantView("backup.enabled: false (default) → true")
		h.keys(k)
		if h.m.set.mode != setMain {
			t.Fatalf("%s не отменил: mode %v", k, h.m.set.mode)
		}
		if h.quit {
			t.Fatal("q в подтверждении вышел")
		}
	}
	h.wantCount("config-set", 0)
}

// Подтверждение, которое не помещается целиком, показывается как TooSmall, и y не работает.
func TestSettingsConfirmTruncatedBlocksY(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("issue_id_mode")
	h.keys("enter")
	h.wantView("Affects new tasks only")
	h.send(tea.WindowSizeMsg{Width: 24, Height: 7})
	h.wantFits(24, 7, "крошечное подтверждение")
	h.wantView("window too")
	h.keys("y")
	h.wantCount("config-set", 0)
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.keys("y")
	h.wantWrites("config-set", []string{"config", "set", "issue_id_mode", "counter"})
}

// enum с предупреждением: issue_id_mode → counter; после записи перечитываются и
// настройки, и список задач.
func TestSettingsEnumWarnAndListReload(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("issue_id_mode")
	h.keys("enter")
	h.wantView("issue_id_mode: hash (default) → counter", "⚠ Affects new tasks only; existing IDs do not change.")
	lists := h.fake.Count("list")
	h.fake.SetConfigList(`{"issue_prefix":"demo-app","issue_id_mode":"counter"}`)
	h.keys("y")
	h.wantWrites("config-set", []string{"config", "set", "issue_id_mode", "counter"})
	h.wantCount("list", lists+1)
	h.wantView("counter")
	h.wantNoView("Min hash length")
	// Курсор остался на той же настройке.
	if h.settingKey() != "issue_id_mode" {
		t.Fatalf("курсор на %s", h.settingKey())
	}
	// dolt.auto-commit: off и batch — с предупреждением, on — без.
	h.gotoSetting("dolt.auto-commit")
	h.keys("enter")
	h.wantView("dolt.auto-commit: on (default) → off", "uncommitted")
	h.keys("n")
	h.gotoSetting("dolt.auto-push")
	h.keys("enter")
	h.wantView("dolt.auto-push: false (default) → true", "Needs a remote; concurrent pushes can corrupt data.")
}

// u на заданном ключе — сброс к умолчанию через bd config unset; на незаданном — ничего.
func TestSettingsUnset(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.fake.SetConfigList(`{"issue_prefix":"demo-app","issue_id_mode":"counter"}`)
	h.openSettings()
	h.gotoSetting("issue_id_mode")
	h.keys("u")
	h.wantView("issue_id_mode: counter → hash (default)", "Affects new tasks only")
	lists := h.fake.Count("list")
	h.keys("y")
	h.wantWrites("config-unset", []string{"config", "unset", "issue_id_mode"})
	h.wantCount("list", lists+1)
	h.gotoSetting("backup.enabled")
	h.keys("u")
	if h.m.set.mode != setMain {
		t.Fatal("u на незаданном ключе")
	}
	h.wantCount("config-unset", 1)
}

// Ошибка записи — очищенная первая строка под строкой настройки; настройки перечитаны.
func TestSettingsSaveErrorInline(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("backup.enabled")
	h.fake.Fail("config-set", "Error: \x1b[31mmigration freeze\x1b[0m\nsecond line", 14)
	lists := h.fake.Count("list")
	h.keys("enter", "y")
	h.wantView("⚠ not saved: Error: migration freeze")
	h.wantNoView("second line")
	if n := len(h.callsOf("config-list")); n != 2 {
		t.Fatalf("после ошибки не перечитано: %d", n)
	}
	h.wantCount("list", lists)
	// Ошибка видна и когда курсор уходит.
	h.keys("k")
	h.wantView("not saved")
}

// Во время записи — «saving…», клавиши не работают, кроме Ctrl+C.
func TestSettingsSavingBlocksInput(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("backup.enabled")
	h.holdSave = true
	h.keys("enter", "y")
	h.wantView("saving…")
	h.keys("esc", "j", "enter", "u")
	if !h.m.set.open || h.settingKey() != "backup.enabled" || h.m.set.mode != setMain {
		t.Fatalf("клавиши во время записи: open=%v key=%s", h.m.set.open, h.settingKey())
	}
	h.holdSave = false
	h.holdSet = true
	h.release()
	// Пока перечитывается — по-прежнему «saving…».
	h.wantView("saving…")
	h.holdSet = false
	h.release()
	h.wantNoView("saving…")
	h.keys("esc")
	if h.m.set.open {
		t.Fatal("Esc после записи")
	}
	h = settingsHarness(t, fixtureMain)
	h.openSettings()
	h.holdSave = true
	h.gotoSetting("backup.enabled")
	h.keys("enter", "y", "ctrl+c")
	if !h.quit {
		t.Fatal("Ctrl+C во время записи")
	}
}

// int: поле ввода с проверкой диапазона; ошибка не закрывает поле, Esc отменяет.
func TestSettingsIntEditor(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("hierarchy.max-depth")
	h.keys("enter")
	h.wantView("value: 3", "Enter check · Esc cancel")
	h.keys("backspace")
	h.typeText("0")
	h.keys("enter")
	h.wantView("⚠ enter a number from 1 to 10")
	if h.m.set.mode != setNumber {
		t.Fatal("поле закрылось после ошибки")
	}
	h.keys("backspace")
	h.typeText("x")
	h.keys("enter")
	h.wantView("⚠ enter a whole number")
	h.keys("esc")
	if h.m.set.mode != setMain {
		t.Fatal("Esc не отменил ввод")
	}
	h.wantCount("config-set", 0)
	h.keys("enter", "backspace")
	h.typeText("5")
	h.keys("enter")
	h.wantView("hierarchy.max-depth: 3 (default) → 5")
	h.keys("y")
	h.wantWrites("config-set", []string{"config", "set", "hierarchy.max-depth", "5"})

	// min ≤ max: max по умолчанию 8.
	h.gotoSetting("min_hash_length")
	h.keys("enter")
	h.wantView("value: 3")
	h.keys("backspace")
	h.typeText("9")
	h.keys("enter")
	h.wantView("⚠ must not exceed max_hash_length (8)")
	h.keys("backspace")
	h.typeText("2")
	h.keys("enter")
	h.wantView("⚠ enter a number from 3 to 10")
	h.keys("esc")
	// int ≥ 1 без верхней границы.
	h.gotoSetting("compact_tier1_days")
	h.keys("enter", "backspace", "backspace")
	h.typeText("0")
	h.keys("enter")
	h.wantView("⚠ enter a number from 1")
	h.keys("backspace")
	h.typeText("400")
	h.keys("enter")
	h.wantView("compact_tier1_days: 30 → 400")
}

// float: 0.01–0.5, значение нормализуется.
func TestSettingsFloatEditor(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("max_collision_prob")
	h.keys("enter")
	h.wantView("value: 0.25")
	for range 4 {
		h.keys("backspace")
	}
	h.typeText("0.9")
	h.keys("enter")
	h.wantView("⚠ enter a number from 0.01 to 0.5")
	h.keys("backspace", "backspace", "backspace")
	h.typeText(".10")
	h.keys("enter")
	h.wantView("max_collision_prob: 0.25 (default) → 0.1")
	h.keys("y")
	h.wantWrites("config-set", []string{"config", "set", "max_collision_prob", "0.1"})
}

// Редактор списка: встроенные только для чтения, добавление с проверками имени,
// удаление, предупреждение о задачах с удалённым статусом, перечитывание списка задач.
func TestSettingsListEditorStatus(t *testing.T) {
	h := settingsHarness(t, fixtureSettings)
	h.fake.SetConfigList(`{"issue_prefix":"p","status.custom":"awaiting_review,qa"}`)
	h.openSettings()
	h.gotoSetting("status.custom")
	h.wantView("awaiting_review, qa")
	h.keys("enter")
	h.wantView("Custom statuses · status.custom", "built-in:", "open, in_progress, blocked, deferred, closed",
		"custom:", "awaiting_review", "qa", "a add · x remove · Enter save · Esc cancel")
	h.keys("a")
	h.wantView("new: ", "Enter add · Esc cancel")
	h.typeText("Bad-Name")
	h.keys("enter")
	h.wantView("⚠ use a–z, 0–9 and _ only")
	for range 8 {
		h.keys("backspace")
	}
	h.typeText("qa")
	h.keys("enter")
	h.wantView("⚠ qa is already in the list")
	h.keys("backspace", "backspace")
	h.typeText("closed")
	h.keys("enter")
	h.wantView("⚠ closed is built in")
	for range 6 {
		h.keys("backspace")
	}
	h.typeText("needs_docs")
	h.keys("enter")
	if h.m.set.adding || !slices.Equal(h.m.set.items, []string{"awaiting_review", "qa", "needs_docs"}) {
		t.Fatalf("после добавления: adding=%v items=%q", h.m.set.adding, h.m.set.items)
	}
	// Курсор на первом своём элементе: x удаляет awaiting_review.
	h.keys("g", "x")
	if !slices.Equal(h.m.set.items, []string{"qa", "needs_docs"}) {
		t.Fatalf("после удаления: %q", h.m.set.items)
	}
	lists := h.fake.Count("list")
	h.keys("enter")
	h.wantView("status.custom: awaiting_review, qa → qa, needs_docs", "⚠ 1 task uses awaiting_review")
	h.keys("y")
	h.wantWrites("config-set", []string{"config", "set", "status.custom", "qa,needs_docs"})
	h.wantCount("list", lists+1)

	// Esc в редакторе отменяет всё; d тоже удаляет.
	h.keys("enter", "d", "esc")
	if h.m.set.mode != setMain {
		t.Fatal("Esc не закрыл редактор")
	}
	h.wantCount("config-set", 1)
	// Без изменений Enter закрывает редактор без подтверждения.
	h.keys("enter", "enter")
	if h.m.set.mode != setMain {
		t.Fatalf("Enter без изменений: mode %v", h.m.set.mode)
	}
	h.wantCount("config-set", 1)
}

// types.custom: встроенные из bd types, удаление используемого типа с предупреждением,
// пустой список — сброс через unset, затем перечитывание списка задач.
func TestSettingsListEditorTypes(t *testing.T) {
	h := settingsHarness(t, fixtureSettings)
	h.fake.SetConfigGet("types.custom", `{"key":"types.custom","value":"spike"}`)
	h.openSettings()
	h.gotoSetting("types.custom")
	h.keys("enter")
	h.wantView("task, bug, feature, chore, epic, decision", "spike")
	h.keys("a")
	h.typeText("bug")
	h.keys("enter")
	h.wantView("⚠ bug is built in")
	h.keys("esc")
	if h.m.set.mode != setListEdit || h.m.set.adding {
		t.Fatal("Esc в поле добавления должен вернуть в редактор")
	}
	h.keys("x")
	lists := h.fake.Count("list")
	h.keys("enter")
	h.wantView("types.custom: spike → (default)", "⚠ 2 tasks use spike")
	h.keys("y")
	h.wantWrites("config-unset", []string{"config", "unset", "types.custom"})
	h.wantCount("list", lists+1)
}

// Встроенные элементы выбрать и удалить нельзя: курсор ходит только по своим.
func TestSettingsListBuiltinsProtected(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.openSettings()
	h.gotoSetting("status.custom")
	h.keys("enter")
	h.wantView("(none)")
	h.keys("x", "d", "k", "j", "x")
	if len(h.m.set.items) != 0 {
		t.Fatalf("items = %q", h.m.set.items)
	}
	h.keys("enter")
	if h.m.set.mode != setMain {
		t.Fatal("пустой список без изменений должен закрыться")
	}
	h.wantCount("config-set", 0)
	h.wantCount("config-unset", 0)
}

// Записи: только comments add, init, config set и config unset идут без --readonly.
func TestSettingsWriteDiscipline(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.keys("m")
	h.typeText("note")
	h.keys("ctrl+s")
	h.openSettings()
	h.gotoSetting("backup.enabled")
	h.keys("enter", "y")
	h.fake.SetConfigList(`{"issue_prefix":"demo-app","issue_id_mode":"counter"}`)
	h.keys("esc")
	h.openSettings()
	h.gotoSetting("issue_id_mode")
	h.keys("u", "y", "esc", "r")
	h.tick()
	seen := wantWriteDiscipline(t, h.fake.Calls())
	if seen["config-set"] != 1 || seen["config-unset"] != 1 || seen["comments-add"] != 1 ||
		seen["config-list"] < 3 || seen["config-get"] == 0 || seen["types"] == 0 || seen["list"] < 3 {
		t.Fatalf("вызовы: %v", seen)
	}
}

// Экраны настроек помещаются в 80x24, 70x24, 40x12 и 20x6 на обоих языках; меньше —
// одна строка TooSmall.
func TestSettingsScreensFit(t *testing.T) {
	for _, l := range []i18n.Lang{i18n.EN, i18n.RU} {
		for _, sz := range [][2]int{{80, 24}, {70, 24}, {40, 12}, {20, 6}} {
			w, ht := sz[0], sz[1]
			what := fmt.Sprintf("%v %dx%d", l, w, ht)
			h := settingsHarness(t, fixtureSettings, func(o *Options) { o.Lang = l })
			h.fake.SetConfigList(`{"issue_prefix":"demo-app","status.custom":"` + strings.Repeat("very_long_status_name,", 8) + `qa"}`)
			h.send(tea.WindowSizeMsg{Width: w, Height: ht})
			h.holdSet = true
			h.keys(",")
			h.wantFits(w, ht, what+" загрузка")
			h.holdSet = false
			h.release()
			for i := 0; i < 20; i++ {
				h.wantFits(w, ht, fmt.Sprintf("%s строка %d", what, i))
				h.keys("j")
			}
			h.gotoSetting("hierarchy.max-depth")
			h.keys("enter")
			h.wantFits(w, ht, what+" поле числа")
			h.typeText("99")
			h.keys("enter")
			h.wantFits(w, ht, what+" ошибка числа")
			h.keys("esc")
			h.gotoSetting("status.custom")
			h.keys("enter")
			h.wantFits(w, ht, what+" редактор списка")
			for i := 0; i < 12; i++ {
				h.keys("j")
				h.wantFits(w, ht, what+" редактор списка, прокрутка")
			}
			h.keys("a")
			h.typeText("Bad")
			h.keys("enter")
			h.wantFits(w, ht, what+" ошибка имени")
			h.keys("esc", "x", "enter")
			h.wantFits(w, ht, what+" подтверждение списка")
			h.keys("n", "esc")
			h.gotoSetting("issue_id_mode")
			h.keys("enter")
			h.wantFits(w, ht, what+" подтверждение")
			if w >= 70 {
				h.fake.Fail("config-set", "Error: "+strings.Repeat("long failure ", 20), 1)
				h.holdSave = true
				h.keys("y")
				h.wantFits(w, ht, what+" запись")
				h.wantView(i18n.For(l).SettingsSaving)
				h.holdSave = false
				h.release()
				h.wantFits(w, ht, what+" ошибка записи")
			}
			if w == 80 {
				h.wantView(i18n.For(l).SettingsHeader("demo-app"))
			}
		}
		h := settingsHarness(t, fixtureMain, func(o *Options) { o.Lang = l })
		h.openSettings()
		h.send(tea.WindowSizeMsg{Width: 19, Height: 5})
		if v := h.view(); strings.Contains(v, "\n") || !strings.HasPrefix(i18n.For(l).TooSmall, v) {
			t.Fatalf("%v крошечное окно: %q", l, v)
		}
	}
}

// В 80x24 видны все секции первого экрана, курсор при прокрутке остаётся видимым.
func TestSettingsScrollKeepsCursor(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.openSettings()
	h.keys("G")
	if h.settingKey() != "compact_tier2_days" {
		t.Fatalf("G: %s", h.settingKey())
	}
	h.wantView("Compact tier 2 after, days", "Age of closed tasks for tier 2")
	h.keys("g")
	h.wantView("ID prefix", "IDs")
}

// В английском интерфейсе экранов настроек нет кириллицы.
func TestSettingsNoCyrillicEN(t *testing.T) {
	h := settingsHarness(t, fixtureSettings)
	h.fake.SetConfigList(`{"issue_prefix":"demo-app","status.custom":"awaiting_review"}`)
	h.holdSet = true
	h.keys(",")
	h.noCyrillic("загрузка настроек")
	h.holdSet = false
	h.release()
	for i := 0; i < 20; i++ {
		h.noCyrillic("строки настроек")
		h.keys("j")
	}
	h.gotoSetting("min_hash_length")
	h.keys("enter", "backspace")
	h.typeText("99")
	h.keys("enter")
	h.noCyrillic("ошибка числа")
	h.keys("esc")
	h.gotoSetting("status.custom")
	h.keys("enter", "a")
	h.typeText("X")
	h.keys("enter")
	h.noCyrillic("редактор списка")
	h.keys("esc", "x", "enter")
	h.noCyrillic("подтверждение списка")
	h.keys("n", "esc")
	h.gotoSetting("dolt.auto-commit")
	h.keys("enter")
	h.noCyrillic("подтверждение")
	h.fake.Fail("config-set", "Error: nope", 1)
	h.holdSave = true
	h.keys("y")
	h.noCyrillic("запись")
	h.holdSave = false
	h.release()
	h.noCyrillic("ошибка записи")
	h.fake.Fail("config-list", "Error: nope", 1)
	h.keys("esc", ",")
	h.noCyrillic("ошибка чтения")
	h.send(tea.WindowSizeMsg{Width: 70, Height: 24})
	h.noCyrillic("узкий")
}

// Русский экран: заголовок, секции, пометка умолчания, подсказка.
func TestSettingsRussian(t *testing.T) {
	h := settingsHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	h.openSettings()
	h.wantView("Настройки · demo-app", "ID задач", "Префикс ID", "hash (по умолчанию)",
		"j k выбор · Enter изменить · u сброс · Esc назад", "меняется командой bd rename-prefix")
	h.gotoSetting("issue_id_mode")
	h.keys("enter")
	h.wantView("issue_id_mode: hash (по умолчанию) → counter", "Действует только на новые задачи",
		"y — применить · любая другая клавиша — отмена")
}

// Справка: «,» — настройки; по-прежнему 80x24 на обоих языках.
func TestHelpSettingsKey(t *testing.T) {
	for _, c := range []struct {
		lang   i18n.Lang
		action string
	}{{i18n.EN, "bd settings"}, {i18n.RU, "настройки bd"}} {
		h := newHarness(t, fixtureMain, func(o *Options) { o.Lang = c.lang })
		h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
		h.keys("?")
		h.wantFits(80, 24, "справка "+c.lang.String())
		h.wantHelpEntry(",", c.action)
		if _, _, maxOff := h.m.helpWindow(); maxOff != 0 {
			t.Fatalf("%v: справка прокручивается", c.lang)
		}
	}
}

// Ответ чтения, пришедший после Esc (или после повторного открытия), отбрасывается.
func TestSettingsStaleLoadDropped(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.holdSet = true
	h.keys(",", "esc")
	h.holdSet = false
	h.release()
	if h.m.set.open {
		t.Fatal("опоздавший ответ открыл настройки")
	}
	h.holdSet = true
	h.keys(",")
	stale := h.held
	h.held = nil
	h.keys("esc", ",")
	h.held = append(stale, h.held...)
	h.holdSet = false
	h.release()
	if !h.m.set.open || !h.m.set.loaded {
		t.Fatal("настройки не загрузились")
	}
}

// Запись не уложилась в DetailTimeout — ошибка таймаута под строкой.
func TestSettingsSaveTimeout(t *testing.T) {
	h := settingsHarness(t, fixtureMain, func(o *Options) { o.DetailTimeout = 300 * time.Millisecond })
	h.openSettings()
	h.gotoSetting("backup.enabled")
	h.fake.Sleep("config-set", 3)
	h.keys("enter", "y")
	h.wantView("⚠ not saved: bd did not respond in 0.3 s")
}

// v1.3 fix: окно меньше 20x6 показывает «window too small» — y в скрытом подтверждении
// не пишет ни в настройках, ни в bd init.
func TestConfirmBelowMinSizeBlocksY(t *testing.T) {
	for _, sz := range [][2]int{{19, 40}, {80, 5}} {
		h := settingsHarness(t, fixtureMain)
		h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
		h.openSettings()
		h.gotoSetting("backup.enabled")
		h.keys("enter")
		h.wantView("backup.enabled: false (default) → true")
		h.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		h.wantView("window too")
		h.keys("y")
		h.wantCount("config-set", 0)

		n := newNoDB(t)
		n.keys("i")
		n.send(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
		n.keys("y")
		n.wantCount("init", 0)
		if n.m.initRunning {
			t.Fatalf("%v: y запустил bd init", sz)
		}
	}
}

// v1.3 fix: YAML-ключ без location config.yaml (в 0.59 — не YAML, запись осиротела бы
// в базе) скрыт; types.custom виден и без location.
func TestSettingsYAMLWithoutLocationHidden(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.fake.SetConfigGet("backup.enabled", `{"key":"backup.enabled","value":""}`)
	h.openSettings()
	h.wantNoView("Auto-export JSONL", "Backups")
	h.wantView("Custom types", "Dolt auto-push")
	h.fake.SetConfigGet("types.custom", `{"key":"types.custom","location":"config.yaml","value":"spike"}`)
	h.keys("esc")
	h.openSettings()
	h.wantView("Custom types", "spike")
}

// v1.3 fix: u не сбрасывает ключ, если min_hash_length станет больше max_hash_length.
func TestSettingsUnsetKeepsMinMax(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.fake.SetConfigList(`{"issue_prefix":"p","min_hash_length":"9","max_hash_length":"10"}`)
	h.openSettings()
	h.gotoSetting("max_hash_length")
	h.keys("u")
	if h.m.set.mode != setMain {
		t.Fatalf("u открыл подтверждение: mode %v", h.m.set.mode)
	}
	h.wantView("⚠ cannot reset: min_hash_length (9) must not exceed max_hash_length (8)")
	// Сообщение уходит со следующей клавишей; сброс min допустим (умолчание 3 ≤ 10).
	h.gotoSetting("min_hash_length")
	h.wantNoView("cannot reset")
	h.keys("u")
	h.wantView("min_hash_length: 9 → 3 (default)")
	h.keys("n")
	h.wantCount("config-unset", 0)
}

// v1.3 fix: упавшие config get не пропадают молча: строка с числом и r — повторить.
func TestSettingsReadFailuresShown(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.fake.FailID("config-get", "backup.enabled", "Error: database is locked", 1)
	h.fake.FailID("config-get", "dolt.auto-push", "Error: database is locked", 1)
	h.openSettings()
	h.wantView("2 settings could not be read · r — retry")
	h.wantNoView("Backups")
	h.fake.FailID("config-get", "backup.enabled", "", 0)
	h.fake.FailID("config-get", "dolt.auto-push", "", 0)
	h.keys("r")
	h.wantNoView("could not be read")
	h.wantView("Backups", "Dolt auto-push")

	r := settingsHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	r.fake.FailID("config-get", "backup.enabled", "Error: locked", 1)
	r.openSettings()
	r.wantView("не удалось прочитать 1 настройку · r — повторить")
}

// v1.3 fix: подвал напоминает, что значения могут переопределяться окружением.
func TestSettingsEnvNote(t *testing.T) {
	h := settingsHarness(t, fixtureMain)
	h.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	h.openSettings()
	h.wantView("BD_* env vars or the global config may override")
	r := settingsHarness(t, fixtureMain, func(o *Options) { o.Lang = i18n.RU })
	r.send(tea.WindowSizeMsg{Width: 80, Height: 24})
	r.openSettings()
	r.wantView("BD_*")
	if d := i18n.For(i18n.RU).SettingDescs["max_collision_prob"]; !strings.Contains(d, "0.01–0.5") {
		t.Fatalf("RU описание: %q", d)
	}
}
