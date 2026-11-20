package tree

import (
	"reflect"
	"testing"

	"github.com/nizom333/bdv/internal/bd"
)

func TestSortSiblings(t *testing.T) {
	f := Build([]bd.Issue{
		iss("a", "", "closed", "task", 0, "01"),
		iss("b", "", "open", "task", 2, "02"),
		iss("c", "", "in_progress", "task", 3, "01"),
		iss("d", "", "open", "task", 2, "01"),
		iss("e", "", "blocked", "task", 0, "01"),
		iss("g", "", "deferred", "task", 0, "01"),
		iss("h", "", "pinned", "task", 0, "01"),
	})
	got := ids(Rows(f, Options{ShowClosed: true}, nil))
	want := []string{"c", "d", "b", "e", "g", "h", "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestClosedHiddenByDefault(t *testing.T) {
	f := Build([]bd.Issue{iss("x", "", "open", "task", 2, "01"), iss("y", "", "closed", "task", 2, "01")})
	if got := ids(Rows(f, Options{}, nil)); !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("got %v", got)
	}
}

func TestEventsHiddenByDefault(t *testing.T) {
	f := Build([]bd.Issue{iss("x", "", "open", "task", 2, "01"), iss("e1", "x", "open", "event", 2, "01")})
	rows := Rows(f, Options{}, nil)
	if got := ids(rows); !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("got %v", got)
	}
	if rows[0].HasChildren {
		t.Fatal("x.HasChildren должен быть false")
	}
}

func TestShowEventsToggle(t *testing.T) {
	f := Build([]bd.Issue{iss("x", "", "open", "task", 2, "01"), iss("e1", "x", "open", "event", 2, "01")})
	got := ids(Rows(f, Options{ShowEvents: true}, Expansion{"x": true}))
	if !reflect.DeepEqual(got, []string{"x", "e1"}) {
		t.Fatalf("got %v", got)
	}
}

func TestClosedParentWithOpenChildIsContext(t *testing.T) {
	f := Build([]bd.Issue{
		iss("p", "", "closed", "task", 2, "01"),
		iss("c1", "p", "open", "task", 2, "01"),
		iss("c2", "p", "closed", "task", 2, "02"),
	})
	rows := Rows(f, Options{}, nil)
	if got := ids(rows); !reflect.DeepEqual(got, []string{"p", "c1"}) {
		t.Fatalf("got %v", got)
	}
	if !rows[0].Context || rows[1].Context || !rows[0].Expanded {
		t.Fatalf("flags %+v %+v", rows[0], rows[1])
	}
}

func TestDefaultExpansion(t *testing.T) {
	f := Build([]bd.Issue{
		iss("p1", "", "open", "task", 2, "01"),
		iss("k1", "p1", "open", "task", 2, "01"),
		iss("p2", "", "open", "task", 2, "02"),
		iss("k2", "p2", "closed", "task", 2, "01"),
	})
	rows := Rows(f, Options{ShowClosed: true}, nil)
	if got := ids(rows); !reflect.DeepEqual(got, []string{"p1", "k1", "p2"}) {
		t.Fatalf("got %v", got)
	}
	if !rows[0].Expanded || rows[2].Expanded || !rows[2].HasChildren {
		t.Fatalf("flags %+v", rows)
	}
}

func TestManualExpansionOverridesDefault(t *testing.T) {
	f := Build([]bd.Issue{iss("p1", "", "open", "task", 2, "01"), iss("k1", "p1", "open", "task", 2, "01")})
	got := ids(Rows(f, Options{}, Expansion{"p1": false}))
	if !reflect.DeepEqual(got, []string{"p1"}) {
		t.Fatalf("got %v", got)
	}
}

func TestDepth(t *testing.T) {
	f := Build([]bd.Issue{
		iss("a", "", "open", "task", 2, "01"),
		iss("a.1", "a", "open", "task", 2, "01"),
		iss("a.1.1", "a.1", "open", "task", 2, "01"),
	})
	rows := Rows(f, Options{}, nil)
	if len(rows) != 3 || rows[0].Depth != 0 || rows[1].Depth != 1 || rows[2].Depth != 2 {
		t.Fatalf("rows %+v", rows)
	}
}
