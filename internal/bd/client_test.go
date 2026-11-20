package bd_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/testbd"
)

func TestListArgsAndParse(t *testing.T) {
	f := testbd.New(t)
	f.SetList(`[{"id":"p-1","title":"Эпик","status":"open","priority":1,"issue_type":"epic",
      "created_at":"2026-09-23T08:48:29Z","updated_at":"2026-09-23T09:27:18Z","comment_count":2,
      "dependencies":[{"issue_id":"p-1","depends_on_id":"p-0","type":"blocks"}],"labels":["ui"],"unknown_field":1}]`)

	got, err := bd.Client{DB: "/x/.beads"}.List(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	i := got[0]
	if i.Title != "Эпик" || i.IssueType != "epic" || i.CommentCount != 2 || i.Priority != 1 {
		t.Errorf("issue: %+v", i)
	}
	if i.Dependencies[0].DependsOnID != "p-0" || i.ClosedAt != nil || i.CreatedAt.Year() != 2026 {
		t.Errorf("issue: %+v", i)
	}
	want := []string{"--readonly", "--db", "/x/.beads", "list", "--flat", "--json", "--all", "-n", "0"}
	if c := f.Calls(); len(c) != 1 || !reflect.DeepEqual(c[0], want) {
		t.Errorf("calls: %q", c)
	}
}

func TestListWithoutDBOmitsFlag(t *testing.T) {
	f := testbd.New(t)

	if _, err := (bd.Client{}).List(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"--readonly", "list", "--flat", "--json", "--all", "-n", "0"}
	if c := f.Calls(); !reflect.DeepEqual(c[0], want) {
		t.Errorf("calls: %q", c)
	}
}

func TestShowParsesDependents(t *testing.T) {
	f := testbd.New(t)
	f.SetShow("p-1", `[{"id":"p-1","dependents":[{"id":"p-2","title":"T","status":"open","priority":2,"dependency_type":"blocks"}]}]`)

	d, err := bd.Client{}.Show(context.Background(), "p-1")

	if err != nil {
		t.Fatal(err)
	}
	if len(d.Dependents) != 1 || d.Dependents[0].DependencyType != "blocks" || d.Dependents[0].ID != "p-2" {
		t.Errorf("detail: %+v", d)
	}
	if want := []string{"--readonly", "show", "p-1", "--json"}; !reflect.DeepEqual(f.Calls()[0], want) {
		t.Errorf("calls: %q", f.Calls())
	}
}

// Без файла ответа фейк отдаёт [{"id":...}]: Show успешен, Detail пуст.
func TestShowDefaultIsEmptyDetail(t *testing.T) {
	testbd.New(t)

	d, err := bd.Client{}.Show(context.Background(), "p-9")

	if err != nil {
		t.Fatal(err)
	}
	if len(d.Dependencies) != 0 || len(d.Dependents) != 0 {
		t.Errorf("detail: %+v", d)
	}
}

func TestShowEmptyArrayIsError(t *testing.T) {
	f := testbd.New(t)
	f.SetShow("p-1", `[]`)

	_, err := bd.Client{}.Show(context.Background(), "p-1")

	if err == nil || !strings.Contains(err.Error(), "bd show p-1: empty response") {
		t.Errorf("err: %v", err)
	}
}

func TestCommentsParse(t *testing.T) {
	f := testbd.New(t)
	f.SetComments("p-1", `[{"id":1,"issue_id":"p-1","author":"A","text":"hi","created_at":"2026-09-23T08:48:29Z"}]`)

	got, err := bd.Client{DB: "/d"}.Comments(context.Background(), "p-1")

	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Author != "A" || got[0].Text != "hi" || got[0].ID != 1 || got[0].CreatedAt.Year() != 2026 {
		t.Errorf("comments: %+v", got)
	}
	if want := []string{"--readonly", "--db", "/d", "comments", "p-1", "--json"}; !reflect.DeepEqual(f.Calls()[0], want) {
		t.Errorf("calls: %q", f.Calls())
	}
}

func TestErrorCarriesStderr(t *testing.T) {
	f := testbd.New(t)
	f.Fail("list", "\nError: failed to open database: database \"x\" not found\nmore", 1)

	_, err := bd.Client{}.List(context.Background())

	var be *bd.Error
	if !errors.As(err, &be) {
		t.Fatalf("err = %v", err)
	}
	if be.Code != 1 {
		t.Errorf("code = %d", be.Code)
	}
	want := `Error: failed to open database: database "x" not found`
	if be.FirstLine() != want {
		t.Errorf("FirstLine = %q", be.FirstLine())
	}
	if !strings.HasPrefix(err.Error(), "bd list ") || !strings.HasSuffix(err.Error(), want) {
		t.Errorf("Error() = %q", err.Error())
	}
}

func TestFirstLineFallsBackToExitStatus(t *testing.T) {
	e := &bd.Error{Args: []string{"list"}, Stderr: " \n", Code: 3}
	if e.FirstLine() != "exit status 3" {
		t.Errorf("FirstLine = %q", e.FirstLine())
	}
}

func TestFailCanBeCleared(t *testing.T) {
	f := testbd.New(t)
	f.Fail("list", "boom", 2)
	f.Fail("list", "", 0)

	if _, err := (bd.Client{}).List(context.Background()); err != nil {
		t.Errorf("err: %v", err)
	}
}

func TestBadJSONIsError(t *testing.T) {
	f := testbd.New(t)
	f.SetList("not json")

	_, err := bd.Client{}.List(context.Background())

	if err == nil || !strings.Contains(err.Error(), "bd list") || !strings.Contains(err.Error(), ": parse JSON: ") {
		t.Errorf("err: %v", err)
	}
}

func TestMissingBinaryIsError(t *testing.T) {
	if _, err := (bd.Client{Bin: "/nonexistent/bd"}).List(context.Background()); err == nil {
		t.Error("ожидалась ошибка")
	}
}

func TestCountAndPbcopy(t *testing.T) {
	f := testbd.New(t)
	c := bd.Client{}
	c.List(context.Background())
	c.List(context.Background())
	c.Show(context.Background(), "a")
	if f.Count("list") != 2 || f.Count("show") != 1 || f.Count("comments") != 0 {
		t.Errorf("calls: %q", f.Calls())
	}
	out := f.SetPbcopy()
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader("hello")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != "hello" {
		t.Errorf("pbcopy.out = %q", b)
	}
}

// Зависший bd обрывается контекстом: ошибка говорит о таймауте и не выдаётся за *Error.
func TestTimeoutIsNotBdError(t *testing.T) {
	f := testbd.New(t)
	f.Sleep("list", 2)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := bd.Client{}.List(ctx)

	if d := time.Since(start); d > time.Second {
		t.Errorf("List ждал %v", d)
	}
	var be *bd.Error
	if errors.As(err, &be) {
		t.Fatalf("err — *bd.Error: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("err = %v, want таймаут", err)
	}
}

func TestSleepWithoutTimeoutStillAnswers(t *testing.T) {
	f := testbd.New(t)
	f.SetList(`[{"id":"a"}]`)
	f.Sleep("list", 0.05)

	got, err := bd.Client{}.List(context.Background())

	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, err %v", got, err)
	}
}

// Зависший bd, чей дочерний процесс держит stdout открытым: после отмены контекста
// Run не ждёт сироту, а возвращается за WaitDelay. Отмена — только когда дочерний
// sleep уже запущен, иначе убийство sh опередило бы его и проверка ничего бы не значила.
func TestCancelWithOrphanHoldingStdout(t *testing.T) {
	dir := t.TempDir()
	bin, started := filepath.Join(dir, "bd"), filepath.Join(dir, "started")
	script := "#!/bin/sh\n(touch '" + started + "'; sleep 5)\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	at := make(chan time.Time, 1)
	go func() {
		for ctx.Err() == nil {
			if _, err := os.Stat(started); err == nil {
				at <- time.Now()
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	_, err := bd.Client{Bin: bin}.List(ctx)

	select {
	case c := <-at:
		if d := time.Since(c); d > time.Second {
			t.Errorf("после отмены List ждал %v", d)
		}
	default:
		t.Fatalf("дочерний процесс не запустился, err = %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want отмену", err)
	}
}

// commentFile — путь из аргумента -f вызова bd comments add.
func commentFile(t *testing.T, call []string) string {
	t.Helper()
	for i, a := range call {
		if a == "-f" && i+1 < len(call) {
			return call[i+1]
		}
	}
	t.Fatalf("в вызове нет -f: %q", call)
	return ""
}

// Комментарий — единственная запись: без --readonly, с --db, текст через временный
// файл -f, который после вызова удалён.
func TestAddCommentArgs(t *testing.T) {
	f := testbd.New(t)

	err := bd.Client{DB: "/x/.beads"}.AddComment(context.Background(), "p-1", "Привет\nвторая строка")

	if err != nil {
		t.Fatal(err)
	}
	calls := f.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls: %q", calls)
	}
	c := calls[0]
	path := commentFile(t, c)
	want := []string{"--db", "/x/.beads", "comments", "add", "p-1", "-f", path}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("call = %q, want %q", c, want)
	}
	for _, a := range c {
		if a == "--readonly" {
			t.Errorf("comments add с --readonly: %q", c)
		}
	}
	if !strings.HasPrefix(filepath.Base(path), "bdv-comment-") || !strings.HasSuffix(path, ".txt") {
		t.Errorf("имя временного файла: %q", path)
	}
	adds := f.CommentAdds()
	if len(adds) != 1 || adds[0].ID != "p-1" || adds[0].Text != "Привет\nвторая строка" {
		t.Errorf("CommentAdds = %+v", adds)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("временный файл не удалён: %v", err)
	}
}

// Без DB флага --db нет, --readonly тоже нет.
func TestAddCommentWithoutDB(t *testing.T) {
	f := testbd.New(t)

	if err := (bd.Client{}).AddComment(context.Background(), "p-1", "x"); err != nil {
		t.Fatal(err)
	}

	c := f.Calls()[0]
	if want := []string{"comments", "add", "p-1", "-f", commentFile(t, c)}; !reflect.DeepEqual(c, want) {
		t.Errorf("call = %q, want %q", c, want)
	}
}

// Текст, начинающийся с «-», не превращается во флаг: он идёт файлом.
func TestAddCommentTextStartingWithDash(t *testing.T) {
	f := testbd.New(t)
	text := "--readonly -f x \"кавычки\" 'и' $HOME `cmd`"

	if err := (bd.Client{}).AddComment(context.Background(), "p-1", text); err != nil {
		t.Fatal(err)
	}

	c := f.Calls()[0]
	for _, a := range c {
		if a == text || a == "--readonly" {
			t.Errorf("текст попал в аргументы: %q", c)
		}
	}
	if adds := f.CommentAdds(); len(adds) != 1 || adds[0].Text != text {
		t.Errorf("CommentAdds = %+v", adds)
	}
}

// Ошибка bd — *Error со stderr; временный файл удалён и при ошибке.
func TestAddCommentError(t *testing.T) {
	f := testbd.New(t)
	f.Fail("comments-add", "Error: issue p-1 not found\nmore", 1)

	err := bd.Client{}.AddComment(context.Background(), "p-1", "x")

	var be *bd.Error
	if !errors.As(err, &be) {
		t.Fatalf("err = %v", err)
	}
	if be.FirstLine() != "Error: issue p-1 not found" || be.Code != 1 {
		t.Errorf("FirstLine = %q, code %d", be.FirstLine(), be.Code)
	}
	if _, err := os.Stat(commentFile(t, f.Calls()[0])); !os.IsNotExist(err) {
		t.Errorf("временный файл не удалён: %v", err)
	}
}

// Таймаут: вызов обрывается, ошибка — таймаут, временный файл удалён.
func TestAddCommentTimeout(t *testing.T) {
	f := testbd.New(t)
	f.Sleep("comments-add", 5)
	// Запас на старт sh: фейк должен успеть записать вызов до таймаута.
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := bd.Client{}.AddComment(ctx, "p-1", "x")

	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("AddComment ждал %v", d)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want таймаут", err)
	}
	if adds := f.CommentAdds(); len(adds) != 1 || adds[0].Text != "x" {
		t.Errorf("CommentAdds = %+v", adds)
	}
	if _, err := os.Stat(commentFile(t, f.Calls()[0])); !os.IsNotExist(err) {
		t.Errorf("временный файл не удалён: %v", err)
	}
}

// Count различает bd comments и bd comments add.
func TestCountSeparatesCommentAdds(t *testing.T) {
	f := testbd.New(t)
	c := bd.Client{}
	c.Comments(context.Background(), "a")
	c.AddComment(context.Background(), "a", "x")
	if f.Count("comments") != 1 || f.Count("comments-add") != 1 {
		t.Errorf("calls: %q", f.Calls())
	}
}

// Ctrl+C во время отправки: корневой контекст отменён, bdv выходит. Wait дожидается
// идущих AddComment, и временный файл удаляется до выхода процесса.
func TestWaitRemovesTempFileAfterCancel(t *testing.T) {
	f := testbd.New(t)
	f.Sleep("comments-add", 5)
	c := bd.NewClient("", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.AddComment(ctx, "p-1", "x") }()
	// Фейк записал вызов и спит: файл комментария создан и ещё не удалён.
	deadline := time.Now().Add(5 * time.Second)
	for len(f.CommentAdds()) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("фейк не получил вызов")
		}
		time.Sleep(10 * time.Millisecond)
	}
	path := commentFile(t, f.Calls()[0])

	cancel()
	start := time.Now()
	c.Wait(3 * time.Second)

	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Wait ждал %v", d)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("после Wait временный файл на месте: %v", err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want отмену", err)
	}
}

// Wait не ждёт дольше timeout, даже если bd ещё работает; без идущих вызовов
// и у клиента без учёта (bd.Client{}) возвращается сразу.
func TestWaitTimeout(t *testing.T) {
	f := testbd.New(t)
	f.Sleep("comments-add", 5)
	c := bd.NewClient("", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.AddComment(ctx, "p-1", "x")
	for len(f.CommentAdds()) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	start := time.Now()
	c.Wait(200 * time.Millisecond)
	if d := time.Since(start); d < 150*time.Millisecond || d > time.Second {
		t.Errorf("Wait с идущим вызовом ждал %v, want ~200ms", d)
	}
	cancel()
	c.Wait(3 * time.Second)

	start = time.Now()
	c.Wait(time.Second)
	(&bd.Client{}).Wait(time.Second)
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("Wait без вызовов ждал %v", d)
	}
}

// Уже отменённый контекст: AddComment сразу возвращает его ошибку — без временного
// файла (TMPDIR недоступен, CreateTemp упал бы) и без вызова bd.
func TestAddCommentCancelledBeforeStart(t *testing.T) {
	f := testbd.New(t)
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "нет-такого"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := bd.NewClient("", "")
	err := c.AddComment(ctx, "p-1", "x")

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want отмену", err)
	}
	if calls := f.Calls(); len(calls) != 0 {
		t.Errorf("bd вызван: %q", calls)
	}
	start := time.Now()
	c.Wait(time.Second)
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("Wait ждал %v", d)
	}
}

// bd init — вторая запись bdv: ровно «init» без --readonly и без --db (даже если DB
// задан), stdin пустой, вывод — stdout и stderr вместе.
func TestInitArgs(t *testing.T) {
	f := testbd.New(t)
	f.SetInitOutput("Created .beads\nprefix: proj\n", "warning: hooks\n")

	out, err := bd.NewClient("", "/x/.beads").Init(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	calls := f.Calls()
	if len(calls) != 1 || !reflect.DeepEqual(calls[0], []string{"init"}) {
		t.Fatalf("calls: %q, want [init]", calls)
	}
	if f.Count("init") != 1 {
		t.Errorf("Count(init) = %d", f.Count("init"))
	}
	for _, s := range []string{"Created .beads", "prefix: proj", "warning: hooks"} {
		if !strings.Contains(out, s) {
			t.Errorf("вывод %q не содержит %q", out, s)
		}
	}
	if stdin := f.InitStdin(); stdin != "" {
		t.Errorf("stdin bd init = %q, want пусто", stdin)
	}
}

// Ошибка bd init — *Error со stderr; вывод возвращается и при ошибке.
func TestInitError(t *testing.T) {
	f := testbd.New(t)
	f.Fail("init", "Error: dolt server unreachable\nmore", 1)

	out, err := bd.Client{}.Init(context.Background())

	var be *bd.Error
	if !errors.As(err, &be) {
		t.Fatalf("err = %v", err)
	}
	if be.FirstLine() != "Error: dolt server unreachable" || be.Code != 1 || !reflect.DeepEqual(be.Args, []string{"init"}) {
		t.Errorf("FirstLine = %q, code %d, args %q", be.FirstLine(), be.Code, be.Args)
	}
	if !strings.Contains(out, "dolt server unreachable") || !strings.Contains(out, "more") {
		t.Errorf("out = %q", out)
	}
}

// Таймаут: вызов обрывается, ошибка — таймаут, а не *Error; Wait учитывает идущий Init.
func TestInitTimeout(t *testing.T) {
	f := testbd.New(t)
	f.Sleep("init", 5)
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	c := bd.NewClient("", "")

	start := time.Now()
	_, err := c.Init(ctx)

	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Init ждал %v", d)
	}
	var be *bd.Error
	if errors.As(err, &be) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want таймаут", err)
	}
	if f.Count("init") != 1 {
		t.Errorf("calls: %q", f.Calls())
	}
	start = time.Now()
	c.Wait(time.Second)
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("Wait после Init ждал %v", d)
	}
}

// Идущий Init учитывается в Wait так же, как AddComment.
func TestWaitCoversInit(t *testing.T) {
	f := testbd.New(t)
	f.Sleep("init", 5)
	c := bd.NewClient("", "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Init(ctx)
	for f.Count("init") == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	start := time.Now()
	c.Wait(200 * time.Millisecond)
	if d := time.Since(start); d < 150*time.Millisecond {
		t.Errorf("Wait с идущим Init ждал %v, want ~200ms", d)
	}
	cancel()
	c.Wait(3 * time.Second)
}

// Отмена bd init (Ctrl+C, таймаут) шлёт SIGINT, а не SIGKILL: bd успевает прибраться.
// Скрипт ловит INT и пишет метку; sleep в фоне, чтобы wait прервался сигналом сразу.
func TestInitInterruptsOnCancel(t *testing.T) {
	dir := t.TempDir()
	bin, started, got := filepath.Join(dir, "bd"), filepath.Join(dir, "started"), filepath.Join(dir, "got-int")
	script := "#!/bin/sh\ntrap 'touch \"" + got + "\"; exit 130' INT\n" +
		"sleep 5 </dev/null >/dev/null 2>&1 &\ntouch '" + started + "'\nwait $!\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for ctx.Err() == nil {
			if _, err := os.Stat(started); err == nil {
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	start := time.Now()
	_, err := bd.Client{Bin: bin}.Init(ctx)

	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("Init после отмены ждал %v", d)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want отмену", err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("bd init не получил SIGINT: %v", err)
	}
}

// bd config set — третья запись bdv: без --readonly, с --db, если задан; ровно
// «config set <key> <value>».
func TestSetConfigArgs(t *testing.T) {
	f := testbd.New(t)
	if err := bd.NewClient("", "/x/.beads").SetConfig(context.Background(), "issue_id_mode", "counter"); err != nil {
		t.Fatal(err)
	}
	if err := (bd.Client{}).SetConfig(context.Background(), "issue_id_mode", "counter"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"--db", "/x/.beads", "config", "set", "issue_id_mode", "counter"},
		{"config", "set", "issue_id_mode", "counter"},
	}
	if got := f.Calls(); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %q, want %q", got, want)
	}
	if f.Count("config-set") != 2 {
		t.Errorf("Count(config-set) = %d", f.Count("config-set"))
	}
}

// Ошибка bd config set — *Error со stderr; отменённый контекст — bd не запускается.
func TestSetConfigErrorAndCancelled(t *testing.T) {
	f := testbd.New(t)
	f.Fail("config-set", "Error: unknown key\n", 1)
	err := bd.NewClient("", "").SetConfig(context.Background(), "k", "v")
	var be *bd.Error
	if !errors.As(err, &be) || be.FirstLine() != "Error: unknown key" ||
		!reflect.DeepEqual(be.Args, []string{"config", "set", "k", "v"}) {
		t.Fatalf("err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := bd.NewClient("", "").SetConfig(ctx, "k", "v"); !errors.Is(err, context.Canceled) {
		t.Fatalf("отменённый контекст: err = %v", err)
	}
	if n := len(f.Calls()); n != 1 {
		t.Fatalf("вызовов %d, want 1: %q", n, f.Calls())
	}
}

// Wait дожидается идущего SetConfig: это запись.
func TestSetConfigTrackedByWait(t *testing.T) {
	f := testbd.New(t)
	f.Sleep("config-set", 0.5)
	c := bd.NewClient("", "")
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.SetConfig(context.Background(), "k", "v")
	}()
	// Ждём, пока фейк запишет вызов: SetConfig уже учтён.
	for i := 0; i < 200 && len(f.Calls()) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	start := time.Now()
	c.Wait(5 * time.Second)
	if d := time.Since(start); d < 200*time.Millisecond {
		t.Errorf("Wait вернулся через %v, не дождавшись SetConfig", d)
	}
	<-done
}

// Ответы настоящего bd 0.59 (тестовый проект, --readonly): config list — плоский объект ключей
// базы, значения строками; config get — key/value и location только у YAML-ключей.
const (
	realConfigList = `{
  "auto_compact_enabled": "false",
  "compact_batch_size": "50",
  "compact_tier1_days": "30",
  "compaction_enabled": "false",
  "issue_prefix": "demo-app",
  "schema_version": "6"
}`
	realGetYAML = `{
  "key": "create.require-description",
  "location": "config.yaml",
  "value": "false"
}`
	realGetUnset = `{
  "key": "issue_id_mode",
  "value": ""
}`
	realTypes = `{"core_types":[{"name":"task","description":"General work item (default)"},
{"name":"bug","description":"Bug report or defect"},{"name":"decision","description":"Architecture decision record (ADR)"}]}`
)

// bd config list --json — чтение с --readonly; ответ — карта строк.
func TestConfigListArgsAndParse(t *testing.T) {
	f := testbd.New(t)
	f.SetConfigList(realConfigList)
	got, err := bd.Client{DB: "/x/.beads"}.ConfigList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got["issue_prefix"] != "demo-app" || got["compact_tier1_days"] != "30" || len(got) != 6 {
		t.Fatalf("config list = %v", got)
	}
	want := []string{"--readonly", "--db", "/x/.beads", "config", "list", "--json"}
	if c := f.Calls(); len(c) != 1 || !reflect.DeepEqual(c[0], want) {
		t.Fatalf("calls: %q", c)
	}
}

// Значение не строкой (на будущее) не ломает разбор: берётся JSON как есть.
func TestConfigListNonStringValue(t *testing.T) {
	f := testbd.New(t)
	f.SetConfigList(`{"a":"x","n":5,"b":true}`)
	got, err := bd.Client{}.ConfigList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != "x" || got["n"] != "5" || got["b"] != "true" {
		t.Fatalf("config list = %v", got)
	}
}

// bd config get <key> --json — чтение с --readonly: значение и location (у YAML-ключа).
func TestConfigGetArgsAndParse(t *testing.T) {
	f := testbd.New(t)
	f.SetConfigGet("create.require-description", realGetYAML)
	f.SetConfigGet("issue_id_mode", realGetUnset)
	c := bd.Client{DB: "/x/.beads"}
	v, loc, err := c.ConfigGet(context.Background(), "create.require-description")
	if err != nil || v != "false" || loc != "config.yaml" {
		t.Fatalf("get yaml = %q %q %v", v, loc, err)
	}
	v, loc, err = c.ConfigGet(context.Background(), "issue_id_mode")
	if err != nil || v != "" || loc != "" {
		t.Fatalf("get unset = %q %q %v", v, loc, err)
	}
	want := []string{"--readonly", "--db", "/x/.beads", "config", "get", "create.require-description", "--json"}
	if calls := f.Calls(); len(calls) != 2 || !reflect.DeepEqual(calls[0], want) {
		t.Fatalf("calls: %q", calls)
	}
}

// Ошибка get одного ключа — *Error; остальные ключи отвечают.
func TestConfigGetError(t *testing.T) {
	f := testbd.New(t)
	f.FailID("config-get", "bad.key", "Error: unknown key\n", 1)
	_, _, err := bd.Client{}.ConfigGet(context.Background(), "bad.key")
	var be *bd.Error
	if !errors.As(err, &be) || be.FirstLine() != "Error: unknown key" {
		t.Fatalf("err = %v", err)
	}
	if v, _, err := (bd.Client{}).ConfigGet(context.Background(), "good.key"); err != nil || v != "" {
		t.Fatalf("good.key: %q %v", v, err)
	}
}

// bd config unset — запись: без --readonly, с --db, учитывается в Wait.
func TestConfigUnsetArgsAndWait(t *testing.T) {
	f := testbd.New(t)
	if err := bd.NewClient("", "/x/.beads").ConfigUnset(context.Background(), "issue_id_mode"); err != nil {
		t.Fatal(err)
	}
	want := []string{"--db", "/x/.beads", "config", "unset", "issue_id_mode"}
	if c := f.Calls(); len(c) != 1 || !reflect.DeepEqual(c[0], want) {
		t.Fatalf("calls: %q", c)
	}
	if f.Count("config-unset") != 1 {
		t.Fatalf("Count(config-unset) = %d", f.Count("config-unset"))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := bd.NewClient("", "").ConfigUnset(ctx, "k"); !errors.Is(err, context.Canceled) {
		t.Fatalf("отменённый контекст: %v", err)
	}
	f.Sleep("config-unset", 0.5)
	c := bd.NewClient("", "")
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.ConfigUnset(context.Background(), "k")
	}()
	for i := 0; i < 200 && len(f.Calls()) < 2; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	start := time.Now()
	c.Wait(5 * time.Second)
	if d := time.Since(start); d < 200*time.Millisecond {
		t.Errorf("Wait вернулся через %v, не дождавшись ConfigUnset", d)
	}
	<-done
}

// bd types --json — чтение с --readonly; встроенные типы из core_types по порядку.
func TestTypesParse(t *testing.T) {
	f := testbd.New(t)
	f.SetTypes(realTypes)
	got, err := bd.Client{}.Types(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Name != "task" || got[2].Name != "decision" || got[1].Description != "Bug report or defect" {
		t.Fatalf("types = %+v", got)
	}
	if c := f.Calls(); !reflect.DeepEqual(c[0], []string{"--readonly", "types", "--json"}) {
		t.Fatalf("calls: %q", c)
	}
	// Без файла ответа фейк отдаёт встроенные типы 0.59.
	testbd.New(t)
	got, err = bd.Client{}.Types(context.Background())
	if err != nil || len(got) != 6 {
		t.Fatalf("типы по умолчанию: %+v %v", got, err)
	}
}

// v1.3 fix: отмена bd config set/unset — SIGINT, как у bd init: bd успевает прибраться.
func TestConfigWritesInterruptOnCancel(t *testing.T) {
	for _, name := range []string{"set", "unset"} {
		dir := t.TempDir()
		bin, started, got := filepath.Join(dir, "bd"), filepath.Join(dir, "started"), filepath.Join(dir, "got-int")
		script := "#!/bin/sh\ntrap 'touch \"" + got + "\"; exit 130' INT\n" +
			"sleep 5 </dev/null >/dev/null 2>&1 &\ntouch '" + started + "'\nwait $!\n"
		if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			for ctx.Err() == nil {
				if _, err := os.Stat(started); err == nil {
					cancel()
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
		c := bd.Client{Bin: bin}
		var err error
		if name == "set" {
			err = c.SetConfig(ctx, "k", "v")
		} else {
			err = c.ConfigUnset(ctx, "k")
		}
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s: err = %v, want отмену", name, err)
		}
		if _, err := os.Stat(got); err != nil {
			t.Errorf("bd config %s не получил SIGINT: %v", name, err)
		}
	}
}

// v1.4: поля спеки — spec_id, design, acceptance_criteria (json-теги bd 0.59, omitempty)
// разбираются и из bd show, и из bd list: оба отдают поля задачи целиком.
func TestSpecFieldsParsed(t *testing.T) {
	f := testbd.New(t)
	f.SetShow("p-1", `[{"id":"p-1","spec_id":"docs/spec.md","design":"## D","acceptance_criteria":"- [ ] A","dependents":[]}]`)
	f.SetList(`[{"id":"p-1","title":"T","status":"open","issue_type":"epic","spec_id":"S","design":"D","acceptance_criteria":"A","is_template":true}]`)
	d, err := bd.Client{}.Show(context.Background(), "p-1")
	if err != nil {
		t.Fatal(err)
	}
	if d.SpecID != "docs/spec.md" || d.Design != "## D" || d.AcceptanceCriteria != "- [ ] A" {
		t.Errorf("detail: %+v", d)
	}
	l, err := bd.Client{}.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if i := l[0]; i.SpecID != "S" || i.Design != "D" || i.AcceptanceCriteria != "A" || !i.IsTemplate {
		t.Errorf("issue: %+v", i)
	}
}
