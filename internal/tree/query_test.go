package tree

import (
	"reflect"
	"testing"

	"github.com/nizom333/bdv/internal/bd"
)

// titled — задача с заголовком.
func titled(id, parent, status, title string) bd.Issue {
	i := iss(id, parent, status, "task", 2, "01")
	i.Title = title
	return i
}

func searchForest() *Forest {
	return Build([]bd.Issue{
		titled("e", "", "open", "Эпик"),
		titled("e.1", "e", "open", "Таблица"),
		titled("e.2", "e", "open", "Поиск"),
	})
}

func TestQueryShowsMatchWithAncestorsAsContext(t *testing.T) {
	rows := Rows(searchForest(), Options{Query: "поиск"}, nil)
	if got := ids(rows); !reflect.DeepEqual(got, []string{"e", "e.2"}) {
		t.Fatalf("got %v", got)
	}
	if !rows[0].Context || !rows[0].Expanded || rows[1].Context {
		t.Fatalf("e=%+v t2=%+v", rows[0], rows[1])
	}
}

func TestQueryMatchesID(t *testing.T) {
	if got := ids(Rows(searchForest(), Options{Query: "e.2"}, nil)); !reflect.DeepEqual(got, []string{"e", "e.2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestQueryCyrillicCaseInsensitive(t *testing.T) {
	f := Build([]bd.Issue{titled("a", "", "open", "Контекст компании")})
	if got := ids(Rows(f, Options{Query: "КОН"}, nil)); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("got %v", got)
	}
}

func TestQueryRespectsHiddenClosed(t *testing.T) {
	f := Build([]bd.Issue{titled("a", "", "closed", "Поиск")})
	if got := ids(Rows(f, Options{Query: "поиск"}, nil)); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := ids(Rows(f, Options{Query: "поиск", ShowClosed: true}, nil)); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("got %v", got)
	}
}

func TestQueryIgnoresCollapse(t *testing.T) {
	got := ids(Rows(searchForest(), Options{Query: "поиск"}, Expansion{"e": false}))
	if !reflect.DeepEqual(got, []string{"e", "e.2"}) {
		t.Fatalf("got %v", got)
	}
}

func TestQueryNoMatches(t *testing.T) {
	if got := ids(Rows(searchForest(), Options{Query: "zzz"}, nil)); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestQueryTrimsSpaces(t *testing.T) {
	got := ids(Rows(searchForest(), Options{Query: "  поиск "}, nil))
	if !reflect.DeepEqual(got, []string{"e", "e.2"}) {
		t.Fatalf("got %v", got)
	}
}
