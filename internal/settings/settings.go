// Пакет settings — декларативная таблица настроек bd для экрана настроек bdv (v1.3)
// и чистая логика над ней: снимок значений, видимость, проверка ввода, списки.
// Тексты (названия, описания, предупреждения) — в каталоге i18n по ключу bd.
package settings

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Kind — тип значения настройки.
type Kind int

const (
	Bool     Kind = iota // переключатель true/false
	Enum                 // перебор Values
	Int                  // целое в [Min, Max]
	Float                // дробное в [Min, Max]
	List                 // список через запятую; встроенные элементы — только для чтения
	ReadOnly             // только показ
)

// Store — откуда читается значение.
type Store int

const (
	// DB — ключ базы (Dolt): одним вызовом bd config list --json.
	DB Store = iota
	// YAML — ключ .beads/config.yaml: bd config get <key> --json. Ключ виден, только если
	// get вернул location "config.yaml": иначе этот bd не считает его YAML-ключом (так
	// export.* в 0.59), и запись осиротела бы в базе.
	YAML
	// Any — хранилище неоднозначно (types.custom): bd config get без проверки location.
	Any
)

// Section — секция экрана; порядок констант — порядок секций.
type Section int

const (
	IDs Section = iota
	StatusesTypes
	Creating
	Storage
)

// Entry — одна настройка.
type Entry struct {
	Key     string
	Section Section
	Kind    Kind
	Store   Store
	Values  []string // Enum: варианты по порядку перебора
	Min     float64  // Int, Float: нижняя граница
	Max     float64  // Int, Float: верхняя граница; 0 — без неё
	Default string   // умолчание bd; "" — неизвестно (показывается только «(default)»)
	// Builtin — встроенные элементы списка; nil у types.custom — типы из bd types.
	Builtin []string
	Warn    string   // ключ предупреждения в каталоге; "" — без предупреждения
	WarnOn  []string // новые значения, при которых нужно предупреждение; nil — при любом
	// HashOnly — видна только в режиме hash (issue_id_mode не counter).
	HashOnly bool
}

// builtinStatuses — встроенные статусы bd 0.59 (bd config --help); команды bd statuses в 0.59 нет.
var builtinStatuses = []string{"open", "in_progress", "blocked", "deferred", "closed"}

// FallbackTypes — встроенные типы bd 0.59 на случай ошибки bd types.
var FallbackTypes = []string{"task", "bug", "feature", "chore", "epic", "decision"}

// Table — все настройки экрана по секциям. Чего здесь нет, на экране не бывает: ключи
// из config list вне таблицы (schema_version, compact_batch_size, секреты…) не видны.
// Ключи сверены с bd 0.59 (тестовый проект, --readonly) и спекой v1.3.
var Table = []Entry{
	{Key: "issue_prefix", Section: IDs, Kind: ReadOnly, Store: DB},
	{Key: "issue_id_mode", Section: IDs, Kind: Enum, Store: DB, Values: []string{"hash", "counter"},
		Default: "hash", Warn: "issue_id_mode"},
	// Документация расходится в умолчании (3 или 4); на деле bd берёт 3 — оно и служит
	// для проверки min ≤ max при вводе max и при сбросе.
	{Key: "min_hash_length", Section: IDs, Kind: Int, Store: DB, Min: 3, Max: 10, Default: "3", HashOnly: true},
	{Key: "max_hash_length", Section: IDs, Kind: Int, Store: DB, Min: 3, Max: 10, Default: "8", HashOnly: true},
	{Key: "max_collision_prob", Section: IDs, Kind: Float, Store: DB, Min: 0.01, Max: 0.5, Default: "0.25", HashOnly: true},

	{Key: "status.custom", Section: StatusesTypes, Kind: List, Store: DB, Builtin: builtinStatuses},
	// Хранилище types.custom в 0.59 неоднозначно (справка bd types говорит о config.yaml,
	// маршрутизация config set — о базе): читаем через config get, он видит оба.
	{Key: "types.custom", Section: StatusesTypes, Kind: List, Store: Any},

	{Key: "create.require-description", Section: Creating, Kind: Bool, Store: YAML, Default: "false"},
	{Key: "validation.on-create", Section: Creating, Kind: Enum, Store: YAML, Values: []string{"none", "warn", "error"},
		Default: "none", Warn: "validation", WarnOn: []string{"error"}},
	{Key: "validation.on-close", Section: Creating, Kind: Enum, Store: YAML, Values: []string{"none", "warn", "error"},
		Default: "none", Warn: "validation", WarnOn: []string{"error"}},
	{Key: "hierarchy.max-depth", Section: Creating, Kind: Int, Store: YAML, Min: 1, Max: 10, Default: "3"},

	{Key: "dolt.auto-commit", Section: Storage, Kind: Enum, Store: YAML, Values: []string{"on", "off", "batch"},
		Default: "on", Warn: "dolt.auto-commit", WarnOn: []string{"off", "batch"}},
	{Key: "dolt.auto-push", Section: Storage, Kind: Bool, Store: YAML, Default: "false",
		Warn: "dolt.auto-push", WarnOn: []string{"true"}},
	{Key: "export.auto", Section: Storage, Kind: Bool, Store: YAML, Default: "false"},
	{Key: "backup.enabled", Section: Storage, Kind: Bool, Store: YAML, Default: "false"},
	{Key: "compaction_enabled", Section: Storage, Kind: Bool, Store: DB, Default: "false"},
	{Key: "auto_compact_enabled", Section: Storage, Kind: Bool, Store: DB, Default: "false"},
	{Key: "compact_tier1_days", Section: Storage, Kind: Int, Store: DB, Min: 1, Default: "30"},
	{Key: "compact_tier2_days", Section: Storage, Kind: Int, Store: DB, Min: 1, Default: "90"},
}

// Lookup — настройка по ключу bd.
func Lookup(key string) (Entry, bool) {
	for _, e := range Table {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

// forbiddenKeys — ключи, которые нельзя показывать и менять (спека v1.3).
var forbiddenKeys = []string{"schema_version", "db", "actor", "identity", "sync.remote"}

// Forbidden — ключ нельзя показывать и менять: служебные, путь к базе, личность,
// remote синхронизации, federation.* и всё похожее на секрет.
func Forbidden(key string) bool {
	k := strings.ToLower(key)
	if slices.Contains(forbiddenKeys, k) || strings.HasPrefix(k, "federation.") {
		return true
	}
	for _, s := range []string{"token", "key", "secret", "password"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// GetKeys — ключи, которые читаются через bd config get (Store YAML и Any), по порядку таблицы.
func GetKeys() []string {
	var out []string
	for _, e := range Table {
		if e.Store != DB {
			out = append(out, e.Key)
		}
	}
	return out
}

// Value — значение ключа: Raw — как отдал bd; Set — задано явно (не умолчание).
type Value struct {
	Raw string
	Set bool
}

// GetResult — ответ bd config get одного ключа.
type GetResult struct {
	Value    string
	Location string // "config.yaml" у YAML-ключей, иначе ""
	Err      error
}

// yamlLocation — location YAML-ключа в ответе bd config get.
const yamlLocation = "config.yaml"

// Snapshot — значения настроек проекта на момент чтения.
type Snapshot struct {
	Values map[string]Value
	Hidden map[string]bool // config get упал, не ответил или ключ не YAML: не показывается
	Failed int             // сколько config get упало (ошибка или нет ответа)
	Types  []string        // встроенные типы из bd types
}

// Build собирает снимок: ключи базы — из list (bd config list), YAML-ключи — из gets
// (bd config get); YAML-ключ без location config.yaml скрыт. Ключи list вне таблицы отбрасываются. YAML-ключ задан, если bd
// отдал непустое значение, отличное от умолчания: config get показывает и умолчание,
// и отличить «задано равным умолчанию» от «не задано» по JSON нельзя.
func Build(list map[string]string, gets map[string]GetResult, types []string) Snapshot {
	s := Snapshot{Values: map[string]Value{}, Hidden: map[string]bool{}, Types: types}
	if len(s.Types) == 0 {
		s.Types = FallbackTypes
	}
	for _, e := range Table {
		switch e.Store {
		case DB:
			if v, ok := list[e.Key]; ok {
				s.Values[e.Key] = Value{Raw: v, Set: true}
			} else {
				s.Values[e.Key] = Value{}
			}
		case YAML, Any:
			g, ok := gets[e.Key]
			if !ok || g.Err != nil {
				s.Hidden[e.Key] = true
				s.Failed++
				continue
			}
			if e.Store == YAML && g.Location != yamlLocation {
				s.Hidden[e.Key] = true
				continue
			}
			s.Values[e.Key] = Value{Raw: g.Value, Set: g.Value != "" && g.Value != e.Default}
		}
	}
	return s
}

// Effective — действующее значение: заданное или умолчание (может быть "").
func (s Snapshot) Effective(e Entry) string {
	if v := s.Values[e.Key]; v.Set {
		return v.Raw
	}
	return e.Default
}

// HashMode — ID задач хешевые (issue_id_mode не counter).
func (s Snapshot) HashMode() bool {
	e, _ := Lookup("issue_id_mode")
	return s.Effective(e) != "counter"
}

// Visible — видимые настройки по порядку таблицы: без скрытых и без длин хеша в режиме counter.
func (s Snapshot) Visible() []Entry {
	var out []Entry
	hash := s.HashMode()
	for _, e := range Table {
		if s.Hidden[e.Key] || (e.HashOnly && !hash) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// WarnFor — нужно ли предупреждение при новом значении v.
func (e Entry) WarnFor(v string) bool {
	if e.Warn == "" {
		return false
	}
	return e.WarnOn == nil || slices.Contains(e.WarnOn, v)
}

// truthy — значение bool у bd.
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true
	}
	return false
}

// Toggle — новое значение bool: противоположное действующему.
func Toggle(e Entry, s Snapshot) string {
	if truthy(s.Effective(e)) {
		return "false"
	}
	return "true"
}

// Cycle — следующее значение enum после действующего; незнакомое — первый вариант.
func Cycle(e Entry, s Snapshot) string {
	i := slices.Index(e.Values, s.Effective(e))
	if i < 0 {
		return e.Values[0]
	}
	return e.Values[(i+1)%len(e.Values)]
}

// ProblemKind — почему ввод не принят.
type ProblemKind int

const (
	_           ProblemKind = iota
	NotInt                  // не целое число
	NotNumber               // не число
	OutOfRange              // вне [A, B]
	AtLeast                 // меньше A (верхней границы нет)
	AboveOther              // больше значения B ключа A (min_hash_length > max_hash_length)
	BelowOther              // меньше значения B ключа A
	BadName                 // имя не из [a-z0-9_]+
	Duplicate               // A уже в списке
	IsBuiltin               // A — встроенный элемент
	UnsetBreaks             // сброс сделает A (=B) больше C (=D)
)

// Problem — ошибка ввода с подстановками для текста.
type Problem struct {
	Kind       ProblemKind
	A, B, C, D string
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// CheckNumber проверяет ввод int/float: формат, диапазон и min_hash_length ≤ max_hash_length.
// Возвращает нормализованное значение для bd config set.
func CheckNumber(e Entry, s Snapshot, input string) (string, *Problem) {
	in := strings.TrimSpace(input)
	var v float64
	var out string
	if e.Kind == Int {
		n, err := strconv.Atoi(in)
		if err != nil {
			return "", &Problem{Kind: NotInt}
		}
		v, out = float64(n), strconv.Itoa(n)
	} else {
		// Запятая как десятичный разделитель (русская раскладка) — точка для bd.
		f, err := strconv.ParseFloat(strings.ReplaceAll(in, ",", "."), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return "", &Problem{Kind: NotNumber}
		}
		v, out = f, num(f)
	}
	switch {
	case e.Max > 0 && (v < e.Min || v > e.Max):
		return "", &Problem{Kind: OutOfRange, A: num(e.Min), B: num(e.Max)}
	case v < e.Min:
		return "", &Problem{Kind: AtLeast, A: num(e.Min)}
	}
	// Пара длин хеша: min ≤ max. Неизвестное умолчание другой стороны не проверяется.
	other, above := "", false
	switch e.Key {
	case "min_hash_length":
		other, above = "max_hash_length", true
	case "max_hash_length":
		other = "min_hash_length"
	}
	if other != "" {
		oe, _ := Lookup(other)
		ov := s.Effective(oe)
		if o, err := strconv.Atoi(ov); err == nil {
			if above && v > float64(o) {
				return "", &Problem{Kind: AboveOther, A: other, B: ov}
			}
			if !above && v < float64(o) {
				return "", &Problem{Kind: BelowOther, A: other, B: ov}
			}
		}
	}
	return out, nil
}

// CheckUnset — можно ли сбросить ключ: сброс одной из длин хеша к умолчанию таблицы
// (min 3, max 8) не должен сделать min_hash_length больше max_hash_length.
func CheckUnset(e Entry, s Snapshot) *Problem {
	if e.Key != "min_hash_length" && e.Key != "max_hash_length" {
		return nil
	}
	minE, _ := Lookup("min_hash_length")
	maxE, _ := Lookup("max_hash_length")
	minV, maxV := s.Effective(minE), s.Effective(maxE)
	if e.Key == "min_hash_length" {
		minV = minE.Default
	} else {
		maxV = maxE.Default
	}
	lo, err1 := strconv.Atoi(minV)
	hi, err2 := strconv.Atoi(maxV)
	if err1 == nil && err2 == nil && lo > hi {
		return &Problem{Kind: UnsetBreaks, A: minE.Key, B: minV, C: maxE.Key, D: maxV}
	}
	return nil
}

// ParseList разбирает список bd: элементы через запятую без пробелов по краям, без пустых.
func ParseList(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// JoinList — список для bd config set.
func JoinList(items []string) string { return strings.Join(items, ",") }

// ItemName — имя элемента без категории («qa:wip» → «qa»; категории — из документации новых bd).
func ItemName(item string) string {
	name, _, _ := strings.Cut(item, ":")
	return name
}

// BuiltinItems — встроенные элементы списка: статусы bd или типы из bd types.
func (e Entry) BuiltinItems(s Snapshot) []string {
	if e.Builtin != nil {
		return e.Builtin
	}
	return s.Types
}

var nameRE = regexp.MustCompile(`^[a-z0-9_]+$`)

// CheckName проверяет новый элемент списка: [a-z0-9_]+, не повтор items, не встроенный.
func CheckName(e Entry, s Snapshot, items []string, name string) *Problem {
	if !nameRE.MatchString(name) {
		return &Problem{Kind: BadName}
	}
	for _, it := range items {
		if ItemName(it) == name {
			return &Problem{Kind: Duplicate, A: name}
		}
	}
	if slices.Contains(e.BuiltinItems(s), name) {
		return &Problem{Kind: IsBuiltin, A: name}
	}
	return nil
}
