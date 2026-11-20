package tree

import (
	"reflect"
	"testing"

	"github.com/nizom333/bdv/internal/bd"
)

func flatFixture() *Forest {
	tpl := iss("tpl", "", "open", "epic", 1, "01")
	tpl.Labels = []string{"template"}
	return Build([]bd.Issue{
		iss("b", "", "open", "epic", 2, "02"),
		iss("a", "x", "in_progress", "epic", 3, "03"), // родитель вне выборки — неважно
		iss("c", "", "closed", "epic", 0, "01"),
		tpl,
		iss("b.1", "b", "closed", "task", 2, "04"),
		iss("b.2", "b", "open", "task", 2, "05"),
		iss("b.3", "b", "closed", "event", 4, "06"),
		iss("d", "", "open", "decision", 2, "01"),
	})
}

// Flat: только задачи keep, фильтр закрытых и запрос, порядок соседей дерева, без групп.
func TestFlat(t *testing.T) {
	f := flatFixture()
	epics := func(is *bd.Issue) bool { return is.IssueType == "epic" }
	if got := ids(Flat(f, Options{GroupByPriority: true}, epics)); !reflect.DeepEqual(got, []string{"a", "tpl", "b"}) {
		t.Fatalf("без закрытых: %v", got)
	}
	if got := ids(Flat(f, Options{ShowClosed: true}, epics)); !reflect.DeepEqual(got, []string{"a", "tpl", "b", "c"}) {
		t.Fatalf("с закрытыми: %v", got)
	}
	if got := ids(Flat(f, Options{Query: " B "}, epics)); !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("запрос: %v", got)
	}
	// ReadyOnly и раскрытие на плоский список не влияют; строки без глубины и контекста.
	for _, r := range Flat(f, Options{ReadyOnly: true}, epics) {
		if r.Depth != 0 || r.Context || r.IsGroup || r.HasChildren {
			t.Fatalf("строка %+v", r)
		}
	}
}

// Progress: прямые дети без событий; закрытые / все.
func TestProgress(t *testing.T) {
	f := flatFixture()
	if done, total := f.Progress("b"); done != 1 || total != 2 {
		t.Fatalf("Progress(b) = %d/%d", done, total)
	}
	if done, total := f.Progress("d"); done != 0 || total != 0 {
		t.Fatalf("Progress(d) = %d/%d", done, total)
	}
	if got := f.Tickets("b"); len(got) != 2 || got[0].ID != "b.2" || got[1].ID != "b.1" {
		t.Fatalf("Tickets(b) = %v", got)
	}
}
