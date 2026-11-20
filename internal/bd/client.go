package bd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Error — bd завершился с ненулевым кодом.
type Error struct {
	Args   []string // подкоманда и её аргументы, без --readonly и --db
	Stderr string
	Code   int
}

func (e *Error) Error() string {
	return "bd " + strings.Join(e.Args, " ") + ": " + e.FirstLine()
}

// FirstLine — первая непустая строка stderr, иначе "exit status N".
func (e *Error) FirstLine() string {
	for _, l := range strings.Split(e.Stderr, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return "exit status " + strconv.Itoa(e.Code)
}

// Client запускает bd. Bin == "" означает "bd" из PATH; DB — путь к .beads (необязателен).
// Клиент из NewClient учитывает идущие записи (AddComment, Init, SetConfig, ConfigUnset), чтобы Wait дождался
// их завершения (и удаления временных файлов) перед выходом; литерал Client{} их не учитывает.
type Client struct {
	Bin string
	DB  string
	// writes — идущие записи. Указатель: Client передаётся по значению.
	writes *inflight
}

// inflight — счётчик идущих записей под мьютексом. sync.WaitGroup не годится: Add
// идёт в горутине команды Bubble Tea, Wait — в main, и без упорядочивания между ними
// Add при нулевом счётчике одновременно с Wait — гонка (и ошибка по правилам WaitGroup).
type inflight struct {
	mu   sync.Mutex
	n    int
	idle chan struct{} // закрывается, когда n падает до нуля; новый — при переходе 0 → 1
}

func (f *inflight) add() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.n == 0 {
		f.idle = make(chan struct{})
	}
	f.n++
}

func (f *inflight) done() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n--
	if f.n == 0 {
		close(f.idle)
	}
}

// track отмечает начало записи и возвращает функцию её завершения; без учёта — пустую.
func (c Client) track() (done func()) {
	if c.writes == nil {
		return func() {}
	}
	c.writes.add()
	return c.writes.done
}

// NewClient — клиент с учётом идущих записей (см. Wait).
func NewClient(bin, db string) *Client {
	return &Client{Bin: bin, DB: db, writes: &inflight{}}
}

// Wait ждёт завершения идущих записей (AddComment, Init, SetConfig, ConfigUnset), но не дольше timeout.
// Вызывается после выхода из TUI: корневой контекст уже отменён, bd прерван, и отложенное
// удаление временного файла успевает отработать до выхода процесса.
func (c *Client) Wait(timeout time.Duration) {
	if c.writes == nil {
		return
	}
	c.writes.mu.Lock()
	if c.writes.n == 0 {
		c.writes.mu.Unlock()
		return
	}
	idle := c.writes.idle
	c.writes.mu.Unlock()
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case <-idle:
	case <-t.C:
	}
}

// args собирает полный список аргументов: всегда --readonly, затем --db, затем подкоманда.
func (c Client) args(sub ...string) []string {
	a := []string{"--readonly"}
	if c.DB != "" {
		a = append(a, "--db", c.DB)
	}
	return append(a, sub...)
}

// writeArgs собирает аргументы записи (comments add, config set, config unset): без --readonly, затем --db
// и подкоманда. Все чтения идут через args; bd init — без --db (см. Init).
func (c Client) writeArgs(sub ...string) []string {
	var a []string
	if c.DB != "" {
		a = append(a, "--db", c.DB)
	}
	return append(a, sub...)
}

// waitDelay — сколько Run ждёт закрытия труб после смерти bd или отмены контекста.
const waitDelay = 500 * time.Millisecond

// run запускает bd только для чтения (с --readonly) и возвращает stdout;
// при ненулевом коде — *Error со stderr.
func (c Client) run(ctx context.Context, sub ...string) ([]byte, error) {
	return c.exec(ctx, c.args(sub...), sub)
}

// exec запускает bd с полным списком аргументов argv; sub — подкоманда для текста ошибок.
func (c Client) exec(ctx context.Context, argv, sub []string) ([]byte, error) {
	var stdout bytes.Buffer
	if err := c.start(ctx, argv, sub, &stdout, nil, false); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// start запускает bd: stdout идёт в stdout, stderr копится для *Error и, если tee
// не nil, дублируется в него. stdin пустой (/dev/null). interrupt: при отмене контекста
// bd получает SIGINT, а не SIGKILL, и успевает прибраться; не вышел за WaitDelay — убивается.
func (c Client) start(ctx context.Context, argv, sub []string, stdout, tee io.Writer, interrupt bool) error {
	bin := c.Bin
	if bin == "" {
		bin = "bd"
	}
	cmd := exec.CommandContext(ctx, bin, argv...)
	// Убитый по контексту bd может оставить дочерний процесс с нашим stdout: без
	// WaitDelay Run ждал бы, пока сирота закроет трубу, и таймаут не срабатывал бы.
	cmd.WaitDelay = waitDelay
	if interrupt {
		cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	}
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = stdout, &stderr
	if tee != nil {
		cmd.Stderr = io.MultiWriter(&stderr, tee)
	}
	if err := cmd.Run(); err != nil {
		// Процесс убит контекстом (таймаут или выход из bdv): это не ошибка bd,
		// и код выхода -1 со пустым stderr не должен выглядеть как *Error.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("bd %s: %w", strings.Join(sub, " "), ctxErr)
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return &Error{Args: sub, Stderr: stderr.String(), Code: ee.ExitCode()}
		}
		return fmt.Errorf("bd %s: %w", strings.Join(sub, " "), err)
	}
	return nil
}

// lockedBuffer — буфер, в который пишут сразу stdout и stderr: os/exec копирует их
// разными горутинами, поэтому запись под мьютексом.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// decode запускает bd и разбирает JSON-ответ в v.
func (c Client) decode(ctx context.Context, v any, sub ...string) error {
	out, err := c.run(ctx, sub...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(out, v); err != nil {
		return fmt.Errorf("bd %s: parse JSON: %w", strings.Join(sub, " "), err)
	}
	return nil
}

// List возвращает все задачи, включая закрытые. Без --flat bd игнорирует --json.
func (c Client) List(ctx context.Context) ([]Issue, error) {
	var issues []Issue
	if err := c.decode(ctx, &issues, "list", "--flat", "--json", "--all", "-n", "0"); err != nil {
		return nil, err
	}
	return issues, nil
}

// Show возвращает связи одной задачи (bd show отдаёт массив из одного элемента).
func (c Client) Show(ctx context.Context, id string) (Detail, error) {
	var items []Detail
	if err := c.decode(ctx, &items, "show", id, "--json"); err != nil {
		return Detail{}, err
	}
	if len(items) == 0 {
		return Detail{}, fmt.Errorf("bd show %s: empty response", id)
	}
	return items[0], nil
}

// Comments возвращает комментарии задачи.
func (c Client) Comments(ctx context.Context, id string) ([]Comment, error) {
	var comments []Comment
	if err := c.decode(ctx, &comments, "comments", id, "--json"); err != nil {
		return nil, err
	}
	return comments, nil
}

// AddComment добавляет комментарий text к задаче id — запись, поэтому без --readonly.
// Текст идёт через временный файл (-f): кавычки, длина и текст, начинающийся с «-»,
// не ломают аргументы. Файл удаляется в любом случае.
func (c Client) AddComment(ctx context.Context, id, text string) error {
	// Контекст уже отменён (выход из bdv): ни файла, ни bd, ни Add — иначе Add мог бы
	// прийти одновременно с Wait, который уже ждёт после отмены.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("bd comments add %s: %w", id, err)
	}
	// Учёт — первым делом, до любой работы; завершение — после удаления файла:
	// отложенные вызовы идут в обратном порядке.
	defer c.track()()
	f, err := os.CreateTemp("", "bdv-comment-*.txt")
	if err != nil {
		return fmt.Errorf("bd comments add %s: %w", id, err)
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = f.WriteString(text)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("bd comments add %s: %w", id, err)
	}
	sub := []string{"comments", "add", id, "-f", name}
	_, err = c.exec(ctx, c.writeArgs(sub...), sub)
	return err
}

// SetConfig задаёт настройку bd: «bd [--db P] config set <key> <value>» — запись, поэтому
// без --readonly. Последовательные ID после bd init (issue_id_mode counter) и экран настроек.
// Учитывается в Wait, как AddComment и Init.
func (c Client) SetConfig(ctx context.Context, key, value string) error {
	// Контекст уже отменён (выход из bdv): bd не запускается и Add не делается, как в AddComment.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("bd config set %s: %w", key, err)
	}
	defer c.track()()
	sub := []string{"config", "set", key, value}
	return c.interruptible(ctx, sub)
}

// interruptible запускает запись sub так же, как bd init: при отмене контекста bd получает
// SIGINT (через WaitDelay — SIGKILL) и успевает прибраться. stdout не нужен.
func (c Client) interruptible(ctx context.Context, sub []string) error {
	var stdout bytes.Buffer
	return c.start(ctx, c.writeArgs(sub...), sub, &stdout, nil, true)
}

// ConfigUnset сбрасывает настройку bd к умолчанию: «bd [--db P] config unset <key>» —
// запись, поэтому без --readonly. Учитывается в Wait, как SetConfig.
func (c Client) ConfigUnset(ctx context.Context, key string) error {
	// Контекст уже отменён (выход из bdv): bd не запускается и Add не делается, как в AddComment.
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("bd config unset %s: %w", key, err)
	}
	defer c.track()()
	sub := []string{"config", "unset", key}
	return c.interruptible(ctx, sub)
}

// ConfigList возвращает настройки из базы (Dolt): «bd --readonly config list --json» —
// плоский объект строк. YAML-ключей (.beads/config.yaml) и незаданных ключей в нём нет.
// Значение не строкой берётся текстом JSON как есть.
func (c Client) ConfigList(ctx context.Context) (map[string]string, error) {
	var raw map[string]json.RawMessage
	if err := c.decode(ctx, &raw, "config", "list", "--json"); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			out[k] = s
		} else {
			out[k] = string(v)
		}
	}
	return out, nil
}

// ConfigGet возвращает действующее значение ключа: «bd --readonly config get <key> --json»
// → {"key","value","location"}. location ("config.yaml") есть только у YAML-ключей.
// Незаданный (и неизвестный) ключ bd 0.59 отдаёт с пустым value и кодом 0.
func (c Client) ConfigGet(ctx context.Context, key string) (value, location string, err error) {
	var r struct {
		Value    string `json:"value"`
		Location string `json:"location"`
	}
	if err := c.decode(ctx, &r, "config", "get", key, "--json"); err != nil {
		return "", "", err
	}
	return r.Value, r.Location, nil
}

// Types возвращает встроенные типы задач: «bd --readonly types --json» → core_types.
func (c Client) Types(ctx context.Context) ([]TypeInfo, error) {
	var r struct {
		Core []TypeInfo `json:"core_types"`
	}
	if err := c.decode(ctx, &r, "types", "--json"); err != nil {
		return nil, err
	}
	return r.Core, nil
}

// Memories возвращает постоянную память bd: «bd --readonly memories --json». В bd 0.59
// это объект «ключ → текст» ({} — памяти нет); результат упорядочен по ключу. Значение
// не строкой берётся текстом JSON как есть.
func (c Client) Memories(ctx context.Context) ([]Memory, error) {
	var raw map[string]json.RawMessage
	if err := c.decode(ctx, &raw, "memories", "--json"); err != nil {
		return nil, err
	}
	out := make([]Memory, 0, len(raw))
	for k, v := range raw {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			s = string(v)
		}
		out = append(out, Memory{Key: k, Text: s})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Formulas возвращает формулы: «bd --readonly formula list --json». Без формул bd 0.59
// печатает null — это пустой список. Форма непустого ответа на настоящем bd не проверена;
// берутся только name, type, description и source, прочие поля игнорируются.
func (c Client) Formulas(ctx context.Context) ([]Formula, error) {
	var out []Formula
	if err := c.decode(ctx, &out, "formula", "list", "--json"); err != nil {
		return nil, err
	}
	return out, nil
}

// Templates возвращает шаблоны: «bd --readonly list --include-templates --flat --json
// --all -n 0», из которого остаются эпики с меткой template и шаблоны-молекулы
// (is_template). Обычный List шаблонов-молекул не отдаёт: bd скрывает их без флага.
func (c Client) Templates(ctx context.Context) ([]Issue, error) {
	var all []Issue
	if err := c.decode(ctx, &all, "list", "--include-templates", "--flat", "--json", "--all", "-n", "0"); err != nil {
		return nil, err
	}
	var out []Issue
	for _, is := range all {
		if IsTemplate(is) {
			out = append(out, is)
		}
	}
	return out, nil
}

// IsTemplate — задача-шаблон: эпик с меткой template или шаблон-молекула bd (is_template).
// Такие эпики не спеки (вкладка Specs), а шаблоны (вкладка Formulas).
func IsTemplate(is Issue) bool {
	if is.IsTemplate {
		return true
	}
	return is.IssueType == "epic" && slices.Contains(is.Labels, "template")
}

// Init создаёт базу beads командой «bd init» в текущей папке процесса — вторая
// запись bdv (третья — SetConfig). Без --readonly, без --db (Init вызывается, только когда база
// не задана) и без флагов: префикс и хуки — по умолчанию bd. stdin пустой. Отмена
// контекста шлёт bd SIGINT (через WaitDelay — SIGKILL).
// Возвращает stdout и stderr вместе в порядке записи — и при ошибке тоже.
func (c Client) Init(ctx context.Context) (string, error) {
	// Контекст уже отменён (выход из bdv): bd не запускается и Add не делается, как в AddComment.
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("bd init: %w", err)
	}
	defer c.track()()
	sub := []string{"init"}
	var out lockedBuffer
	// Отмена (Ctrl+C, таймаут) — SIGINT: bd init успевает прибраться за собой.
	err := c.start(ctx, sub, sub, &out, &out, true)
	return out.String(), err
}
