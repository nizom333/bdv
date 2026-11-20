package tree

import (
	"reflect"
	"testing"
	"time"

	"github.com/nizom333/bdv/internal/bd"
)

// iss строит задачу для фикстур; created — день месяца ("" → нулевое время).
func iss(id, parent, status, typ string, prio int, created string) bd.Issue {
	var t time.Time
	if created != "" {
		t, _ = time.Parse("2006-01-02", "2026-01-"+created)
	}
	return bd.Issue{ID: id, Parent: parent, Status: status, IssueType: typ, Priority: prio, CreatedAt: t}
}

func ids(rows []Row) []string {
	out := []string{}
	for _, r := range rows {
		if r.IsGroup {
			out = append(out, GroupKey(r.Group))
			continue
		}
		out = append(out, r.Issue.ID)
	}
	return out
}

func TestAncestors(t *testing.T) {
	f := Build([]bd.Issue{
		iss("a", "", "open", "task", 2, "01"),
		iss("a.1", "a", "open", "task", 2, "01"),
		iss("a.1.1", "a.1", "open", "task", 2, "01"),
	})
	got := Ancestors(f, "a.1.1")
	if !reflect.DeepEqual(got, []string{"a", "a.1"}) {
		t.Fatalf("got %v", got)
	}
	if f.ParentID("a") != "" || f.ParentID("a.1") != "a" {
		t.Fatal("ParentID")
	}
}

func TestOrphanParentBecomesRoot(t *testing.T) {
	f := Build([]bd.Issue{iss("c", "missing", "open", "task", 2, "01")})
	rows := Rows(f, Options{}, nil)
	if len(rows) != 1 || rows[0].Depth != 0 {
		t.Fatalf("rows %v", ids(rows))
	}
}

func TestCycleTwoNodes(t *testing.T) {
	f := Build([]bd.Issue{
		iss("a", "b", "open", "task", 2, "01"),
		iss("b", "a", "open", "task", 2, "01"),
	})
	rows := Rows(f, Options{}, nil)
	if got := ids(rows); !reflect.DeepEqual(got, []string{"a", "b"}) || rows[0].Depth != 0 || rows[1].Depth != 1 {
		t.Fatalf("rows %v", got)
	}
	if len(Ancestors(f, "a")) != 0 || !reflect.DeepEqual(Ancestors(f, "b"), []string{"a"}) {
		t.Fatal("Ancestors")
	}
	if f.ParentID("a") != "" {
		t.Fatal("ParentID(a)")
	}
}

func TestCycleThreeNodes(t *testing.T) {
	f := Build([]bd.Issue{
		iss("a", "b", "open", "task", 2, "01"),
		iss("b", "c", "open", "task", 2, "01"),
		iss("c", "a", "open", "task", 2, "01"),
	})
	if got := ids(Rows(f, Options{}, nil)); len(got) != 3 {
		t.Fatalf("rows %v", got)
	}
	if got := Nearest(f, "b", func(string) bool { return false }); got != "" {
		t.Fatal(got)
	}
	_ = f.DefaultExpanded("a")
}

func TestForestLen(t *testing.T) {
	if n := Build(nil).Len(); n != 0 {
		t.Errorf("Len пустого = %d", n)
	}
	if n := Build([]bd.Issue{{ID: "a"}, {ID: "b", Parent: "a"}}).Len(); n != 2 {
		t.Errorf("Len = %d, want 2", n)
	}
}
