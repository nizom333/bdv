package bd_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/nizom333/bdv/internal/bd"
	"github.com/nizom333/bdv/internal/testbd"
)

// bd memories --json — чтение с --readonly; в bd 0.59 пустая память — {} (снято с тестового проекта).
func TestMemoriesEmpty(t *testing.T) {
	f := testbd.New(t)
	got, err := bd.Client{DB: "/x/.beads"}.Memories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("memories = %v", got)
	}
	want := []string{"--readonly", "--db", "/x/.beads", "memories", "--json"}
	if c := f.Calls(); len(c) != 1 || !reflect.DeepEqual(c[0], want) {
		t.Fatalf("calls: %q", c)
	}
}

// Объект «ключ → текст» отдаётся списком по ключам; значение не строкой — JSON как есть.
func TestMemoriesSortedByKey(t *testing.T) {
	f := testbd.New(t)
	f.SetMemories(`{"zeta":"последняя","alpha":"## Первая\n- пункт","n":5}`)
	got, err := bd.Client{}.Memories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []bd.Memory{{Key: "alpha", Text: "## Первая\n- пункт"}, {Key: "n", Text: "5"}, {Key: "zeta", Text: "последняя"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("memories = %+v", got)
	}
}

// bd formula list --json — чтение с --readonly; без формул bd 0.59 печатает null.
func TestFormulasNull(t *testing.T) {
	f := testbd.New(t)
	got, err := bd.Client{}.Formulas(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("formulas = %v, %v", got, err)
	}
	if c := f.Calls(); len(c) != 1 || !reflect.DeepEqual(c[0], []string{"--readonly", "formula", "list", "--json"}) {
		t.Fatalf("calls: %q", c)
	}
}

// Непустой ответ (форма не проверена на настоящем bd: FormulaListEntry с name, type,
// description, source, steps, vars) — разбираются имя, тип, описание и источник.
func TestFormulasParsed(t *testing.T) {
	f := testbd.New(t)
	f.SetFormulas(`[{"name":"mol-release","type":"workflow","description":"Release checklist","source":".beads/formulas/mol-release.formula.json","steps":5,"vars":2},
{"name":"mol-fix","type":"workflow","description":"","source":"~/.beads/formulas/mol-fix.formula.toml","steps":3,"vars":0,"extra":{"x":1}}]`)
	got, err := bd.Client{}.Formulas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []bd.Formula{
		{Name: "mol-release", Type: "workflow", Description: "Release checklist", Source: ".beads/formulas/mol-release.formula.json"},
		{Name: "mol-fix", Type: "workflow", Source: "~/.beads/formulas/mol-fix.formula.toml"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("formulas = %+v", got)
	}
}

// Шаблоны — bd list --include-templates --flat --json --all -n 0 с --readonly; из ответа
// остаются эпики с меткой template и шаблоны-молекулы (is_template).
func TestTemplatesFiltered(t *testing.T) {
	f := testbd.New(t)
	f.SetTemplates(`[
{"id":"p-1","title":"Шаблон спеки","status":"open","issue_type":"epic","labels":["template"]},
{"id":"p-2","title":"Обычный эпик","status":"open","issue_type":"epic","labels":["ui"]},
{"id":"p-3","title":"Молекула","status":"open","issue_type":"task","is_template":true},
{"id":"p-4","title":"Задача с меткой","status":"open","issue_type":"task","labels":["template"]}]`)
	got, err := bd.Client{DB: "/d"}.Templates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, is := range got {
		ids = append(ids, is.ID)
	}
	if !reflect.DeepEqual(ids, []string{"p-1", "p-3"}) {
		t.Fatalf("templates = %v", ids)
	}
	want := []string{"--readonly", "--db", "/d", "list", "--include-templates", "--flat", "--json", "--all", "-n", "0"}
	if c := f.Calls(); len(c) != 1 || !reflect.DeepEqual(c[0], want) {
		t.Fatalf("calls: %q", c)
	}
}

// Ошибка bd — *bd.Error с первой строкой stderr, как у остальных чтений.
func TestTabReadsErrors(t *testing.T) {
	f := testbd.New(t)
	f.Fail("memories", "Error: memory store locked", 1)
	f.Fail("formula-list", "Error: bad formula", 1)
	f.Fail("list-templates", "Error: templates", 1)
	ctx := context.Background()
	if _, err := (bd.Client{}).Memories(ctx); err == nil || err.(*bd.Error).FirstLine() != "Error: memory store locked" {
		t.Errorf("memories err = %v", err)
	}
	if _, err := (bd.Client{}).Formulas(ctx); err == nil || err.(*bd.Error).FirstLine() != "Error: bad formula" {
		t.Errorf("formulas err = %v", err)
	}
	if _, err := (bd.Client{}).Templates(ctx); err == nil || err.(*bd.Error).FirstLine() != "Error: templates" {
		t.Errorf("templates err = %v", err)
	}
	// Обычный bd list от сбоя шаблонов не страдает.
	if _, err := (bd.Client{}).List(ctx); err != nil {
		t.Errorf("list err = %v", err)
	}
}
