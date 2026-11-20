// Пакет testbd подменяет команду bd в PATH скриптом, который отвечает заготовленным JSON
// и записывает аргументы каждого вызова. Нужен только тестам.
package testbd

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Fake — поддельный bd. Dir содержит bin/bd и файлы ответов.
type Fake struct {
	Dir string
	t   testing.TB
}

const script = `#!/bin/sh
d="$(dirname "$0")/.."
line=""
first=1
tpl=0
for a in "$@"; do
  if [ "$first" = 1 ]; then line="$a"; first=0; else line="$line$(printf '\037')$a"; fi
  # bd list --include-templates — отдельный ответ: шаблоны (вкладка Formulas).
  if [ "$a" = --include-templates ]; then tpl=1; fi
done
printf '%s\n' "$line" >> "$d/calls"
cmd=""
id=""
src=""
while [ $# -gt 0 ]; do
  case "$1" in
    list|show) cmd="$1"; id="$2"; break;;
    init) cmd="init"; break;;
    config)
      # config set|unset|list|get: у get — ключ в id.
      cmd="config-$2"; id="$3"
      if [ "$2" != get ]; then id=""; fi
      break;;
    types) cmd="types"; break;;
    memories) cmd="memories"; break;;
    formula) cmd="formula-$2"; break;;
    comments)
      if [ "$2" = add ]; then
        cmd="comments-add"; id="$3"; shift 3
        while [ $# -gt 0 ]; do
          if [ "$1" = -f ]; then src="$2"; break; fi
          shift
        done
      else
        cmd="comments"; id="$2"
      fi
      break;;
  esac
  shift
done
if [ "$cmd" = list ] && [ "$tpl" = 1 ]; then cmd="list-templates"; fi
if [ "$cmd" = comments-add ]; then
  # Файл комментария живёт только пока идёт вызов: копия и ID — для проверок теста.
  n=$(( $(cat "$d/comment-adds" 2>/dev/null | wc -l) + 1 ))
  cp "$src" "$d/comment-add-$n.txt" 2>/dev/null || : > "$d/comment-add-$n.txt"
  printf '%s\n' "$id" >> "$d/comment-adds"
fi
if [ "$cmd" = init ]; then
  # stdin bd init: bdv не должен ничего туда отдавать (пустой stdin).
  cat > "$d/init-stdin"
fi
if [ -f "$d/sleep-$cmd" ]; then
  # Потоки sleep отвязаны от bd: после убийства скрипта по таймауту
  # осиротевший sleep не держит открытым stdout.
  sleep "$(cat "$d/sleep-$cmd")" </dev/null >/dev/null 2>&1
fi
if [ -n "$id" ] && [ -f "$d/fail-$cmd-$id" ]; then
  # Сбой одного ключа или одной задачи (FailID).
  cat "$d/fail-$cmd-$id" >&2
  exit "$(cat "$d/fail-$cmd-$id.code")"
fi
if [ -f "$d/fail-$cmd" ]; then
  cat "$d/fail-$cmd" >&2
  exit "$(cat "$d/fail-$cmd.code")"
fi
# Ответ bd types --json в 0.59 без своих типов (снят с настоящего bd).
typesDef='{"core_types":[{"name":"task","description":"General work item (default)"},{"name":"bug","description":"Bug report or defect"},{"name":"feature","description":"New feature or enhancement"},{"name":"chore","description":"Maintenance or housekeeping"},{"name":"epic","description":"Large body of work spanning multiple issues"},{"name":"decision","description":"Architecture decision record (ADR)"}]}'
case "$cmd" in
  list) f="$d/list.json"; def='[]';;
  show) f="$d/show-$id.json"; def="[{\"id\":\"$id\"}]";;
  comments) f="$d/comments-$id.json"; def='[]';;
  init)
    if [ -f "$d/init.err" ]; then cat "$d/init.err" >&2; fi
    f="$d/init.out"; def='bd initialized';;
  config-set|config-unset) f="$d/config.out"; def='Set issue_id_mode = counter';;
  config-list) f="$d/config-list.json"; def='{}';;
  config-get) f="$d/config-get-$id.json"; def="{\"key\":\"$id\",\"value\":\"\"}";;
  types) f="$d/types.json"; def="$typesDef";;
  # Пустые ответы bd 0.59 (сняты с тестового проекта): memories — {}, formula list — null.
  list-templates) f="$d/templates.json"; def='[]';;
  memories) f="$d/memories.json"; def='{}';;
  formula-list) f="$d/formulas.json"; def='null';;
  *) exit 0;;
esac
if [ -f "$f" ]; then cat "$f"; else printf '%s\n' "$def"; fi
`

// New пишет скрипт bd во временную папку и ставит её первой в PATH на время теста.
func New(t testing.TB) *Fake {
	t.Helper()
	f := &Fake{Dir: t.TempDir(), t: t}
	if err := os.Mkdir(f.bin(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.bin(), "bd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", f.bin()+string(os.PathListSeparator)+os.Getenv("PATH"))
	return f
}

func (f *Fake) bin() string { return filepath.Join(f.Dir, "bin") }

func (f *Fake) write(name, content string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.Dir, name), []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// SetList задаёт ответ на bd list.
func (f *Fake) SetList(json string) { f.write("list.json", json) }

// SetShow задаёт ответ на bd show <id>.
func (f *Fake) SetShow(id, json string) { f.write("show-"+id+".json", json) }

// SetComments задаёт ответ на bd comments <id>.
func (f *Fake) SetComments(id, json string) { f.write("comments-"+id+".json", json) }

// SetInitOutput задаёт вывод успешного bd init: stdout и stderr.
func (f *Fake) SetInitOutput(stdout, stderr string) {
	f.write("init.out", stdout)
	f.write("init.err", stderr)
}

// InitStdin — что получил на stdin последний bd init; "" — ничего или вызова не было.
func (f *Fake) InitStdin() string {
	data, _ := os.ReadFile(filepath.Join(f.Dir, "init-stdin"))
	return string(data)
}

// Fail заставляет команду cmd (list|list-templates|show|comments|comments-add|init|config-set|config-unset|config-list|config-get|types|memories|formula-list) печатать stderr и выходить с кодом code.
// Пустой stderr вместе с code == 0 снимает сбой.
func (f *Fake) Fail(cmd, stderr string, code int) {
	f.t.Helper()
	if stderr == "" && code == 0 {
		os.Remove(filepath.Join(f.Dir, "fail-"+cmd))
		os.Remove(filepath.Join(f.Dir, "fail-"+cmd+".code"))
		return
	}
	f.write("fail-"+cmd, stderr)
	f.write("fail-"+cmd+".code", strconv.Itoa(code))
}

// Sleep заставляет команду cmd (list|list-templates|show|comments|comments-add|init|config-set|config-unset|config-list|config-get|types|memories|formula-list) ждать seconds секунд перед ответом;
// 0 снимает задержку.
func (f *Fake) Sleep(cmd string, seconds float64) {
	f.t.Helper()
	if seconds == 0 {
		os.Remove(filepath.Join(f.Dir, "sleep-"+cmd))
		return
	}
	f.write("sleep-"+cmd, strconv.FormatFloat(seconds, 'f', -1, 64))
}

// FailID — как Fail, но только для вызова cmd с этим id: задача у show и comments,
// ключ у config-get. Пустой stderr вместе с code == 0 снимает сбой.
func (f *Fake) FailID(cmd, id, stderr string, code int) { f.Fail(cmd+"-"+id, stderr, code) }

// SetConfigList задаёт ответ на bd config list --json.
func (f *Fake) SetConfigList(json string) { f.write("config-list.json", json) }

// SetConfigGet задаёт ответ на bd config get <key> --json; без него — {"key":…,"value":""},
// как у настоящего bd для незаданного ключа.
func (f *Fake) SetConfigGet(key, json string) { f.write("config-get-"+key+".json", json) }

// SetTypes задаёт ответ на bd types --json; без него — встроенные типы bd 0.59.
func (f *Fake) SetTypes(json string) { f.write("types.json", json) }

// SetMemories задаёт ответ на bd memories --json; без него — {} (памяти нет).
func (f *Fake) SetMemories(json string) { f.write("memories.json", json) }

// SetFormulas задаёт ответ на bd formula list --json; без него — null (формул нет).
func (f *Fake) SetFormulas(json string) { f.write("formulas.json", json) }

// SetTemplates задаёт ответ на bd list --include-templates; без него — [].
func (f *Fake) SetTemplates(json string) { f.write("templates.json", json) }

// Calls возвращает аргументы каждого вызова bd по порядку.
func (f *Fake) Calls() [][]string {
	data, err := os.ReadFile(filepath.Join(f.Dir, "calls"))
	if err != nil {
		return nil
	}
	var calls [][]string
	for _, l := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		calls = append(calls, strings.Split(l, "\x1f"))
	}
	return calls
}

// Count — сколько раз вызвана подкоманда cmd (см. Subcommand).
// bd comments add считается как comments-add, а не как comments.
func (f *Fake) Count(cmd string) int {
	n := 0
	for _, c := range f.Calls() {
		if Subcommand(c) == cmd {
			n++
		}
	}
	return n
}

// Subcommand — подкоманда вызова: list, list-templates (list --include-templates), show,
// comments, comments-add, init, config-set, config-unset, config-list, config-get, types,
// memories или formula-list; "" — другое.
func Subcommand(call []string) string {
	for i, a := range call {
		switch a {
		case "list":
			for _, b := range call[i+1:] {
				if b == "--include-templates" {
					return "list-templates"
				}
			}
			return a
		case "memories":
			return a
		case "formula":
			if i+1 < len(call) {
				return "formula-" + call[i+1]
			}
			return a
		case "show", "init", "types":
			return a
		case "config":
			if i+1 < len(call) {
				return "config-" + call[i+1]
			}
			return "config"
		case "comments":
			if i+1 < len(call) && call[i+1] == "add" {
				return "comments-add"
			}
			return a
		}
	}
	return ""
}

// CommentAdd — один вызов bd comments add: ID задачи и текст файла -f на момент вызова.
type CommentAdd struct{ ID, Text string }

// CommentAdds возвращает все вызовы bd comments add по порядку, включая неудачные
// (Fail) и оборванные таймаутом (Sleep): фейк записывает вызов до ответа.
func (f *Fake) CommentAdds() []CommentAdd {
	data, err := os.ReadFile(filepath.Join(f.Dir, "comment-adds"))
	if err != nil {
		return nil
	}
	var out []CommentAdd
	for i, id := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		text, err := os.ReadFile(filepath.Join(f.Dir, "comment-add-"+strconv.Itoa(i+1)+".txt"))
		if err != nil {
			f.t.Fatal(err)
		}
		out = append(out, CommentAdd{ID: id, Text: string(text)})
	}
	return out
}

// SetPbcopy кладёт в тот же bin поддельный pbcopy: stdin уходит в файл, путь к файлу возвращается.
func (f *Fake) SetPbcopy() (out string) {
	f.t.Helper()
	out = filepath.Join(f.Dir, "pbcopy.out")
	body := "#!/bin/sh\ncat > \"$(dirname \"$0\")/../pbcopy.out\"\n"
	if err := os.WriteFile(filepath.Join(f.bin(), "pbcopy"), []byte(body), 0o755); err != nil {
		f.t.Fatal(err)
	}
	return out
}
