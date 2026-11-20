package tree

import (
	"reflect"
	"testing"

	"github.com/nizom333/bdv/internal/bd"
)

// groupForest — корни трёх приоритетов; у b есть ребёнок, у c ребёнок закрыт.
func groupForest() *Forest {
	return Build([]bd.Issue{
		iss("a", "", "open", "task", 1, "01"),
		iss("b", "", "in_progress", "task", 1, "02"),
		iss("b.1", "b", "open", "task", 1, "03"),
		iss("c", "", "open", "task", 3, "01"),
		iss("c.1", "c", "closed", "task", 3, "02"),
		iss("z", "", "open", "task", 0, "05"),
	})
}

var grouped = Options{GroupByPriority: true}

func TestGroupsOrderAndCounts(t *testing.T) {
	rows := Rows(groupForest(), grouped, nil)
	want := []string{"#P0", "z", "#P1", "b", "b.1", "a", "#P3", "c"}
	if got := ids(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	counts := map[string]int{"#P0": 1, "#P1": 3, "#P3": 1}
	for _, r := range rows {
		if !r.IsGroup {
			continue
		}
		k := GroupKey(r.Group)
		if r.Issue != nil || r.Depth != 0 || !r.HasChildren || !r.Expanded {
			t.Errorf("%s: %+v", k, r)
		}
		if r.GroupCount != counts[k] {
			t.Errorf("%s: GroupCount = %d, want %d", k, r.GroupCount, counts[k])
		}
	}
	// Задачи группы на уровень глубже и знают свою группу.
	if rows[1].Depth != 1 || rows[4].Depth != 2 || rows[4].Group != 1 {
		t.Fatalf("depths %+v %+v", rows[1], rows[4])
	}
	// c.1 закрыт: c — лист.
	if rows[7].HasChildren {
		t.Fatalf("c %+v", rows[7])
	}
}

func TestGroupChildrenStayUnderParent(t *testing.T) {
	f := Build([]bd.Issue{
		iss("r", "", "open", "task", 2, "01"),
		iss("r.1", "r", "open", "task", 0, "02"),
	})
	got := ids(Rows(f, grouped, nil))
	if want := []string{"#P2", "r", "r.1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestEmptyGroupsHidden(t *testing.T) {
	f := Build([]bd.Issue{
		iss("a", "", "open", "task", 1, "01"),
		iss("x", "", "closed", "task", 2, "01"),
		iss("e", "", "open", "event", 4, "01"),
		iss("w", "", "open", "task", 7, "01"),
	})
	got := ids(Rows(f, grouped, nil))
	// Вне 0–4 — своя группа по номеру; закрытые и события не дают групп.
	if want := []string{"#P1", "a", "#P7", "w"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	o := grouped
	o.Query = "a"
	if got := ids(Rows(f, o, nil)); !reflect.DeepEqual(got, []string{"#P1", "a"}) {
		t.Fatalf("query: got %v", got)
	}
}

func TestGroupCollapse(t *testing.T) {
	rows := Rows(groupForest(), grouped, Expansion{GroupKey(1): false})
	want := []string{"#P0", "z", "#P1", "#P3", "c"}
	if got := ids(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	// Счётчик не зависит от сворачивания.
	if r := rows[2]; r.Expanded || r.GroupCount != 3 {
		t.Fatalf("#P1 %+v", r)
	}
}

func TestGroupForcedOpenOnQuery(t *testing.T) {
	o := grouped
	o.Query = "b.1"
	rows := Rows(groupForest(), o, Expansion{GroupKey(1): false})
	if got := ids(rows); !reflect.DeepEqual(got, []string{"#P1", "b", "b.1"}) {
		t.Fatalf("got %v", got)
	}
	if !rows[0].Expanded || rows[0].Context || rows[0].GroupCount != 2 {
		t.Fatalf("#P1 %+v", rows[0])
	}
}

func TestGroupingOff(t *testing.T) {
	f := groupForest()
	rows := Rows(f, Options{}, Expansion{GroupKey(1): false})
	want := []string{"b", "b.1", "z", "a", "c"}
	if got := ids(rows); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for _, r := range rows {
		if r.IsGroup || (r.Issue.Parent == "" && r.Depth != 0) {
			t.Fatalf("row %+v", r)
		}
	}
}

func TestGroupMembers(t *testing.T) {
	var got []string
	for _, is := range GroupMembers(groupForest(), Options{ShowClosed: true}, 3) {
		got = append(got, is.ID)
	}
	if want := []string{"c", "c.1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if g := GroupOf(groupForest(), "b.1"); g != 1 {
		t.Fatalf("GroupOf(b.1) = %d", g)
	}
}

func TestShortPrefix(t *testing.T) {
	ii := func(ids ...string) []bd.Issue {
		var out []bd.Issue
		for _, id := range ids {
			out = append(out, bd.Issue{ID: id})
		}
		return out
	}
	for _, c := range []struct {
		name string
		in   []bd.Issue
		want string
	}{
		{"общий", ii("acme-shop-fjpw", "acme-shop-x1y2.1", "acme-shop-q"), "acme-shop-"},
		{"разные", ii("acme-shop-fjpw", "other-x1"), ""},
		{"без дефиса", ii("ep", "ep.1"), ""},
		{"часть без дефиса", ii("bd-a1", "bda2"), ""},
		{"пустой хвост", ii("acme-", "acme-x"), ""},
		{"пусто", nil, ""},
		{"одна задача", ii("bd-a1"), "bd-"},
	} {
		if got := ShortPrefix(c.in); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

// StatusCounts считает задачи по статусам с учётом c/e, без запроса и сворачивания.
func TestStatusCounts(t *testing.T) {
	f := Build([]bd.Issue{
		{ID: "a", Status: "open", IssueType: "task"},
		{ID: "b", Status: "in_progress", IssueType: "task"},
		{ID: "c", Status: "closed", IssueType: "task"},
		{ID: "d", Status: "closed", IssueType: "event"},
		{ID: "e", Status: "open", IssueType: "task", Parent: "c"},
	})
	got := StatusCounts(f, Options{Query: "zzz"})
	if len(got) != 2 || got["open"] != 2 || got["in_progress"] != 1 {
		t.Errorf("без c/e = %v", got)
	}
	got = StatusCounts(f, Options{ShowClosed: true})
	if got["closed"] != 1 {
		t.Errorf("с c = %v", got)
	}
	got = StatusCounts(f, Options{ShowClosed: true, ShowEvents: true})
	if got["closed"] != 2 {
		t.Errorf("с c и e = %v", got)
	}
}
