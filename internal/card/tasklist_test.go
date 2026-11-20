package card

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/bd"
)

// TaskList: секция с линией, строки «[n] иконка id  заголовок», ⊘ у ждущих, номера
// только у первых девяти, управляющие символы из bd вычищены, строки не шире width.
func TestTaskList(t *testing.T) {
	var items []*bd.Issue
	for i := 1; i <= 10; i++ {
		items = append(items, &bd.Issue{ID: fmt.Sprintf("t%d", i), Title: fmt.Sprintf("Задача %d\x07", i), Status: "open"})
	}
	items[1].Status = "closed"
	out, links := TaskList("Tasks", items, func(id string) bool { return id == "t3" }, 30)
	ls := lines(out)
	if !strings.HasPrefix(ls[0], "Tasks ─") {
		t.Fatalf("заголовок секции: %q", ls[0])
	}
	for _, want := range []string{"[1] ○ t1  Задача 1", "[2] ✓ t2  Задача 2", "[3] ○ t3 " + BlockedMark + "  Задача 3", "    ○ t10  Задача 10"} {
		if !slices.Contains(ls, want) {
			t.Errorf("нет строки %q в:\n%s", want, strings.Join(ls, "\n"))
		}
	}
	if len(links) != 9 || links[0] != "t1" || links[8] != "t9" {
		t.Errorf("links = %v", links)
	}
	if strings.ContainsAny(out, "\x07\x1b") {
		t.Errorf("управляющая последовательность из bd прошла: %q", out)
	}
	for _, l := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(l); w > 30 {
			t.Errorf("строка шире 30: %q", l)
		}
	}
}
