package app

import (
	"testing"

	"github.com/nizom333/bdv/internal/i18n"
)

// v1.4: Spec, Design и Acceptance из bd show появляются в карточке после Notes,
// названия секций английские и в русском интерфейсе.
func TestCardSpecSectionsFromShow(t *testing.T) {
	for _, l := range []i18n.Lang{i18n.EN, i18n.RU} {
		h := prepareFlatHarness(t, fixtureMain, func(o *Options) { o.Lang = l })
		h.fake.SetShow("bl", `[{"id":"bl","spec_id":"docs/spec.md","design":"слой A","acceptance_criteria":"- [ ] тесты зелёные","dependents":[]}]`)
		h.start()
		h.keys("j", "j")
		h.wantSelected("bl")
		h.wantCard("Spec ──", "docs/spec.md", "Design ──", "слой A", "Acceptance ──", "тесты зелёные")
	}
}
