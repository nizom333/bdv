// bdv — TUI поверх CLI bd (beads): чтение с --readonly; записи — комментарий, bd init и bd config set после него.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/nizom333/bdv/internal/app"
	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/i18n"
)

// Заполняются через -ldflags (см. Makefile).
var (
	version = "dev"
	commit  string
	date    string
)

// config — разобранные аргументы командной строки.
type config struct {
	ID, DB        string
	Help, Version bool
	Lang          i18n.Lang // язык интерфейса: определяется в run по окружению
	Project       string    // имя проекта для верхней строки: определяется в run, см. projectName
	// InitAllowed — можно предлагать bd init: --db, BEADS_DIR и BEADS_DB не заданы,
	// база создаётся в текущей папке InitDir. Определяется в run.
	InitAllowed bool
	InitDir     string
	StartNoDB   bool // .beads не найден: TUI стартует сразу на экране без базы
}

// parseArgs разбирает аргументы: bdv [--db P | --db=P] [-h|--help] [--version] [id].
// Тексты ошибок — из каталога t.
func parseArgs(args []string, t *i18n.Catalog) (config, error) {
	var c config
	hasID := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			c.Help = true
		case a == "--version":
			c.Version = true
		case a == "--db":
			if i+1 >= len(args) {
				return config{}, errors.New(t.ErrDBValue)
			}
			i++
			c.DB = args[i]
		case strings.HasPrefix(a, "--db="):
			c.DB = strings.TrimPrefix(a, "--db=")
			if c.DB == "" {
				return config{}, errors.New(t.ErrDBValue)
			}
		case strings.HasPrefix(a, "-") && a != "-":
			return config{}, errors.New(t.ErrUnknownFlag(a))
		default:
			if hasID {
				return config{}, errors.New(t.ErrExtraArg(a))
			}
			c.ID, hasID = a, true
		}
	}
	return c, nil
}

// findBeads ищет каталог .beads от start вверх до корня; файл с таким именем не считается.
// home ("" — неизвестен): $HOME/.beads — глобальная папка bd (machine-id, eventsData),
// а не проект, поэтому она не считается, а выше HOME поиск не идёт. Вне HOME — до корня.
// start и home сравниваются канонически (canonPath): HOME или cwd через символическую
// ссылку всё равно останавливают поиск. Найденный путь — от канонического start.
func findBeads(start, home string) (string, bool) {
	if home != "" {
		home = canonPath(home)
	}
	dir := canonPath(start)
	for {
		if home != "" && dir == home {
			return "", false
		}
		p := filepath.Join(dir, ".beads")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// userBeads переводит канонический путь .beads от findBeads на путь, в котором стоит
// пользователь (cwd, возможно через символическую ссылку): столько же уровней вверх
// от cwd, сколько от канонического cwd до папки проекта. Имя проекта тогда совпадает
// с именем после bd init. Не получилось сопоставить (ссылка ниже папки проекта) —
// канонический путь как есть.
func userBeads(found, cwd string) string {
	rel, err := filepath.Rel(filepath.Dir(found), canonPath(cwd))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return found
	}
	dir := filepath.Clean(cwd)
	if rel != "." {
		for range strings.Split(rel, string(filepath.Separator)) {
			dir = filepath.Dir(dir)
		}
	}
	if canonPath(dir) != filepath.Dir(found) {
		return found
	}
	return filepath.Join(dir, ".beads")
}

// canonPath — путь без символических ссылок (filepath.EvalSymlinks); если разрешить
// не удалось (пути нет, нет прав), — просто filepath.Clean.
func canonPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// projectName — имя папки, в которой лежит .beads. Путь берётся из --db, BEADS_DIR
// или BEADS_DB: если в нём есть компонент .beads, имя — папка перед ним, иначе
// родитель пути. Без них .beads ищется от cwd вверх. Не определилось — "".
func projectName(c config, getenv func(string) string, cwd string) string {
	p := c.DB
	for _, k := range []string{"BEADS_DIR", "BEADS_DB"} {
		if p == "" {
			p = getenv(k)
		}
	}
	if p == "" {
		found, ok := findBeads(cwd, getenv("HOME"))
		if !ok {
			return ""
		}
		// Имя — по пути пользователя, как после bd init (папка InitDir = cwd).
		p = userBeads(found, cwd)
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)
	dir := filepath.Dir(p)
	for d := p; ; d = filepath.Dir(d) {
		if filepath.Base(d) == ".beads" {
			dir = filepath.Dir(d)
			break
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	name := filepath.Base(dir)
	if name == string(filepath.Separator) || name == "." {
		return ""
	}
	return name
}

// noBeadsError — .beads не найден от cwd вверх (а --db, BEADS_DIR и BEADS_DB пусты).
type noBeadsError struct{ msg string }

func (e noBeadsError) Error() string { return e.msg }

// initAllowed — bd init можно предлагать: база не задана ни флагом, ни окружением,
// иначе непонятно, где её создавать.
func initAllowed(c config, getenv func(string) string) bool {
	return c.DB == "" && getenv("BEADS_DIR") == "" && getenv("BEADS_DB") == ""
}

// preflight проверяет окружение до старта TUI; тексты ошибок — из каталога t.
// Ненайденный .beads — noBeadsError: run решает, предложить ли bd init.
func preflight(c config, t *i18n.Catalog, getenv func(string) string, lookPath func(string) (string, error), cwd string) error {
	if _, err := lookPath("bd"); err != nil {
		return errors.New(t.ErrNoBD)
	}
	if initAllowed(c, getenv) {
		if _, ok := findBeads(cwd, getenv("HOME")); !ok {
			return noBeadsError{t.ErrNoBeads(cwd)}
		}
	}
	return nil
}

func versionLine() string {
	info, _ := debug.ReadBuildInfo()
	v := effectiveVersion(version, info)
	if commit == "" || date == "" {
		return "bdv " + v
	}
	return fmt.Sprintf("bdv %s (%s, %s)", v, commit, date)
}

// effectiveVersion: без -ldflags (go install github.com/nizom333/bdv@vX.Y.Z) version
// остаётся dev, но Go записывает версию модуля в бинарник — тогда показываем её.
// Сборка из рабочей копии даёт (devel), +dirty или псевдоверсию (коммит без тега) —
// тогда остаётся dev: коммит и так виден в скобках у make build.
func effectiveVersion(v string, info *debug.BuildInfo) string {
	if v != "dev" || info == nil {
		return v
	}
	mv := strings.TrimPrefix(info.Main.Version, "v")
	if mv == "" || mv == "(devel)" || strings.Contains(mv, "+dirty") || pseudoVersion.MatchString(mv) {
		return v
	}
	return mv
}

// pseudoVersion — хвост псевдоверсии Go: время коммита и 12 символов хеша
// (v0.0.0-20261120211711-544aac4100ee, v0.1.1-0.20261120211711-544aac4100ee).
var pseudoVersion = regexp.MustCompile(`[-.]\d{14}-[0-9a-f]{12}$`)

// run — всё, кроме запуска TUI: start вызывается последним, с проверенной конфигурацией.
// Язык определяется один раз по getenv и идёт и в тексты командной строки, и в TUI.
func run(args []string, getenv func(string) string, lookPath func(string) (string, error), cwd string,
	stdout, stderr io.Writer, start func(config) error) int {
	lang := i18n.Detect(getenv)
	t := i18n.For(lang)
	c, err := parseArgs(args, t)
	if err != nil {
		fmt.Fprintf(stderr, "bdv: %v\n%s\n", err, t.Usage)
		return 1
	}
	c.Lang = lang
	if c.Help {
		fmt.Fprint(stdout, t.Usage+"\n"+t.HelpKeys)
		return 0
	}
	if c.Version {
		fmt.Fprintln(stdout, versionLine())
		return 0
	}
	c.InitAllowed, c.InitDir = initAllowed(c, getenv), cwd
	if err := preflight(c, t, getenv, lookPath, cwd); err != nil {
		// Нет .beads — сразу экран без базы. noBeadsError бывает только при
		// разрешённом bd init: при --db, BEADS_DIR или BEADS_DB .beads не ищется.
		var nb noBeadsError
		if !errors.As(err, &nb) {
			fmt.Fprintf(stderr, "bdv: %v\n", err)
			return 1
		}
		c.StartNoDB = true
	}
	c.Project = projectName(c, getenv, cwd)
	if err := start(c); err != nil {
		fmt.Fprintf(stderr, "bdv: %v\n", err)
		return 1
	}
	return 0
}

// markdownStyle — стиль glamour: без терминала notty, иначе dark или light по фону.
// Стиль "auto" не используется: glamour опрашивал бы терминал (OSC 11 + DSR) при каждом
// новом рендерере изнутри Update, споря с Bubble Tea за ввод.
func markdownStyle(isTTY, dark bool) string {
	switch {
	case !isTTY:
		return "notty"
	case dark:
		return "dark"
	}
	return "light"
}

// appOptions — настройки модели из конфигурации командной строки.
func appOptions(c config, src app.Source, style string) app.Options {
	return app.Options{
		Source:        src,
		InitialID:     c.ID,
		MarkdownStyle: style,
		Lang:          c.Lang,
		Project:       c.Project,
		InitAllowed:   c.InitAllowed,
		InitDir:       c.InitDir,
		StartNoDB:     c.StartNoDB,
	}
}

// startTUI запускает программу Bubble Tea.
func startTUI(c config) error {
	// Фон терминала определяется один раз до старта программы, пока Bubble Tea ещё
	// не читает stdin; заодно прогревается кеш lipgloss для AdaptiveColor.
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	style := markdownStyle(tty, tty && lipgloss.HasDarkBackground())
	src := bd.NewClient("bd", c.DB)
	m := app.New(appOptions(c, src, style))
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	// Корневой контекст отменяется всегда: при выходе по сигналу quit не вызывался.
	// Функция отмены общая у всех копий модели, поэтому без итоговой модели (ошибка
	// Run) годится и исходная.
	if fm, ok := final.(app.Model); ok {
		fm.Close()
		restoreAltScroll(fm, os.Stdout)
	} else {
		m.Close()
	}
	// Выход во время записи (комментарий, bd init): bd уже убит отменой контекста,
	// ждём, пока AddComment удалит временный файл и Init дождётся bd.
	src.Wait(time.Second)
	return err
}

// restoreAltScroll возвращает терминалу alternate scroll (DECSET 1007), если его
// выключил S в bdv (app.Model.AltScrollOff): иначе колесо в less, vim и прочих
// полноэкранных программах этого терминала перестало бы прокручивать.
func restoreAltScroll(m interface{ AltScrollOff() bool }, w io.Writer) {
	if m.AltScrollOff() {
		_, _ = io.WriteString(w, app.AltScrollOn)
	}
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bdv: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], os.Getenv, exec.LookPath, cwd, os.Stdout, os.Stderr, startTUI))
}
