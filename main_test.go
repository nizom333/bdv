package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"unicode"

	"github.com/nizom333/bdv/internal/i18n"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    config
		wantErr bool
	}{
		{"пусто", nil, config{}, false},
		{"id", []string{"ep.1"}, config{ID: "ep.1"}, false},
		{"db", []string{"--db", "/p"}, config{DB: "/p"}, false},
		{"db=", []string{"--db=/p", "x"}, config{DB: "/p", ID: "x"}, false},
		{"два id", []string{"a", "b"}, config{}, true},
		{"db без значения", []string{"--db"}, config{}, true},
		{"help", []string{"-h"}, config{Help: true}, false},
		{"help длинный", []string{"--help"}, config{Help: true}, false},
		{"version", []string{"--version"}, config{Version: true}, false},
		{"неизвестный флаг", []string{"--foo"}, config{}, true},
		{"неизвестный короткий", []string{"-x"}, config{}, true},
	}
	for _, c := range cases {
		got, err := parseArgs(c.args, en)
		if (err != nil) != c.wantErr {
			t.Errorf("%s: err=%v", c.name, err)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

func TestFindBeads(t *testing.T) {
	// Найденный путь канонический (canonPath): на macOS TempDir лежит за ссылкой /var.
	tmp := canonPath(t.TempDir())
	deep := filepath.Join(tmp, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, "a", ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := findBeads(deep, "")
	if !ok || got != filepath.Join(tmp, "a", ".beads") {
		t.Errorf("got %q %v", got, ok)
	}
	if _, ok := findBeads(tmp, ""); ok {
		t.Error("выше .beads не должно находиться")
	}
}

func TestFindBeadsIgnoresFile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".beads"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := findBeads(tmp, ""); ok {
		t.Error("файл .beads не считается")
	}
}

func okLook(string) (string, error)  { return "/bin/bd", nil }
func badLook(string) (string, error) { return "", errors.New("нет") }
func noEnv(string) string            { return "" }

var en = i18n.For(i18n.EN)

func TestPreflightNoBD(t *testing.T) {
	err := preflight(config{}, en, noEnv, badLook, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "bd not found in PATH") {
		t.Errorf("err=%v", err)
	}
}

func TestPreflightNoBeads(t *testing.T) {
	err := preflight(config{}, en, noEnv, okLook, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "no .beads directory") {
		t.Errorf("err=%v", err)
	}
}

func TestPreflightSkipsLookupWithDB(t *testing.T) {
	if err := preflight(config{DB: "/x"}, en, noEnv, okLook, t.TempDir()); err != nil {
		t.Error(err)
	}
}

func TestPreflightSkipsLookupWithEnv(t *testing.T) {
	for _, k := range []string{"BEADS_DIR", "BEADS_DB"} {
		env := func(s string) string {
			if s == k {
				return "/x"
			}
			return ""
		}
		if err := preflight(config{}, en, env, okLook, t.TempDir()); err != nil {
			t.Errorf("%s: %v", k, err)
		}
	}
}

func TestRunExitCodes(t *testing.T) {
	started := false
	start := func(config) error { started = true; return nil }
	run1 := func(args []string, look func(string) (string, error), cwd string) (int, string, string) {
		var out, errb bytes.Buffer
		code := run(args, noEnv, look, cwd, &out, &errb, start)
		return code, out.String(), errb.String()
	}
	tmp := t.TempDir()

	code, out, _ := run1([]string{"--help"}, okLook, tmp)
	if code != 0 || !strings.Contains(out, "usage: bdv") || !strings.Contains(out, "Tab") {
		t.Errorf("help: %d %q", code, out)
	}
	version, commit, date = "1.2", "abc", "2026"
	code, out, _ = run1([]string{"--version"}, okLook, tmp)
	if code != 0 || strings.TrimSpace(out) != "bdv 1.2 (abc, 2026)" {
		t.Errorf("version: %d %q", code, out)
	}
	commit = ""
	_, out, _ = run1([]string{"--version"}, okLook, tmp)
	if strings.TrimSpace(out) != "bdv 1.2" {
		t.Errorf("version без коммита: %q", out)
	}
	code, _, errs := run1([]string{"a", "b"}, okLook, tmp)
	if code != 1 || !strings.Contains(errs, "usage: bdv [--db <path>] [id]") {
		t.Errorf("bad arg: %d %q", code, errs)
	}
	code, _, errs = run1(nil, badLook, tmp)
	if code != 1 || !strings.Contains(errs, "bd not found in PATH") {
		t.Errorf("preflight: %d %q", code, errs)
	}
	if started {
		t.Fatal("TUI не должен стартовать при ошибках")
	}
	code, _, _ = run1([]string{"--db", "/x"}, okLook, tmp)
	if code != 0 || !started {
		t.Errorf("старт: %d %v", code, started)
	}
}

// Стиль glamour определяется один раз до старта TUI: без терминала — notty,
// иначе dark или light по фону. "auto" не возвращается никогда: он опрашивал бы
// терминал изнутри Update и съедал нажатия.
func TestMarkdownStyle(t *testing.T) {
	cases := []struct {
		tty, dark bool
		want      string
	}{
		{false, true, "notty"},
		{false, false, "notty"},
		{true, true, "dark"},
		{true, false, "light"},
	}
	for _, c := range cases {
		if got := markdownStyle(c.tty, c.dark); got != c.want {
			t.Errorf("markdownStyle(%v, %v) = %q, want %q", c.tty, c.dark, got, c.want)
		}
	}
}

func envOf(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

// --help по-английски при пустом окружении.
func TestHelpEN(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--help"}, noEnv, okLook, t.TempDir(), &out, &errb, func(config) error { return nil })
	s := out.String()
	if code != 0 || !strings.Contains(s, "usage: bdv [--db <path>] [id]") || !strings.Contains(s, "copy ID") {
		t.Fatalf("help: %d %q", code, s)
	}
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			t.Fatalf("кириллица в английской справке: %q", s)
		}
	}
}

// Русская локаль: --help, ошибки аргументов и preflight по-русски, язык уходит в TUI.
func TestRunRussian(t *testing.T) {
	env := envOf(map[string]string{"LANG": "ru_RU.UTF-8"})
	var got config
	start := func(c config) error { got = c; return nil }
	run1 := func(args []string, look func(string) (string, error)) (int, string, string) {
		var out, errb bytes.Buffer
		code := run(args, env, look, t.TempDir(), &out, &errb, start)
		return code, out.String(), errb.String()
	}
	if code, out, _ := run1([]string{"--help"}, okLook); code != 0 || !strings.Contains(out, "использование: bdv") ||
		!strings.Contains(out, "копировать ID") {
		t.Errorf("help: %d %q", code, out)
	}
	if code, _, errs := run1([]string{"a", "b"}, okLook); code != 1 || !strings.Contains(errs, "лишний аргумент") ||
		!strings.Contains(errs, "использование: bdv") {
		t.Errorf("bad arg: %d %q", code, errs)
	}
	if code, _, errs := run1(nil, badLook); code != 1 || !strings.Contains(errs, "команда bd не найдена") {
		t.Errorf("preflight: %d %q", code, errs)
	}
	// Без .beads и без --db/BEADS_*: не выход, а TUI с предложением bd init.
	if code, _, errs := run1(nil, okLook); code != 0 || !got.StartNoDB || errs != "" {
		t.Errorf("preflight .beads: %d %q, StartNoDB = %v", code, errs, got.StartNoDB)
	}
	if code, _, _ := run1([]string{"--db", "/x"}, okLook); code != 0 || got.Lang != i18n.RU {
		t.Errorf("старт: %d, Lang = %v", code, got.Lang)
	}
}

// Английский по умолчанию: язык в конфигурации TUI — EN.
func TestRunPassesLangEN(t *testing.T) {
	got := config{Lang: i18n.RU}
	var out, errb bytes.Buffer
	code := run([]string{"--db", "/x"}, noEnv, okLook, t.TempDir(), &out, &errb, func(c config) error { got = c; return nil })
	if code != 0 || got.Lang != i18n.EN {
		t.Errorf("старт: %d, Lang = %v", code, got.Lang)
	}
}

// Имя проекта — папка, в которой лежит .beads: из --db, BEADS_DIR, BEADS_DB или поиска
// от cwd вверх; не определилось — пустая строка.
func TestProjectName(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "demo-app")
	sub := filepath.Join(proj, "internal", "app")
	if err := os.MkdirAll(filepath.Join(proj, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		db   string
		env  map[string]string
		cwd  string
		want string
	}{
		{"--db на .beads", "/w/X/.beads", nil, root, "X"},
		{"--db на .beads со слешем", "/w/X/.beads/", nil, root, "X"},
		{"--db на файл внутри .beads", "/w/X/.beads/beads.db", nil, root, "X"},
		{"--db на файл вне .beads", "/w/Y/custom.db", nil, root, "Y"},
		{"--db относительный", ".beads", nil, proj, "demo-app"},
		{"--db важнее BEADS_DIR", "/w/X/.beads", map[string]string{"BEADS_DIR": "/w/Z/.beads"}, root, "X"},
		{"BEADS_DIR", "", map[string]string{"BEADS_DIR": "/w/Z/.beads"}, root, "Z"},
		{"BEADS_DB", "", map[string]string{"BEADS_DB": "/w/Q/.beads/db"}, root, "Q"},
		{"поиск от cwd вверх", "", nil, sub, "demo-app"},
		{"не найдено", "", nil, root, ""},
		{"корень", "/", nil, root, ""},
	}
	for _, c := range cases {
		got := projectName(config{DB: c.db}, envOf(c.env), c.cwd)
		if got != c.want {
			t.Errorf("%s: projectName = %q, want %q", c.name, got, c.want)
		}
	}
	// run передаёт имя в конфигурацию TUI.
	var cfg config
	var out, errb bytes.Buffer
	code := run(nil, noEnv, okLook, sub, &out, &errb, func(c config) error { cfg = c; return nil })
	if code != 0 || cfg.Project != "demo-app" {
		t.Errorf("run: %d, Project = %q (%s)", code, cfg.Project, errb.String())
	}
}

// Нет .beads, а --db, BEADS_DIR и BEADS_DB пусты: bdv не выходит, а стартует TUI сразу
// на экране без базы (StartNoDB) с разрешённым bd init в текущей папке.
func TestPreflightNoBeadsOffersInit(t *testing.T) {
	cwd := t.TempDir()
	var got config
	calls := 0
	var out, errb bytes.Buffer
	code := run(nil, noEnv, okLook, cwd, &out, &errb, func(c config) error { got = c; calls++; return nil })
	if code != 0 || calls != 1 || errb.String() != "" {
		t.Fatalf("run: %d, start вызван %d раз, stderr %q", code, calls, errb.String())
	}
	if !got.StartNoDB || !got.InitAllowed || got.InitDir != cwd {
		t.Errorf("config = %+v, want StartNoDB, InitAllowed, InitDir %q", got, cwd)
	}
	// bd нет в PATH — по-прежнему ошибка, TUI не стартует.
	calls = 0
	if code := run(nil, noEnv, badLook, cwd, &out, &errb, func(config) error { calls++; return nil }); code != 1 || calls != 0 {
		t.Errorf("без bd: %d, start %d", code, calls)
	}
}

// --db, BEADS_DIR или BEADS_DB заданы: bd init не предлагается ни при старте, ни потом
// (InitAllowed выключен) — прежнее поведение: обычный старт, ошибку покажет экран ошибки bd.
func TestPreflightNoBeadsWithDBStillFails(t *testing.T) {
	cwd := t.TempDir()
	for _, c := range []struct {
		name string
		args []string
		env  map[string]string
	}{
		{"--db", []string{"--db", "/nope/.beads"}, nil},
		{"BEADS_DIR", nil, map[string]string{"BEADS_DIR": "/nope/.beads"}},
		{"BEADS_DB", nil, map[string]string{"BEADS_DB": "/nope/.beads/db"}},
	} {
		var got config
		var out, errb bytes.Buffer
		code := run(c.args, envOf(c.env), okLook, cwd, &out, &errb, func(cfg config) error { got = cfg; return nil })
		if code != 0 || got.StartNoDB || got.InitAllowed {
			t.Errorf("%s: %d, config = %+v", c.name, code, got)
		}
	}
	// Сам preflight по-прежнему сообщает об отсутствии .beads: решение «предложить
	// bd init или выйти» принимает run по InitAllowed.
	if err := preflight(config{}, en, noEnv, okLook, cwd); err == nil || !strings.Contains(err.Error(), "no .beads directory") {
		t.Errorf("preflight: %v", err)
	}
}

// InitDir и InitAllowed доходят до конфигурации TUI и при найденном .beads: ошибку
// «no beads database found» модель распознает сама.
func TestRunPassesInitDirAndAllowed(t *testing.T) {
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	if err := os.MkdirAll(filepath.Join(proj, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	var got config
	var out, errb bytes.Buffer
	code := run(nil, noEnv, okLook, proj, &out, &errb, func(c config) error { got = c; return nil })
	if code != 0 || !got.InitAllowed || got.StartNoDB || got.InitDir != proj || got.Project != "proj" {
		t.Errorf("run: %d, config = %+v", code, got)
	}
	o := appOptions(config{InitAllowed: true, InitDir: "/w/p", StartNoDB: true, Lang: i18n.RU, Project: "p", ID: "x"}, nil, "dark")
	if !o.InitAllowed || o.InitDir != "/w/p" || !o.StartNoDB || o.Lang != i18n.RU || o.Project != "p" || o.InitialID != "x" {
		t.Errorf("appOptions = %+v", o)
	}
}

// ~/.beads — глобальная папка bd (machine-id, eventsData), а не проект: findBeads её
// не считает и выше HOME не ищет. Вне HOME поиск идёт до корня, как раньше.
func TestFindBeadsSkipsHome(t *testing.T) {
	root := canonPath(t.TempDir())
	home := filepath.Join(root, "home")
	proj := filepath.Join(home, "work", "proj")
	for _, d := range []string{filepath.Join(root, ".beads"), filepath.Join(home, ".beads"), proj} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got, ok := findBeads(proj, home); ok {
		t.Errorf("под HOME нашёлся %q: ~/.beads и .beads выше HOME не считаются", got)
	}
	if got, ok := findBeads(home, home); ok {
		t.Errorf("в самом HOME нашёлся %q", got)
	}
	if got, ok := findBeads(proj, home+string(filepath.Separator)); ok {
		t.Errorf("HOME со слешем: нашёлся %q", got)
	}
	// Проект под HOME со своим .beads находится.
	if err := os.Mkdir(filepath.Join(home, "work", ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := findBeads(proj, home); !ok || got != filepath.Join(home, "work", ".beads") {
		t.Errorf("проект под HOME: %q %v", got, ok)
	}
	// Вне HOME — до корня: .beads в root находится.
	other := filepath.Join(root, "elsewhere")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := findBeads(other, home); !ok || got != filepath.Join(root, ".beads") {
		t.Errorf("вне HOME: %q %v", got, ok)
	}
}

// Новый проект под HOME с ~/.beads: preflight не принимает ~/.beads за базу, TUI стартует
// на экране без базы, имени проекта нет (его даст bd init).
func TestPreflightUnderHomeOffersInit(t *testing.T) {
	home := t.TempDir()
	proj := filepath.Join(home, "proj")
	for _, d := range []string{filepath.Join(home, ".beads"), proj} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var got config
	var out, errb bytes.Buffer
	code := run(nil, envOf(map[string]string{"HOME": home}), okLook, proj, &out, &errb, func(c config) error { got = c; return nil })
	if code != 0 || !got.StartNoDB || !got.InitAllowed || got.Project != "" || got.InitDir != proj {
		t.Errorf("run: %d %q, config = %+v", code, errb.String(), got)
	}
}

// HOME через символическую ссылку (или cwd через неё): findBeads сравнивает
// канонические пути (filepath.EvalSymlinks), поэтому поиск всё равно останавливается
// на HOME, а ~/.beads и .beads выше HOME не считаются.
func TestFindBeadsSymlinkedHome(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real", "home")
	proj := filepath.Join(real, "work", "proj")
	for _, d := range []string{filepath.Join(root, ".beads"), filepath.Join(real, ".beads"), proj} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "linkhome")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	// HOME — ссылка, cwd — настоящий путь (так его отдаёт os.Getwd без $PWD).
	if got, ok := findBeads(proj, link); ok {
		t.Errorf("HOME-ссылка: нашёлся %q", got)
	}
	// HOME настоящий, cwd — через ссылку.
	if got, ok := findBeads(filepath.Join(link, "work", "proj"), real); ok {
		t.Errorf("cwd через ссылку: нашёлся %q", got)
	}
	// Оба через ссылку.
	if got, ok := findBeads(filepath.Join(link, "work", "proj"), link); ok {
		t.Errorf("оба через ссылку: нашёлся %q", got)
	}
	// preflight с HOME-ссылкой тоже предлагает bd init, а не берёт чужой .beads.
	var got config
	var out, errb bytes.Buffer
	code := run(nil, envOf(map[string]string{"HOME": link}), okLook, proj, &out, &errb, func(c config) error { got = c; return nil })
	if code != 0 || !got.StartNoDB || got.Project != "" {
		t.Errorf("run с HOME-ссылкой: %d %q, config = %+v", code, errb.String(), got)
	}
	// Несуществующий HOME: EvalSymlinks не удаётся — Clean, поиск как раньше.
	if _, ok := findBeads(proj, filepath.Join(root, "nope")+string(filepath.Separator)); !ok {
		t.Error("несуществующий HOME: .beads выше не найден")
	}
}

type altStub bool

func (a altStub) AltScrollOff() bool { return bool(a) }

// После выхода из Bubble Tea alternate scroll (1007) возвращается, только если его
// выключил S и не включил обратно.
func TestRestoreAltScroll(t *testing.T) {
	var b bytes.Buffer
	restoreAltScroll(altStub(false), &b)
	if b.Len() != 0 {
		t.Fatalf("1007 не выключали, а записано %q", b.String())
	}
	restoreAltScroll(altStub(true), &b)
	if got := b.String(); got != "\x1b[?1007h" {
		t.Fatalf("записано %q, want CSI ?1007 h", got)
	}
}

// cwd через символическую ссылку: имя проекта — по пути, в котором стоит пользователь
// (как после bd init — filepath.Base(InitDir)), а не по разрешённому пути; поиск .beads
// идёт по разрешённому.
func TestProjectNameSymlinkedCwd(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real", "realproj")
	if err := os.MkdirAll(filepath.Join(real, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(real, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linkproj")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "nohome")
	for _, cwd := range []string{link, filepath.Join(link, "a", "b")} {
		got := projectName(config{}, envOf(map[string]string{"HOME": home}), cwd)
		if got != "linkproj" {
			t.Errorf("cwd %s: projectName = %q, want linkproj", cwd, got)
		}
	}
	// Без ссылок — как раньше.
	if got := projectName(config{}, envOf(map[string]string{"HOME": home}), filepath.Join(real, "a")); got != "realproj" {
		t.Errorf("настоящий путь: %q", got)
	}
}

func TestEffectiveVersion(t *testing.T) {
	mod := func(v string) *debug.BuildInfo { return &debug.BuildInfo{Main: debug.Module{Version: v}} }
	for _, tc := range []struct {
		name, v string
		info    *debug.BuildInfo
		want    string
	}{
		{"ldflags важнее", "1.2.3", mod("v9.9.9"), "1.2.3"},
		{"go install @vX", "dev", mod("v0.2.0"), "0.2.0"},
		{"рабочая копия", "dev", mod("(devel)"), "dev"},
		{"грязная копия", "dev", mod("v0.2.1-0.20260101000000-abcdef123456+dirty"), "dev"},
		{"псевдоверсия без тега", "dev", mod("v0.0.0-20261120211711-544aac4100ee"), "dev"},
		{"псевдоверсия после тега", "dev", mod("v0.1.1-0.20261120211711-544aac4100ee"), "dev"},
		{"пре-релиз", "dev", mod("v0.2.0-rc.1"), "0.2.0-rc.1"},
		{"без BuildInfo", "dev", nil, "dev"},
	} {
		if got := effectiveVersion(tc.v, tc.info); got != tc.want {
			t.Errorf("%s: effectiveVersion = %q, want %q", tc.name, got, tc.want)
		}
	}
}
