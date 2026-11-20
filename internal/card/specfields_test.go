package card

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/i18n"
)

// v1.4: секции Spec (одна строка), Design и Acceptance (markdown) — после Notes, в этом
// порядке; значения из bd show (Detail) важнее значений из списка.
func TestSpecDesignAcceptanceSections(t *testing.T) {
	is := base()
	is.Notes = "заметка"
	is.Design = "из списка"
	d := &bd.Detail{
		SpecID:             "docs/spec.md#v1.4",
		Design:             "## Схема\n- слой **A**",
		AcceptanceCriteria: "- [ ] работает\n- [x] проверено",
	}
	out, _ := render(Input{Issue: is, Detail: d})
	plain := ansi.Strip(out)
	pos := func(s string) int {
		i := strings.Index(plain, s)
		if i < 0 {
			t.Fatalf("нет %q в:\n%s", s, plain)
		}
		return i
	}
	if !(pos("Notes ─") < pos("Spec ─") && pos("Spec ─") < pos("Design ─") && pos("Design ─") < pos("Acceptance ─")) {
		t.Fatalf("порядок секций:\n%s", plain)
	}
	pos("docs/spec.md#v1.4")
	pos("Схема")
	pos("• слой")
	pos("работает")
	if strings.Contains(plain, "из списка") {
		t.Errorf("Design из списка вместо bd show:\n%s", plain)
	}
}

// До ответа bd show поля берутся из списка (bd list отдаёт те же поля задачи).
func TestSpecFieldsFromListBeforeDetail(t *testing.T) {
	is := base()
	is.SpecID = "SPEC-7"
	is.AcceptanceCriteria = "готово, когда тесты зелёные"
	out, _ := render(Input{Issue: is})
	plain := ansi.Strip(out)
	for _, want := range []string{"Spec ─", "SPEC-7", "Acceptance ─", "готово, когда тесты зелёные"} {
		if !strings.Contains(plain, want) {
			t.Errorf("нет %q в:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "Design") {
		t.Errorf("пустая секция Design:\n%s", plain)
	}
}

// Названия секций английские и в русском интерфейсе; текст bd очищен от управляющих
// символов (R7/R8), Spec — одной строкой, строки не шире панели.
func TestSpecSectionsCleanAndEnglish(t *testing.T) {
	is := base()
	d := &bd.Detail{SpecID: "a\nb\x1b]0;x\x07c", Design: "x\x1b[2Jy", AcceptanceCriteria: "a &#27;]0;x&#7; b &#x1b;[2J c"}
	out, _ := Render(Input{Issue: is, Detail: d, Width: 40, Style: "notty", Text: i18n.For(i18n.RU)})
	for _, want := range []string{"Spec ─", "Design ─", "Acceptance ─", "a b]0;xc"} {
		if !strings.Contains(out, want) {
			t.Errorf("нет %q в:\n%s", want, out)
		}
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "\x1b") && !strings.Contains(l, "\x1b[") {
			t.Errorf("сырой ESC: %q", l)
		}
		if strings.Contains(l, "\x1b[2J") || strings.Contains(l, "\x1b]") || strings.Contains(l, "\x07") {
			t.Errorf("управляющая последовательность прошла: %q", l)
		}
		if w := ansi.StringWidth(l); w > 40 {
			t.Errorf("строка шире 40: %q", l)
		}
	}
}
