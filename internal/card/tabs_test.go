package card

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/nizom333/bdv/internal/bd"
)

// Полоса прогресса: 8 клеток █/░ по доле закрытых, затем «n/m»; без тикетов — пусто.
func TestProgressBar(t *testing.T) {
	for _, c := range []struct {
		done, total int
		want        string
	}{
		{0, 3, "░░░░░░░░ 0/3"}, {1, 3, "███░░░░░ 1/3"}, {3, 5, "█████░░░ 3/5"}, {5, 5, "████████ 5/5"},
		{1, 100, "░░░░░░░░ 1/100"}, {99, 100, "███████░ 99/100"}, {0, 0, ""},
	} {
		if got := ansi.Strip(ProgressBar(c.done, c.total)); got != c.want {
			t.Errorf("ProgressBar(%d, %d) = %q, want %q", c.done, c.total, got, c.want)
		}
	}
}

// Карточка спеки: прогресс в чипах, дети — секцией Tickets (вместо Children), нумерованы.
func TestSpecCard(t *testing.T) {
	is := base()
	is.Description = "# Спека\nтекст"
	kids := []*bd.Issue{iss("p-1.1"), iss("p-1.2")}
	kids[1].Status = "closed"
	out, links := render(Input{Issue: is, Children: kids, ChildrenTitle: "Tickets", Progress: ProgressBar(1, 2)})
	plain := ansi.Strip(out)
	for _, want := range []string{"████░░░░ 1/2", "Tickets ─", "[1] ○ P2 p-1.1  T p-1.1", "[2] ✓ P2 p-1.2  T p-1.2", "Спека"} {
		if !strings.Contains(plain, want) {
			t.Errorf("нет %q в:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "Children") {
		t.Errorf("секция Children в карточке спеки:\n%s", plain)
	}
	if len(links) != 2 || links[0] != "p-1.1" {
		t.Errorf("links = %v", links)
	}
}

// Doc — карточка записи без задачи (память, формула): заголовок, приглушённая строка
// meta, текст как markdown; управляющие символы вычищены, строки не шире width.
func TestDoc(t *testing.T) {
	out := Doc("key\x1b]0;t\x07\nnext", "formula · .beads/x", "## Заголовок\n- пункт &#27;]0;y&#7;", "notty", 30)
	plain := ansi.Strip(out)
	for _, want := range []string{"key]0;t next", "formula · .beads/x", "Заголовок", "• пункт"} {
		if !strings.Contains(plain, want) {
			t.Errorf("нет %q в:\n%s", want, plain)
		}
	}
	for _, bad := range []string{"\x1b]", "\x07"} {
		if strings.Contains(out, bad) {
			t.Errorf("найдено %q в %q", bad, out)
		}
	}
	for _, l := range strings.Split(out, "\n") {
		if w := ansi.StringWidth(l); w > 30 {
			t.Errorf("строка шире 30: %q", l)
		}
	}
	if got := ansi.Strip(Doc("k", "", "", "notty", 30)); got != "k" {
		t.Errorf("пустая запись: %q", got)
	}
}
