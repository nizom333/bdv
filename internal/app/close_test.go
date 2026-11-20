package app

import (
	"context"
	"testing"
	"time"

	"github.com/nizom333/bdv/internal/bd"
)

// ctxSource — источник, который ждёт отмены контекста вызова List.
type ctxSource struct{ started chan struct{} }

func (s ctxSource) List(ctx context.Context) ([]bd.Issue, error) {
	close(s.started)
	<-ctx.Done()
	return nil, ctx.Err()
}
func (ctxSource) Show(context.Context, string) (bd.Detail, error)        { return bd.Detail{}, nil }
func (ctxSource) Comments(context.Context, string) ([]bd.Comment, error) { return nil, nil }
func (ctxSource) AddComment(context.Context, string, string) error       { return nil }
func (ctxSource) Init(context.Context) (string, error)                   { return "", nil }
func (ctxSource) SetConfig(context.Context, string, string) error        { return nil }
func (ctxSource) ConfigUnset(context.Context, string) error              { return nil }
func (ctxSource) ConfigList(context.Context) (map[string]string, error)  { return nil, nil }
func (ctxSource) ConfigGet(context.Context, string) (string, string, error) {
	return "", "", nil
}
func (ctxSource) Types(context.Context) ([]bd.TypeInfo, error)   { return nil, nil }
func (ctxSource) Memories(context.Context) ([]bd.Memory, error)  { return nil, nil }
func (ctxSource) Formulas(context.Context) ([]bd.Formula, error) { return nil, nil }
func (ctxSource) Templates(context.Context) ([]bd.Issue, error)  { return nil, nil }

// Close отменяет корневой контекст: идущий вызов bd видит ctx.Done. Так main завершает
// вызовы и после выхода Bubble Tea по сигналу, когда quit не вызывался.
func TestCloseCancelsRootContext(t *testing.T) {
	src := ctxSource{started: make(chan struct{})}
	m := New(Options{Source: src})
	done := make(chan struct{})
	go func() {
		m.listCmd()()
		close(done)
	}()
	<-src.started
	m.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("List не увидел отмену после Close")
	}
	// Повторный Close безопасен.
	m.Close()
}
