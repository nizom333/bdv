package tree

import (
	"testing"

	"github.com/nizom333/bdv/internal/bd"
)

func abc() *Forest {
	return Build([]bd.Issue{
		iss("a", "", "open", "task", 2, "01"),
		iss("b", "", "open", "task", 2, "02"),
		iss("c", "", "open", "task", 2, "03"),
	})
}

func TestNearestNextSibling(t *testing.T) {
	if got := Nearest(abc(), "b", func(id string) bool { return id != "b" }); got != "c" {
		t.Fatal(got)
	}
}

func TestNearestPrevSibling(t *testing.T) {
	f := Build([]bd.Issue{iss("a", "", "open", "task", 2, "01"), iss("b", "", "open", "task", 2, "02")})
	if got := Nearest(f, "b", func(id string) bool { return id != "b" }); got != "a" {
		t.Fatal(got)
	}
}

func TestNearestParent(t *testing.T) {
	f := Build([]bd.Issue{iss("p", "", "open", "task", 2, "01"), iss("q", "p", "open", "task", 2, "01")})
	if got := Nearest(f, "q", func(id string) bool { return id != "q" }); got != "p" {
		t.Fatal(got)
	}
}

func TestNearestNone(t *testing.T) {
	if got := Nearest(abc(), "b", func(string) bool { return false }); got != "" {
		t.Fatal(got)
	}
}
