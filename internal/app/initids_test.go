package app

import (
	"slices"
	"testing"

	"github.com/nizom333/bdv/internal/i18n"
	"github.com/nizom333/bdv/internal/testbd"
)

// configSet — ровно та запись, что включает последовательные ID.
var configSet = []string{"config", "set", "issue_id_mode", "counter"}

// configCalls — вызовы bd config по порядку.
func (h *harness) configCalls() [][]string {
	var out [][]string
	for _, c := range h.fake.Calls() {
		if testbd.Subcommand(c) == "config-set" {
			out = append(out, c)
		}
	}
	return out
}

// c в подтверждении переключает строку режима ID, а не отменяет; по умолчанию хеш.
func TestInitConfirmCTogglesIDs(t *testing.T) {
	h := newNoDB(t)
	h.keys("i")
	h.wantView("Initialize beads in /w/proj?", "prefix: proj", "IDs: hash (bd default)",
		"y — run bd init · c — switch IDs · n — cancel")
	h.wantNoView("sequential")
	h.keys("c")
	h.wantView("Initialize beads in /w/proj?", "IDs: sequential 1, 2, 3")
	h.wantNoView("hash", "i — initialize")
	h.keys("c")
	h.wantView("IDs: hash (bd default)")
	// Склеенные руны: c переключает, n отменяет.
	h.keys("cn")
	h.wantNoDBScreen()
	h.wantCount("init", 0)
	h.wantCount("config-set", 0)
}

// Последовательные ID: после успешного bd init — bd config set issue_id_mode counter
// без --readonly, затем список.
func TestInitSequentialSetsConfig(t *testing.T) {
	h := newNoDB(t)
	h.fake.Fail("list", "", 0)
	h.keys("i", "c", "y")
	h.wantView("No tasks")
	calls := h.fake.Calls()
	var subs []string
	for _, c := range calls {
		subs = append(subs, testbd.Subcommand(c))
	}
	// list (нет базы), init, config, list.
	if want := []string{"list", "init", "config-set", "list"}; !slices.Equal(subs, want) {
		t.Fatalf("вызовы %v, want %v (%q)", subs, want, calls)
	}
	if cc := h.configCalls(); len(cc) != 1 || !slices.Equal(cc[0], configSet) {
		t.Fatalf("bd config = %q, want %q", cc, configSet)
	}
	h.wantNoView("⚠")
}

// Хеш (по умолчанию и после двойного c): bd config не вызывается.
func TestInitHashNoConfig(t *testing.T) {
	for _, keys := range [][]string{{"i", "y"}, {"i", "c", "c", "y"}} {
		h := newNoDB(t)
		h.fake.Fail("list", "", 0)
		h.keys(keys...)
		h.wantView("No tasks")
		h.wantCount("init", 1)
		h.wantCount("config-set", 0)
	}
}

// Неудачный bd config: ошибка видна после init и в статус-строке дерева, список грузится.
func TestInitConfigFailureShownTreeLoads(t *testing.T) {
	h := newNoDB(t)
	h.fake.Fail("config-set", "Error: unknown config key\nmore", 1)
	h.fake.Fail("list", "", 0)
	h.holdList = true
	h.keys("i", "c", "y")
	h.wantView("⚠ sequential IDs not enabled: Error: unknown config key", "Loading tasks…")
	h.wantNoView("more", "No beads database")
	h.holdList = false
	h.release()
	h.wantView("No tasks", "Tasks (0)")
	h.wantStatus("⚠ sequential IDs not enabled: Error: unknown config key")
	h.wantCount("config-set", 1)
	h.wantCount("init", 1)
}

// Неудачный bd init с выбранными последовательными ID: config не вызывается,
// выбор сохраняется для повтора.
func TestInitFailNoConfigChoiceKept(t *testing.T) {
	h := newNoDB(t)
	h.fake.Fail("init", "boom", 1)
	h.keys("i", "c", "y")
	h.wantView("bd init failed: exit status 1")
	h.wantCount("config-set", 0)
	h.keys("i")
	h.wantView("IDs: sequential 1, 2, 3")
	h.fake.Fail("init", "", 0)
	h.fake.Fail("list", "", 0)
	h.keys("y")
	h.wantCount("config-set", 1)
}

// Русские строки режима ID и ошибки config.
func TestInitIDsRU(t *testing.T) {
	h := newNoDB(t, func(o *Options) { o.Lang = i18n.RU })
	h.keys("i")
	h.wantView("ID: хеш (по умолчанию bd)", "y — запустить bd init · c — сменить ID · n — отмена")
	h.keys("c")
	h.wantView("ID: последовательные 1, 2, 3")
	h.fake.Fail("config-set", "Error: nope", 1)
	h.fake.Fail("list", "", 0)
	h.keys("y")
	h.wantStatus("⚠ последовательные ID не включены: Error: nope")
}
