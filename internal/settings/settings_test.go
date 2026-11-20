package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// snap — снимок из ответа config list (ключи базы) и config get (YAML-ключи).
func snap(list map[string]string, gets map[string]string) Snapshot {
	g := map[string]GetResult{}
	for _, e := range Table {
		if e.Store != DB {
			g[e.Key] = GetResult{Value: gets[e.Key], Location: "config.yaml"}
		}
	}
	return Build(list, g, []string{"task", "bug"})
}

func keys(es []Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Key)
	}
	return out
}

func mustEntry(t *testing.T, key string) Entry {
	t.Helper()
	e, ok := Lookup(key)
	if !ok {
		t.Fatalf("нет ключа %s", key)
	}
	return e
}

// В таблице ровно ключи спеки v1.3, ни одного запрещённого.
func TestTableMatchesSpec(t *testing.T) {
	want := []string{
		"issue_prefix", "issue_id_mode", "min_hash_length", "max_hash_length", "max_collision_prob",
		"status.custom", "types.custom",
		"create.require-description", "validation.on-create", "validation.on-close", "hierarchy.max-depth",
		"dolt.auto-commit", "dolt.auto-push", "export.auto", "backup.enabled",
		"compaction_enabled", "auto_compact_enabled", "compact_tier1_days", "compact_tier2_days",
	}
	if got := keys(Table); !slices.Equal(got, want) {
		t.Fatalf("ключи таблицы:\n%v\nwant\n%v", got, want)
	}
	for _, e := range Table {
		if Forbidden(e.Key) {
			t.Errorf("запрещённый ключ в таблице: %s", e.Key)
		}
	}
}

func TestForbidden(t *testing.T) {
	for _, k := range []string{"schema_version", "db", "actor", "identity", "sync.remote",
		"federation.remote", "federation.sovereignty", "jira.api_token", "linear.api_key",
		"x.secret", "y.password", "github.TOKEN"} {
		if !Forbidden(k) {
			t.Errorf("%s не запрещён", k)
		}
	}
	for _, k := range []string{"issue_id_mode", "status.custom", "dolt.auto-commit"} {
		if Forbidden(k) {
			t.Errorf("%s запрещён", k)
		}
	}
}

// Ключи базы — из config list, YAML — из config get; неизвестные ключи списка не видны.
func TestBuildStoresAndUnknownHidden(t *testing.T) {
	s := snap(map[string]string{
		"issue_prefix": "demo-app", "schema_version": "6", "compaction_enabled": "false",
		"jira.api_token": "s3cr3t", "unknown.key": "x",
	}, map[string]string{"create.require-description": "false", "dolt.auto-commit": "on", "hierarchy.max-depth": "5"})
	if v := s.Values["issue_prefix"]; v.Raw != "demo-app" || !v.Set {
		t.Fatalf("issue_prefix = %+v", v)
	}
	if v := s.Values["compaction_enabled"]; !v.Set || v.Raw != "false" {
		t.Fatalf("compaction_enabled = %+v", v)
	}
	if v := s.Values["issue_id_mode"]; v.Set {
		t.Fatalf("issue_id_mode задан: %+v", v)
	}
	// YAML: значение по умолчанию bd отдаёт как есть — это не «задано».
	if v := s.Values["create.require-description"]; v.Set {
		t.Fatalf("create.require-description задан: %+v", v)
	}
	if v := s.Values["hierarchy.max-depth"]; !v.Set || v.Raw != "5" {
		t.Fatalf("hierarchy.max-depth = %+v", v)
	}
	for _, e := range s.Visible() {
		if Forbidden(e.Key) || e.Key == "unknown.key" || e.Key == "schema_version" {
			t.Fatalf("виден ключ %s", e.Key)
		}
	}
	if _, ok := s.Values["schema_version"]; ok {
		t.Fatal("schema_version в снимке")
	}
}

// Ключ, чей config get упал, скрыт.
func TestBuildGetErrorHides(t *testing.T) {
	g := map[string]GetResult{}
	for _, k := range GetKeys() {
		g[k] = GetResult{Location: "config.yaml"}
	}
	g["export.auto"] = GetResult{Err: errors.New("boom")}
	s := Build(map[string]string{}, g, nil)
	if slices.Contains(keys(s.Visible()), "export.auto") {
		t.Fatal("export.auto виден после ошибки get")
	}
	if !slices.Contains(keys(s.Visible()), "backup.enabled") {
		t.Fatal("backup.enabled скрыт")
	}
	if s.Failed != 1 {
		t.Fatalf("Failed = %d, want 1", s.Failed)
	}
	// Без ответа get ключ тоже скрыт.
	delete(g, "backup.enabled")
	s = Build(map[string]string{}, g, nil)
	if slices.Contains(keys(s.Visible()), "backup.enabled") {
		t.Fatal("backup.enabled виден без ответа get")
	}
}

// Длины хеша и вероятность коллизий видны только в режиме hash.
func TestHashKeysHiddenInCounterMode(t *testing.T) {
	hashOnly := []string{"min_hash_length", "max_hash_length", "max_collision_prob"}
	s := snap(map[string]string{}, nil)
	for _, k := range hashOnly {
		if !slices.Contains(keys(s.Visible()), k) {
			t.Fatalf("%s скрыт в режиме hash по умолчанию", k)
		}
	}
	s = snap(map[string]string{"issue_id_mode": "counter"}, nil)
	for _, k := range hashOnly {
		if slices.Contains(keys(s.Visible()), k) {
			t.Fatalf("%s виден в режиме counter", k)
		}
	}
	if !slices.Contains(keys(s.Visible()), "issue_id_mode") {
		t.Fatal("issue_id_mode скрыт")
	}
}

func TestToggleAndCycle(t *testing.T) {
	s := snap(map[string]string{"compaction_enabled": "false"}, map[string]string{"backup.enabled": "true"})
	if got := Toggle(mustEntry(t, "compaction_enabled"), s); got != "true" {
		t.Errorf("toggle false = %q", got)
	}
	if got := Toggle(mustEntry(t, "backup.enabled"), s); got != "false" {
		t.Errorf("toggle true = %q", got)
	}
	// Не задан — умолчание false.
	if got := Toggle(mustEntry(t, "export.auto"), s); got != "true" {
		t.Errorf("toggle default = %q", got)
	}
	if got := Cycle(mustEntry(t, "issue_id_mode"), s); got != "counter" {
		t.Errorf("cycle hash = %q", got)
	}
	s = snap(map[string]string{"issue_id_mode": "counter"}, map[string]string{"dolt.auto-commit": "batch"})
	if got := Cycle(mustEntry(t, "issue_id_mode"), s); got != "hash" {
		t.Errorf("cycle counter = %q", got)
	}
	if got := Cycle(mustEntry(t, "dolt.auto-commit"), s); got != "on" {
		t.Errorf("cycle batch = %q", got)
	}
	// Незнакомое значение — первый вариант.
	s = snap(nil, map[string]string{"validation.on-create": "strict"})
	if got := Cycle(mustEntry(t, "validation.on-create"), s); got != "none" {
		t.Errorf("cycle unknown = %q", got)
	}
}

func TestCheckNumber(t *testing.T) {
	s := snap(map[string]string{"max_hash_length": "6"}, nil)
	min := mustEntry(t, "min_hash_length")
	for _, c := range []struct {
		e     Entry
		in    string
		want  string
		kind  ProblemKind
		a, b  string
		snapV Snapshot
	}{
		{min, " 4 ", "4", 0, "", "", s},
		{min, "x", "", NotInt, "", "", s},
		{min, "4.5", "", NotInt, "", "", s},
		{min, "2", "", OutOfRange, "3", "10", s},
		{min, "11", "", OutOfRange, "3", "10", s},
		// min ≤ max: max задан 6.
		{min, "7", "", AboveOther, "max_hash_length", "6", s},
		{mustEntry(t, "max_hash_length"), "9", "9", 0, "", "", snap(map[string]string{"min_hash_length": "5"}, nil)},
		{mustEntry(t, "max_hash_length"), "4", "", BelowOther, "min_hash_length", "5", snap(map[string]string{"min_hash_length": "5"}, nil)},
		// max не задан: умолчание 8.
		{min, "9", "", AboveOther, "max_hash_length", "8", snap(nil, nil)},
		{mustEntry(t, "max_collision_prob"), "0.1", "0.1", 0, "", "", s},
		{mustEntry(t, "max_collision_prob"), "0.5", "0.5", 0, "", "", s},
		{mustEntry(t, "max_collision_prob"), ".25", "0.25", 0, "", "", s},
		// Запятая как десятичный разделитель.
		{mustEntry(t, "max_collision_prob"), "0,1", "0.1", 0, "", "", s},
		// max при незаданном min сверяется с умолчанием min (3) — в пределах диапазона это 3.
		{mustEntry(t, "max_hash_length"), "3", "3", 0, "", "", snap(nil, nil)},
		{mustEntry(t, "max_collision_prob"), "0.6", "", OutOfRange, "0.01", "0.5", s},
		{mustEntry(t, "max_collision_prob"), "0.001", "", OutOfRange, "0.01", "0.5", s},
		{mustEntry(t, "max_collision_prob"), "abc", "", NotNumber, "", "", s},
		{mustEntry(t, "max_collision_prob"), "NaN", "", NotNumber, "", "", s},
		{mustEntry(t, "hierarchy.max-depth"), "0", "", OutOfRange, "1", "10", s},
		{mustEntry(t, "compact_tier1_days"), "0", "", AtLeast, "1", "", s},
		{mustEntry(t, "compact_tier1_days"), "365", "365", 0, "", "", s},
		{mustEntry(t, "compact_tier1_days"), "", "", NotInt, "", "", s},
	} {
		got, p := CheckNumber(c.e, c.snapV, c.in)
		if c.kind == 0 {
			if p != nil || got != c.want {
				t.Errorf("%s %q: %q %+v, want %q", c.e.Key, c.in, got, p, c.want)
			}
			continue
		}
		if p == nil || p.Kind != c.kind || p.A != c.a || p.B != c.b {
			t.Errorf("%s %q: problem %+v, want %v %q %q", c.e.Key, c.in, p, c.kind, c.a, c.b)
		}
	}
}

func TestListParseJoin(t *testing.T) {
	if got := ParseList(" awaiting_review, qa:wip ,,"); !slices.Equal(got, []string{"awaiting_review", "qa:wip"}) {
		t.Fatalf("ParseList = %q", got)
	}
	if got := ParseList(""); len(got) != 0 {
		t.Fatalf("ParseList пусто = %q", got)
	}
	if got := JoinList([]string{"a", "b"}); got != "a,b" {
		t.Fatalf("JoinList = %q", got)
	}
	if ItemName("qa:wip") != "qa" || ItemName("x") != "x" {
		t.Fatal("ItemName")
	}
}

func TestCheckName(t *testing.T) {
	st := mustEntry(t, "status.custom")
	ty := mustEntry(t, "types.custom")
	s := snap(nil, nil)
	items := []string{"awaiting_review", "qa:wip"}
	for _, c := range []struct {
		e    Entry
		name string
		kind ProblemKind
		a    string
	}{
		{st, "review_2", 0, ""},
		{st, "Review", BadName, ""},
		{st, "a-b", BadName, ""},
		{st, "", BadName, ""},
		{st, "awaiting_review", Duplicate, "awaiting_review"},
		{st, "qa", Duplicate, "qa"},
		{st, "in_progress", IsBuiltin, "in_progress"},
		{st, "closed", IsBuiltin, "closed"},
		{ty, "bug", IsBuiltin, "bug"},
		{ty, "spike", 0, ""},
	} {
		p := CheckName(c.e, s, items, c.name)
		if c.kind == 0 {
			if p != nil {
				t.Errorf("%s %q: %+v", c.e.Key, c.name, p)
			}
			continue
		}
		if p == nil || p.Kind != c.kind || p.A != c.a {
			t.Errorf("%s %q: %+v, want %v %q", c.e.Key, c.name, p, c.kind, c.a)
		}
	}
	if got := st.BuiltinItems(s); !slices.Equal(got, []string{"open", "in_progress", "blocked", "deferred", "closed"}) {
		t.Fatalf("встроенные статусы = %v", got)
	}
	if got := ty.BuiltinItems(s); !slices.Equal(got, []string{"task", "bug"}) {
		t.Fatalf("встроенные типы = %v", got)
	}
}

// Предупреждения: issue_id_mode — всегда, dolt.auto-commit — для off и batch,
// dolt.auto-push — при включении.
func TestWarnFor(t *testing.T) {
	for _, c := range []struct {
		key, val string
		want     bool
	}{
		{"issue_id_mode", "counter", true},
		{"issue_id_mode", "hash", true},
		{"dolt.auto-commit", "off", true},
		{"dolt.auto-commit", "batch", true},
		{"dolt.auto-commit", "on", false},
		{"dolt.auto-push", "true", true},
		{"dolt.auto-push", "false", false},
		{"validation.on-create", "error", true},
		{"validation.on-create", "warn", false},
		{"export.auto", "true", false},
	} {
		e, _ := Lookup(c.key)
		if got := e.WarnFor(c.val); got != c.want {
			t.Errorf("%s=%s: WarnFor = %v", c.key, c.val, got)
		}
	}
	for _, e := range Table {
		if e.Warn == "" && len(e.WarnOn) > 0 {
			t.Errorf("%s: WarnOn без Warn", e.Key)
		}
	}
}

func TestSectionsOrder(t *testing.T) {
	last := Section(-1)
	for _, e := range Table {
		if e.Section < last {
			t.Fatalf("%s: секции не по порядку", e.Key)
		}
		last = e.Section
	}
	if mustEntry(t, "issue_prefix").Kind != ReadOnly {
		t.Fatal("issue_prefix не только для чтения")
	}
	if !strings.Contains(strings.Join(GetKeys(), ","), "types.custom") {
		t.Fatal("types.custom читается не через config get")
	}
}

// v1.3 fix: YAML-ключ без location config.yaml скрыт (в 0.59 он не YAML); types.custom
// (хранилище Any) виден и без location; ошибкой get это не считается.
func TestBuildYAMLNeedsLocation(t *testing.T) {
	g := map[string]GetResult{}
	for _, e := range Table {
		if e.Store != DB {
			g[e.Key] = GetResult{Location: "config.yaml"}
		}
	}
	g["export.auto"] = GetResult{}
	g["types.custom"] = GetResult{Value: "spike"}
	s := Build(nil, g, nil)
	vis := keys(s.Visible())
	if slices.Contains(vis, "export.auto") {
		t.Fatal("export.auto без location виден")
	}
	if !slices.Contains(vis, "types.custom") || !s.Values["types.custom"].Set {
		t.Fatalf("types.custom: %v %+v", vis, s.Values["types.custom"])
	}
	if s.Failed != 0 {
		t.Fatalf("Failed = %d", s.Failed)
	}
	if mustEntry(t, "types.custom").Store != Any {
		t.Fatal("types.custom не Any")
	}
}

// v1.3 fix: сброс одной из длин хеша не должен ломать min ≤ max (умолчания 3 и 8).
func TestCheckUnset(t *testing.T) {
	s := snap(map[string]string{"min_hash_length": "9", "max_hash_length": "10"}, nil)
	p := CheckUnset(mustEntry(t, "max_hash_length"), s)
	if p == nil || p.Kind != UnsetBreaks || p.A != "min_hash_length" || p.B != "9" || p.C != "max_hash_length" || p.D != "8" {
		t.Fatalf("unset max: %+v", p)
	}
	if p := CheckUnset(mustEntry(t, "min_hash_length"), s); p != nil {
		t.Fatalf("unset min: %+v", p)
	}
	s = snap(map[string]string{"min_hash_length": "5", "max_hash_length": "6"}, nil)
	if p := CheckUnset(mustEntry(t, "max_hash_length"), s); p != nil {
		t.Fatalf("unset max 6 при min 5: %+v", p)
	}
	if p := CheckUnset(mustEntry(t, "backup.enabled"), s); p != nil {
		t.Fatalf("прочий ключ: %+v", p)
	}
}
