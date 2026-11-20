package app

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/nizom333/bdv/internal/clip"
	"github.com/nizom333/bdv/internal/i18n"
)

func TestCopyID(t *testing.T) {
	h := prepareFlatHarness(t, fixtureMain, func(o *Options) { o.Copy = clip.Copy })
	out := h.fake.SetPbcopy()
	h.start()
	h.keys("j")
	h.wantSelected("ep.1")
	h.keys("y")
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "ep.1" {
		t.Fatalf("pbcopy получил %q, want %q", got, "ep.1")
	}
	h.wantView("copied ID: ep.1")
}

func TestCopyToastClears(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("y")
	h.wantView("copied ID: ep")
	// Смена выбора тост не снимает.
	h.keys("j")
	h.wantView("copied ID: ep")
	h.tick()
	h.wantNoView("copied")
}

func TestCopyToastNewerSurvivesOldTimer(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("y")
	old := h.timers
	h.timers = nil
	h.keys("j", "y")
	// Таймер первого тоста устарел и не снимает второй.
	for _, m := range old {
		if _, ok := m.(toastClearMsg); ok {
			h.send(m)
		}
	}
	h.wantView("copied ID: ep.1")
	h.tick()
	h.wantNoView("copied")
}

func TestCopyError(t *testing.T) {
	h := newHarness(t, fixtureMain, func(o *Options) {
		o.Copy = func(string) error { return errors.New("nope") }
	})
	h.keys("y")
	h.wantView("copy failed: nope")
}

func TestCopyIgnoredWhileTypingAndInHelp(t *testing.T) {
	h := newHarness(t, fixtureMain)
	h.keys("/", "y")
	h.wantView("/y")
	h.keys("esc", "?", "y")
	if len(h.copied) != 0 {
		t.Fatalf("Copy вызван: %v", h.copied)
	}
	h.wantNoView("copied")
}

func TestCopyNothingSelected(t *testing.T) {
	h := newHarness(t, `[{"id":"x","title":"Закрытая","status":"closed","priority":2,"issue_type":"task"}]`)
	h.wantSelected("")
	h.keys("y")
	if len(h.copied) != 0 {
		t.Fatalf("Copy вызван: %v", h.copied)
	}
	h.wantNoView("copied")
}

func TestCopyRecordsSelected(t *testing.T) {
	h := newFlatHarness(t, fixtureMain)
	h.keys("j", "j", "y")
	if !slices.Equal(h.copied, []string{"bl"}) {
		t.Fatalf("copied = %v, want [bl]", h.copied)
	}
}

// fixtureCopy — задачи для Y/D/M: с описанием и заметками (markdown и управляющие
// символы), без них и с пустым типом.
const fixtureCopy = `[
{"id":"cp-1","title":"Заголовок \u001b[31mкрасный","status":"in_progress","priority":1,"issue_type":"feature",
 "description":"# Шаг\n\n**жирно** и ` + "`код`" + `\u0007\n- пункт","notes":"заметка\u001b]0;x\u0007 _курсив_",
 "created_at":"2026-11-01T10:00:00Z","updated_at":"2026-11-01T10:00:00Z"},
{"id":"cp-2","title":"Пустая","status":"open","priority":3,"issue_type":"task","description":"  \n ",
 "created_at":"2026-11-02T10:00:00Z","updated_at":"2026-11-02T10:00:00Z"},
{"id":"cp-3","title":"Только заметки","status":"open","priority":2,
 "notes":"N1","created_at":"2026-11-03T10:00:00Z","updated_at":"2026-11-03T10:00:00Z"}
]`

// copyHarness — плоское дерево fixtureCopy: cp-1, cp-2, cp-3 по порядку.
func copyHarness(t *testing.T, opts ...func(*Options)) *harness {
	t.Helper()
	h := newFlatHarness(t, fixtureCopy, opts...)
	h.wantIDs("cp-1", "cp-3", "cp-2")
	return h
}

func (h *harness) wantCopied(want ...string) {
	h.t.Helper()
	if !slices.Equal(h.copied, want) {
		h.t.Fatalf("copied = %q, want %q", h.copied, want)
	}
}

// y копирует ID, тост называет, что скопировано.
func TestCopyIDToastNamesWhat(t *testing.T) {
	h := copyHarness(t)
	h.keys("y")
	h.wantCopied("cp-1")
	h.wantStatus("copied ID: cp-1")
}

// Y — заголовок как есть, без управляющих символов (card.Clean).
func TestCopyTitle(t *testing.T) {
	h := copyHarness(t)
	h.keys("Y")
	h.wantCopied("Заголовок [31mкрасный")
	h.wantStatus("copied title")
	h.wantNoStatus("copied ID")
}

// D — исходный markdown описания из списка: разметка цела, управляющие символы убраны.
func TestCopyDescription(t *testing.T) {
	h := copyHarness(t)
	h.keys("D")
	h.wantCopied("# Шаг\n\n**жирно** и `код`\n- пункт")
	h.wantStatus("copied description")
}

// D при пустом (из пробелов) описании — «nothing to copy», буфер не трогается.
func TestCopyDescriptionEmpty(t *testing.T) {
	h := copyHarness(t)
	h.keys("G")
	h.wantSelected("cp-2")
	h.keys("D")
	h.wantCopied()
	h.wantStatus("nothing to copy")
}

// M — карточка markdown: заголовок, строка ID · статус · приоритет · тип, Description, Notes.
func TestCopyCardMarkdown(t *testing.T) {
	h := copyHarness(t)
	h.keys("M")
	h.wantCopied("# Заголовок [31mкрасный\n\n" +
		"`cp-1` · in_progress · P1 · feature\n\n" +
		"## Description\n\n# Шаг\n\n**жирно** и `код`\n- пункт\n\n" +
		"## Notes\n\nзаметка]0;x _курсив_")
	h.wantStatus("copied card")
}

// M без пустых секций; пустой тип в строке метаданных не пишется.
func TestCopyCardOmitsEmpty(t *testing.T) {
	h := copyHarness(t)
	h.keys("j", "M", "j", "M")
	h.wantCopied(
		"# Только заметки\n\n`cp-3` · open · P2\n\n## Notes\n\nN1",
		"# Пустая\n\n`cp-2` · open · P3 · task")
}

// На заголовке группы и без выбора копировать нечего: тост, буфер не трогается.
func TestCopyGroupHeaderNothing(t *testing.T) {
	h := newHarness(t, fixtureCopy)
	h.keys("k")
	h.wantSelected("#P1")
	for _, k := range []string{"y", "Y", "D", "M"} {
		h.keys(k)
		h.wantStatus("nothing to copy")
		h.tick()
		h.wantNoStatus("nothing to copy")
	}
	h.wantCopied()

	e := newHarness(t, `[]`)
	e.keys("M")
	e.wantCopied()
	e.wantStatus("nothing to copy")
}

// Y/D/M не копируют при вводе поиска, в справке и в поле комментария.
func TestCopyKeysIgnoredInTypingHelpComment(t *testing.T) {
	h := copyHarness(t)
	h.keys("/", "Y", "D", "M")
	h.wantStatus("/YDM")
	h.keys("esc", "?", "D")
	h.wantNoView("Help ·")
	h.keys("?", "M")
	h.keys("m", "Y", "D", "M")
	h.wantBox("YDM")
	h.keys("esc")
	h.wantCopied()
	h.wantNoView("copied", "nothing to copy")
}

// Русские тосты копирования.
func TestCopyToastsRU(t *testing.T) {
	h := copyHarness(t, func(o *Options) { o.Lang = i18n.RU })
	for _, c := range []struct{ key, want string }{
		{"y", "скопирован ID: cp-1"}, {"Y", "скопирован заголовок"},
		{"D", "скопировано описание"}, {"M", "скопирована карточка"},
	} {
		h.keys(c.key)
		h.wantStatus(c.want)
	}
	h.keys("G", "D")
	h.wantStatus("нечего копировать")
}

// Y — заголовок одной строкой: переводы строк и табуляция становятся пробелами
// (card.CleanLine), управляющие символы убраны.
func TestCopyTitleSingleLine(t *testing.T) {
	h := newFlatHarness(t, `[{"id":"t1","title":"Первая\nвторая\tтретья\u0007","status":"open","priority":2,"issue_type":"task"}]`)
	h.keys("Y")
	h.wantCopied("Первая вторая третья")
	h.wantStatus("copied title")
}
